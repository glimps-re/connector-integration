package sdk

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/glimps-re/connector-integration/sdk/events"
)

var LogLevel = &slog.LevelVar{}

var logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: LogLevel})).WithGroup("connectors-manager-client")

const (
	// Error code
	InvalidAPIKeyCode     = 1
	RevokedAPIKeyCode     = 2
	basePath              = "/api/v1/connectors"
	taskChannelBufferSize = 10
)

type APIErrorResponse struct {
	Code int `json:"code"`
}

var (
	ErrUnauthorizedConnector = errors.New("connector's api key is either revoked or invalid")
	ErrNoHelmConfig          = errors.New("no specific helm config for this connector type")
	ErrInvalidConnectorType  = errors.New("invalid connector type")
	ErrNoHelmForConnector    = errors.New("no helm chart available for this connector")
	ErrNoComposeForConnector = errors.New("no docker compose available for this connector")
)

// Context Key that can be used to insert a specific X-Request-Id header
type CtxRequestIDKey struct{}

type ConnectorManagerClientConfig struct {
	URL      string `mapstructure:"url"`
	APIKey   string `mapstructure:"api-key"` //nolint:gosec // needed to configure connector
	Insecure bool   `mapstructure:"insecure"`
}

type ConnectorManagerClient struct {
	httpClient *http.Client
	url        string
	apiKey     string
}

type ConnectorStatus int

const (
	Started ConnectorStatus = iota
	Stopped
)

// Connector must comply to this interface to be used with manager
type Connector interface {
	Start(ctx context.Context) (err error)
	Stop(ctx context.Context) (err error)
	Configure(ctx context.Context, config json.RawMessage) (err error)
	Restore(ctx context.Context, restoreInfo RestoreActionContent) (err error)
	Status() (status ConnectorStatus)
}

type HTTPError struct {
	StatusCode int
	Body       []byte
}

func (e HTTPError) Error() string {
	return fmt.Sprintf("invalid response from connector manager, %d (%s): %s", e.StatusCode, http.StatusText(e.StatusCode), e.Body)
}

func NewHTTPError(code int, body []byte) HTTPError {
	return HTTPError{
		StatusCode: code,
		Body:       body,
	}
}

type registerRequest struct {
	Version string `json:"version"`
}

func NewConnectorManagerClient(ctx context.Context, config ConnectorManagerClientConfig) (c ConnectorManagerClient) {
	c.httpClient = http.DefaultClient
	if config.Insecure {
		transport := http.DefaultTransport
		transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // optional insecure
		c.httpClient = &http.Client{Transport: transport}
	}
	c.url = config.URL
	c.apiKey = config.APIKey
	return
}

var _ events.Notifier = &ConnectorManagerClient{}

func (c ConnectorManagerClient) NewConsoleEventHandler(logLeveler slog.Leveler, unresolvedError map[events.ErrorEventType]string) *events.Handler {
	return events.NewHandler(c, logLeveler, unresolvedError)
}

type RegistrationInfo struct {
	Stopped          bool                             `json:"stopped"`
	Config           any                              `json:"config"`
	UnresolvedErrors map[events.ErrorEventType]string `json:"unresolved_errors"`
}

func (c ConnectorManagerClient) Register(ctx context.Context, version string, info *RegistrationInfo) (err error) {
	registerReq := registerRequest{
		Version: version,
	}
	err = c.call(ctx, http.MethodPost, "register", registerReq, info)
	if err != nil {
		return
	}
	return
}

type getConfigResponse struct {
	Config json.RawMessage `json:"config"`
}

func (c ConnectorManagerClient) getConfig(ctx context.Context) (config json.RawMessage, err error) {
	resp := new(getConfigResponse)
	err = c.call(ctx, http.MethodGet, "config", nil, &resp)
	if err != nil {
		return
	}
	config = resp.Config
	return
}

func (c ConnectorManagerClient) Start(ctx context.Context, connector Connector) {
	logger.Debug("start connector")
	tasks := c.tasks(ctx)
	for {
		select {
		case task, chanOpened := <-tasks:
			if !chanOpened {
				logger.Warn("tasks channel is closed")
				return
			}
			logger.Debug("received tasks", "task", task)

			var taskError string
			switch task.Action {
			case ActionUpdateConfig:
				config, err := c.getConfig(ctx)
				if err != nil {
					taskError = fmt.Sprintf("error cannot get updated config, error : %v\n", err)
					break
				}
				err = connector.Configure(ctx, config)
				if err != nil {
					taskError = fmt.Sprintf("error reconfiguring connector, error: %v\n", err)
				}
			case ActionStop:
				if connector.Status() == Stopped {
					taskError = "error stopping connector, error: connector is already stopped"
					break
				}
				err := connector.Stop(ctx)
				if err != nil {
					taskError = fmt.Sprintf("error stopping connector, error: %v\n", err)
				}
			case ActionStart:
				if connector.Status() == Started {
					taskError = "error starting connector, error: connector is already started"
					break
				}
				err := connector.Start(ctx)
				if err != nil {
					taskError = fmt.Sprintf("error start connector, error: %s", err)
				}
			case ActionRestore:
				restoreAction := new(RestoreActionContent)
				err := json.Unmarshal(task.Content, restoreAction)
				if err != nil {
					taskError = fmt.Sprintf("error reading restore task, error: %v\n", err.Error())
					break
				}
				if restoreAction.ID == "" {
					taskError = "error reading restore task, the id of the element to restore is not provided"
					break
				}
				err = connector.Restore(ctx, *restoreAction)
				if err != nil {
					taskError = fmt.Sprintf("error restoring element %s, error: %s\n", restoreAction.ID, err.Error())
					logger.Error(taskError)
				}
			}
			event := events.TaskEvent{
				TaskID: task.ID,
				Error:  taskError,
			}
			err := c.Notify(ctx, event)
			switch {
			case errors.Is(err, ErrUnauthorizedConnector):
				return
			case err != nil:
				logger.Error("could not push event to ack task", slog.String("task-id", task.ID))
			}
		case <-ctx.Done():
			logger.Warn("context done", slog.String("reason", ctx.Err().Error()))
			// TODO
			return
		}
	}
}

type postEventRequest struct {
	EventType events.EventType `json:"type"`
	Event     json.RawMessage  `json:"event"`
}

func (c ConnectorManagerClient) Notify(ctx context.Context, event any) (err error) {
	reqBody := new(postEventRequest)
	switch event.(type) {
	case events.MitigationEvent:
		reqBody.EventType = events.Mitigation
	case events.TaskEvent:
		reqBody.EventType = events.TaskAck
	case events.LogEvent:
		reqBody.EventType = events.Log
	case events.ErrorEvent:
		reqBody.EventType = events.Error
	case events.ResolutionEvent:
		reqBody.EventType = events.Resolution
	default:
		err = errors.New("invalid type")
		return
	}
	rawEvent, err := json.Marshal(event)
	if err != nil {
		return
	}
	reqBody.Event = rawEvent
	err = c.call(ctx, http.MethodPost, "events", reqBody, nil)
	if err != nil {
		return
	}
	return
}

func (c ConnectorManagerClient) tasks(ctx context.Context) (tasks <-chan Task) {
	tasksC := make(chan Task, taskChannelBufferSize) // Buffer to allow multiple tasks to be queued
	tasks = tasksC
	go func(ctx context.Context, tasksC chan<- Task) {
		defer close(tasksC)
		for {
			select {
			case <-ctx.Done():
				logger.Warn("context done", "reason", ctx.Err())
				return
			default:
				tasks, err := c.getTasks(ctx)
				switch {
				case errors.Is(err, ErrUnauthorizedConnector):
					return
				case err != nil:
					logger.Error("cannot get tasks", slog.String("error", err.Error()))
					continue
				}
				for _, t := range tasks {
					tasksC <- t
				}
			}
		}
	}(ctx, tasksC)

	return
}

type getTasksResp struct {
	Tasks []Task `json:"tasks"`
}

func (c ConnectorManagerClient) getTasks(ctx context.Context) (tasks []Task, err error) {
	resp := new(getTasksResp)
	err = c.call(ctx, http.MethodGet, "tasks", nil, resp)
	if err != nil {
		return
	}
	tasks = resp.Tasks
	return
}

func (c ConnectorManagerClient) call(ctx context.Context, method string, path string, body any, res any) (err error) {
	reqBody, err := json.Marshal(body)
	if err != nil {
		return
	}
	req, err := c.prepareRequest(ctx, method, path, bytes.NewReader(reqBody))
	if err != nil {
		return
	}
	resp, err := c.retryDo(req)
	if err != nil {
		return
	}
	defer func() {
		if e := resp.Body.Close(); e != nil {
			logger.Warn("could not close response body properly", slog.String("error", e.Error()))
		}
	}()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		apiError := new(APIErrorResponse)
		err = json.Unmarshal(respBody, apiError)
		if err != nil {
			logger.Error("could not parse api error response", slog.String("error", err.Error()))
		}
		err = NewHTTPError(resp.StatusCode, respBody)
		switch apiError.Code {
		case InvalidAPIKeyCode:
			logger.Error("The API key is invalid. The connector may have been started with the wrong API key or has been deleted from the manager.")
		case RevokedAPIKeyCode:
			logger.Error("The API key has been revoked.")
		default:
			logger.Error("Could not connect to connector manager, unauthorized", slog.String("error", string(respBody)))
		}
		return errors.Join(ErrUnauthorizedConnector, err)
	default:
		err = NewHTTPError(resp.StatusCode, respBody)
		return
	}

	// we do not want to unmarshal if nil res is provide
	// we do not want to unmarshal if resp is no content
	if res == nil || resp.ContentLength == 0 {
		return
	}
	err = json.Unmarshal(respBody, res)
	if err != nil {
		return
	}
	return
}

func (c ConnectorManagerClient) retryDo(req *http.Request) (resp *http.Response, err error) {
	resp, err = backoff.Retry(
		req.Context(),
		func() (resp *http.Response, err error) {
			resp, err = c.httpClient.Do(req) //nolint:gosec // only called from code
			if err != nil {
				logger.Debug("try http request error", slog.String("error", err.Error()))
				return
			}
			if resp.StatusCode == http.StatusBadGateway {
				logger.Debug("try http request error", slog.String("error", "bad gateway"))
				err = errors.New("bad gateway")
				return
			}
			return
		},
		backoff.WithBackOff(backoff.NewExponentialBackOff()),
		backoff.WithMaxElapsedTime(3*time.Second),
	)
	return
}

func (c ConnectorManagerClient) prepareRequest(ctx context.Context, method string, path string, body io.Reader) (req *http.Request, err error) {
	reqURL, err := url.JoinPath(c.url, basePath, path)
	if err != nil {
		return
	}
	req, err = http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return
	}
	req.Header.Add("Authorization", "ApiKey "+c.apiKey)
	req.Header.Add("Content-Type", "application/json")
	v := ctx.Value(CtxRequestIDKey{})
	reqID, ok := v.(string)
	if !ok || reqID == "" {
		reqID = generateReqID()
	}
	req.Header.Add("X-Request-Id", reqID)
	return
}

func generateReqID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		logger.Error("cannot generate request id", slog.String("error", err.Error()))
		return ""
	}
	return hex.EncodeToString(b)
}
