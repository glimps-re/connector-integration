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
	"github.com/glimps-re/connector-integration/sdk/metrics"
	"github.com/glimps-re/go-gdetect/pkg/gdetect"
	"github.com/google/uuid"
)

var LogLevel = &slog.LevelVar{}

var (
	consoleLogger = slog.New(slog.DiscardHandler)
	logger        = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: LogLevel}))
)

func main() {
	sdk.LogLevel.Set(slog.LevelDebug)
	consoleURL := getEnvVariableOrPanic("DUMMY_CONSOLE_URL", "console url")
	consoleAPIKey := getEnvVariableOrPanic("DUMMY_CONSOLE_API_KEY", "console apikey")
	gMalwareApiUrl := getEnvVariableOrPanic("GMALWARE_API_URL", "gmalware api url")
	gMalwareApiToken := getEnvVariableOrPanic("GMALWARE_API_TOKEN", "gmalware api token")
	consoleInsecure, err := strconv.ParseBool(os.Getenv("DUMMY_CONSOLE_INSECURE"))
	if err != nil {
		consoleInsecure = false
	}
	c := sdk.NewConnectorManagerClient(context.Background(), sdk.ConnectorManagerClientConfig{
		URL:      consoleURL,
		APIKey:   consoleAPIKey,
		Insecure: consoleInsecure,
	})
	config := &sdk.DummyConfig{
		ReconfigurableDummyConfig: sdk.ReconfigurableDummyConfig{
			CommonConnectorConfig: sdk.CommonConnectorConfig{
				GMalwareAPIURL:   gMalwareApiUrl,
				GMalwareAPIToken: gMalwareApiToken,
			},
		},
	}
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
	detectClient, err := gdetect.NewClientFromConfig(gdetect.ClientConfig{
		Endpoint: config.GMalwareAPIURL,
		Token:    config.GMalwareAPIToken,
		Insecure: config.GMalwareNoCertCheck,
	})
	if err != nil {
		panic(err)
	}
	dummy.metricCollecter = c.NewMetricCollecter(detectClient)
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
	metricCollecter  metrics.MetricCollecter
	gDetectSubmitter gdetect.ControllerGDetectSubmitter
}

func NewDummyConnector(apiURL string, apiToken string, stopped bool, dummyString string) (d *DummyConnector) {
	submitter, err := gdetect.NewClientFromConfig(gdetect.ClientConfig{Endpoint: apiURL, Token: apiToken})
	if err != nil {
		panic(err)
	}
	d = &DummyConnector{
		GMalwareAPIURL:   apiURL,
		GMalwareAPIToken: apiToken,
		DummyString:      dummyString,
		events:           make(chan any),
		quarantine:       make(map[string]bool),
		stopped:          stopped,
		eventHandler:     events.NoopEventHandler{},
		gDetectSubmitter: submitter,
	}
	return
}

func getEnvVariableOrPanic(variableName string, readableVariableName string) (value string) {
	value = os.Getenv(variableName)
	if value == "" {
		panic("Empty " + readableVariableName)
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

type FakeFileData struct {
	malwares []string
	label    string
	filetype string
	size     int64
}

func (d *DummyConnector) pushFileToQuarantine(ctx context.Context, fakeFileData FakeFileData) {
	fmt.Println("adding something to quarantine")
	id, err := genID()
	if err != nil {
		logger.Error("cannot generate id", slog.String("error", err.Error()))
		panic(err)
	}
	d.quarantine[id] = true
	err = d.eventHandler.NotifyFileMitigation(ctx, events.ActionQuarantine, id, events.ReasonMalware, events.FileInfos{
		CommonDetails: events.CommonDetails{
			Malwares:     fakeFileData.malwares,
			GmalwareURLs: []string{"fake.test.local"},
		},
		File:     fakeFileData.label,
		Filetype: fakeFileData.filetype,
	})
	if err != nil {
		logger.Error("cannot push quarantine", slog.String("error", err.Error()))
		consoleLogger.Error("cannot push quarantine", slog.String("error", err.Error()))
	}
	d.metricCollecter.AddItemProcessed(fakeFileData.size)
}

type FakeEmailData struct {
	malwares   []string
	subject    string
	sender     string
	recipients []string
	size       int64
}

func (d *DummyConnector) pushEmailToMitigation(ctx context.Context, fakeEmailData FakeEmailData) {
	err := d.emailMitigation(ctx, events.ActionBlock, events.ReasonPhishing, events.EmailInfos{
		CommonDetails: events.CommonDetails{
			Malwares:     fakeEmailData.malwares,
			GmalwareURLs: []string{"fake.test.local"},
		},
		Subject:    fakeEmailData.subject,
		Sender:     fakeEmailData.sender,
		Recipients: fakeEmailData.recipients,
	})
	if err != nil {
		logger.Error("cannot push email mitigation", slog.String("error", err.Error()))
		consoleLogger.Error("cannot push email mitigation", slog.String("error", err.Error()))
	}
	d.metricCollecter.AddItemProcessed(fakeEmailData.size)
}

func (d *DummyConnector) submitFileToDetect(ctx context.Context) {
	file, err := os.CreateTemp("", "dummy-sample-*.txt")
	if err != nil {
		panic(err)
	}
	filename := file.Name()
	defer func() { _ = os.Remove(filename) }() // best-effort cleanup of the temp sample
	_, err = file.WriteString("I'm a sample!")
	if err != nil {
		panic(err)
	}
	err = file.Close()
	if err != nil {
		panic(err)
	}
	_, err = d.gDetectSubmitter.SubmitFile(ctx, filename, gdetect.SubmitOptions{})
	if err != nil {
		logger.Error("cannot submit file to detect", slog.String("error", err.Error()))
		consoleLogger.Error("cannot submit file to detect", slog.String("error", err.Error()))
		return
	}
}

func getSpeedRate() (speedRate int) {
	speedRate, err := strconv.Atoi(os.Getenv("SPEED_RATE"))
	if err != nil {
		speedRate = 1
	}
	return
}

func sleep(seconds int) {
	speedRate := getSpeedRate()
	duration := time.Second * time.Duration(seconds*speedRate)
	time.Sleep(duration)
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
				d.pushFilesToQuarantine(ctx)
				d.submitFilesToDetect(ctx, 2)
				sleep(10)
				consoleLogger.Debug("trying a debug log")
				d.sendEmails(ctx)
				logger.Error("could not process file test.txt", slog.String("error", "some error happened"))
				d.metricCollecter.AddErrorItem()
				sleep(10)
				d.submitFilesToDetect(ctx, 4)
				d.notifyError(ctx)
				sleep(30)
				d.submitFilesToDetect(ctx, 3)
				d.notifyResolution(ctx)
				sleep(30)
			}
		}
	}(ctx)
}

func (d *DummyConnector) submitFilesToDetect(ctx context.Context, count int) {
	for range count {
		d.submitFileToDetect(ctx)
	}
}

func (d *DummyConnector) notifyResolution(ctx context.Context) {
	err := d.eventHandler.NotifyResolution(ctx, "resolved itself", events.GMalwareError)
	if err != nil {
		logger.Error("cannot push resolution event", slog.String("error", err.Error()))
		consoleLogger.Warn("cannot push resolution event", slog.String("error", err.Error()))
	}
}

func (d *DummyConnector) notifyError(ctx context.Context) {
	err := d.eventHandler.NotifyError(ctx, events.GMalwareError, errors.New("network error"))
	if err != nil {
		logger.Error("cannot push error event", slog.String("error", err.Error()))
		consoleLogger.Warn("cannot push error event", slog.String("error", err.Error()))
	}
}

func (d *DummyConnector) pushFilesToQuarantine(ctx context.Context) {
	d.pushFileToQuarantine(ctx, FakeFileData{malwares: []string{"test malware"}, label: "test.tst", filetype: "tst", size: 1024})
	d.pushFileToQuarantine(ctx, FakeFileData{malwares: []string{"huge Big Bang"}, label: "iAmSoSweet.zip", filetype: "zip", size: 5242880})
}

func (d *DummyConnector) sendEmails(ctx context.Context) {
	d.pushEmailToMitigation(ctx, FakeEmailData{
		malwares:   []string{"testphish"},
		subject:    "PeRsO truc truc perso",
		sender:     "tst@local.fr",
		recipients: []string{"truc@far.away"},
		size:       125,
	})
	d.pushEmailToMitigation(ctx, FakeEmailData{
		malwares:   []string{"testmalware"},
		subject:    "Very important thing ! open it",
		sender:     "yet.another@mail.en",
		recipients: []string{"prenom.nom@domain.fr", "azer.jklm@uiop.ee"},
		size:       614,
	})
	d.pushEmailToMitigation(ctx, FakeEmailData{
		malwares:   []string{"testother"},
		subject:    "Other important content",
		sender:     "yet.another@mail.en",
		recipients: []string{"mister.pouet@domain.fr", "azeryuiop.jklmqsdf@uiop.ee"},
		size:       2947,
	})
	d.pushEmailToMitigation(ctx, FakeEmailData{
		malwares:   []string{"testanother"},
		subject:    "Again an important thing ? don't wait !",
		sender:     "yet.another@mail.en",
		recipients: []string{"miss.pouet@domain.fr", "azerreza.jklmmlkj@uiop.ee"},
		size:       63,
	})
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
