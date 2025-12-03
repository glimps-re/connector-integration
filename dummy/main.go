package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/glimps-re/connector-integration/sdk"
	"github.com/glimps-re/connector-integration/sdk/events"
	"github.com/google/uuid"
)

var LogLevel = &slog.LevelVar{}

var (
	consoleLogger = slog.New(slog.DiscardHandler)
	logger        = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: LogLevel}))
)

func main() {
	sdk.LogLevel.Set(slog.LevelDebug)
	consoleURL := os.Getenv("DUMMY_CONSOLE_URL")
	if consoleURL == "" {
		panic("empty console url: " + consoleURL)
	}
	consoleAPIKey := os.Getenv("DUMMY_CONSOLE_API_KEY")
	if consoleAPIKey == "" {
		panic("empty console apikey: " + consoleAPIKey)
	}
	consoleInsecure, err := strconv.ParseBool(os.Getenv("DUMMY_CONSOLE_INSECURE"))
	if err != nil {
		consoleInsecure = false
	}
	c := sdk.NewConnectorManagerClient(context.Background(), sdk.ConnectorManagerClientConfig{
		URL:      consoleURL,
		APIKey:   consoleAPIKey,
		Insecure: consoleInsecure,
	})
	config := &sdk.DummyConfig{}
	info := &sdk.RegistrationInfo{
		Config: config,
	}
	err = c.Register(context.Background(), "1.0.0", info)
	if err != nil {
		panic(err)
	}
	if config.Debug {
		LogLevel.Set(slog.LevelDebug)
	}
	dummy := NewDummyConnector(config.GMalwareAPIURL, config.GMalwareAPIToken, info.Stopped, config.DummyString)
	dummy.eventHandler = c.NewConsoleEventHandler(LogLevel, info.UnresolvedErrors)
	consoleLogger = slog.New(dummy.eventHandler.GetLogHandler())
	dummy.Launch(context.Background())

	c.Start(context.Background(), dummy)
}

type DummyConnector struct {
	GMalwareAPIURL   string
	GMalwareAPIToken string
	DummyString      string
	events           chan any
	quarantine       map[string]bool
	stopped          bool
	eventHandler     events.EventHandler
}

func NewDummyConnector(apiURL string, apiToken string, stopped bool, dummyString string) (d *DummyConnector) {
	d = &DummyConnector{
		GMalwareAPIURL:   apiURL,
		GMalwareAPIToken: apiToken,
		DummyString:      dummyString,
		events:           make(chan any),
		quarantine:       make(map[string]bool),
		stopped:          stopped,
		eventHandler:     events.NoopEventHandler{},
	}
	return
}

func genID() (id string, err error) {
	u := uuid.New()
	content := u.String()
	h := sha256.New()
	_, err = h.Write([]byte(content))
	if err != nil {
		return
	}
	id = hex.EncodeToString(h.Sum(nil))
	return
}

func (d *DummyConnector) emailMitigation(ctx context.Context, action events.MitigationAction, reason events.MitigationReason, info events.EmailInfos) (err error) {
	id, err := genID()
	if err != nil {
		return
	}
	d.quarantine[id] = true
	err = d.eventHandler.NotifyEmailMitigation(ctx, action, id, reason, info)
	if err != nil {
		return
	}
	return
}

func (d *DummyConnector) Launch(ctx context.Context) {
	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				logger.Warn("connector stopped, context done")
				consoleLogger.Error("connector stopped, context done")
			default:
				if d.stopped {
					continue
				}
				consoleLogger.Info("adding something to quarantine", slog.String("root", "root value"), slog.GroupAttrs("sub", slog.String("test", "test value")))
				fmt.Println("adding something to quarantine")
				id, err := genID()
				if err != nil {
					logger.Error("cannot generate id", slog.String("error", err.Error()))
					panic(err)
				}
				d.quarantine[id] = true
				err = d.eventHandler.NotifyFileMitigation(ctx, events.ActionQuarantine, id, events.ReasonMalware, events.FileInfos{
					CommonDetails: events.CommonDetails{
						Malwares:     []string{"test malware"},
						GmalwareURLs: []string{"fake.test.local"},
					},
					File:     "test.tst",
					Filetype: "tst",
				})
				if err != nil {
					logger.Error("cannot push quarantine", slog.String("error", err.Error()))
					consoleLogger.Error("cannot push quarantine", slog.String("error", err.Error()))
				}
				time.Sleep(time.Second * 10)
				consoleLogger.Debug("trying a debug log")
				err = d.emailMitigation(ctx, events.ActionBlock, events.ReasonPhishing, events.EmailInfos{
					CommonDetails: events.CommonDetails{
						Malwares:     []string{"testphish"},
						GmalwareURLs: []string{"fake.test.local"},
					},
					Subject:    "PeRsO truc truc perso",
					Sender:     "tst@local.fr",
					Recipients: []string{"truc@far.away"},
				})
				if err != nil {
					logger.Error("cannot push quarantine", slog.String("error", err.Error()))
					consoleLogger.Error("cannot push quarantine", slog.String("error", err.Error()))
				}
				err = d.emailMitigation(ctx, events.ActionQuarantine, events.ReasonMalware, events.EmailInfos{
					CommonDetails: events.CommonDetails{
						Malwares:     []string{"testmalware"},
						GmalwareURLs: []string{"fake.test.local"},
					},
					Subject:    "Very important thing ! open it",
					Sender:     "yet.another@mail.en",
					Recipients: []string{"prenom.nom@domain.fr", "azer.jklm@uiop.ee"},
				})
				if err != nil {
					logger.Error("cannot push quarantine", slog.String("error", err.Error()))
					consoleLogger.Error("cannot push quarantine", slog.String("error", err.Error()))
				}
				err = d.emailMitigation(ctx, events.ActionBlock, events.ReasonMalware, events.EmailInfos{
					CommonDetails: events.CommonDetails{
						Malwares:     []string{"testother"},
						GmalwareURLs: []string{"fake.test.local"},
					},
					Subject:    "Other important content",
					Sender:     "yet.another@mail.en",
					Recipients: []string{"mister.pouet@domain.fr", "azeryuiop.jklmqsdf@uiop.ee"},
				})
				if err != nil {
					logger.Error("cannot push quarantine", slog.String("error", err.Error()))
					consoleLogger.Error("cannot push quarantine", slog.String("error", err.Error()))
				}
				err = d.emailMitigation(ctx, events.ActionBlock, events.ReasonMalware, events.EmailInfos{
					CommonDetails: events.CommonDetails{
						Malwares:     []string{"testanother"},
						GmalwareURLs: []string{"fake.test.local"},
					},
					Subject:    "Again an important thing ? don't wait !",
					Sender:     "yet.another@mail.en",
					Recipients: []string{"miss.pouet@domain.fr", "azerreza.jklmmlkj@uiop.ee"},
				})
				if err != nil {
					logger.Error("cannot push quarantine", slog.String("error", err.Error()))
					consoleLogger.Error("cannot push quarantine", slog.String("error", err.Error()))
				}

				time.Sleep(time.Second * 10)
				err = d.eventHandler.NotifyError(ctx, events.GMalwareError, errors.New("network error"))
				if err != nil {
					logger.Error("cannot push error event", slog.String("error", err.Error()))
					consoleLogger.Warn("cannot push error event", slog.String("error", err.Error()))
				}
				time.Sleep(time.Second * 30)
				err = d.eventHandler.NotifyResolution(ctx, "resolved itself", events.GMalwareError)
				if err != nil {
					logger.Error("cannot push resolution event", slog.String("error", err.Error()))
					consoleLogger.Warn("cannot push resolution event", slog.String("error", err.Error()))
				}
				time.Sleep(time.Second * 30)
			}
		}
	}(ctx)
}

func (d *DummyConnector) Start(ctx context.Context) (err error) {
	if d.DummyString == "error start" {
		return errors.New("error start")
	}
	d.stopped = false
	return
}

func (d *DummyConnector) Stop(ctx context.Context) (err error) {
	if d.DummyString == "error stop" {
		return errors.New("error stop")
	}
	d.stopped = true
	return
}

func (d *DummyConnector) Restore(ctx context.Context, restoreInfo sdk.RestoreActionContent) (err error) {
	fmt.Printf("restore %s\n", restoreInfo.ID)
	quarantined, ok := d.quarantine[restoreInfo.ID]
	if !ok || !quarantined {
		err = errors.New("error not in quarantine")
		return
	}
	return
}

func (d *DummyConnector) Configure(ctx context.Context, content json.RawMessage) (err error) {
	config := new(sdk.DummyConfig)
	err = json.Unmarshal(content, config)
	if err != nil {
		return
	}
	d.DummyString = config.DummyString
	d.GMalwareAPIToken = config.GMalwareAPIToken
	d.GMalwareAPIURL = config.GMalwareAPIURL
	if config.Debug {
		LogLevel.Set(slog.LevelDebug)
	} else {
		LogLevel.Set(slog.LevelInfo)
	}
	switch d.DummyString {
	case "wait":
		time.Sleep(5 * time.Minute)
	case "error config":
		err = d.eventHandler.NotifyError(ctx, events.GMalwareConfigError, errors.New("error config"))
		if err != nil {
			logger.Error("cannot push error event", slog.String("error", err.Error()))
			consoleLogger.Error("cannot push error event", slog.String("error", err.Error()))
		}
		return errors.New("configure error")
	}
	err = d.eventHandler.NotifyResolution(ctx, "configuration is now ok", events.GMalwareConfigError)
	if err != nil {
		logger.Error("cannot push resolution event", slog.String("error", err.Error()))
		consoleLogger.Error("cannot push resolution event", slog.String("error", err.Error()))
	}
	return
}

func (d *DummyConnector) Status() (status sdk.ConnectorStatus) {
	if d.stopped {
		return sdk.Stopped
	}
	return sdk.Started
}
