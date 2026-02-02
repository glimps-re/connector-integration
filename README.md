# Connector Integration SDK

Go SDK library for building connectors that integrate with the GLIMPS Connector manager.

## Overview

This SDK provides the methods for creating a connector that communicate with connector manager. Connectors can receive configuration updates, execute tasks, and report events back to the manager.

## Interface

Connectors must implement the `Connector` interface:

```go
type Connector interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Configure(ctx context.Context, config json.RawMessage) error
    Restore(ctx context.Context, restoreInfo RestoreActionContent) error
    Status() ConnectorStatus
}
```

## Metrics

The SDK automatically collects and pushes connector metrics to the console during each get tasks cycle. Some metrics are collected automatically, others require the connector to report them.

### Automatically collected metrics

- `mitigated_items`: incremented automatically when `NotifyFileMitigation`, `NotifyEmailMitigation` or `NotifyURLMitigation` is called
- `daily_quota` and `available_daily_quota`: retrieved automatically via Detect client passed to `NewMetricCollecter()`
- `running_since`: set automatically on `Register()`

### Connector-reported metrics

The connector must call `MetricCollecter` methods (obtained via `client.NewMetricCollecter(detectClient)`) to report:

- `AddItemProcessed(size int64)`: increments items processed count by 1 and total size processed by `size` bytes
- `AddErrorItem()`: increments error items count by 1

### Pushed metrics

example:  
```json
{
    "daily_quota": 100,
    "available_daily_quota": 75,
    "running_since": 1738000000,
    "items_processed": 10,
    "size_processed": 51200,
    "mitigated_items": 3,
    "error_items": 1
}
```

Metrics are always sent, even if nothing changed since last push.

## Add a connector

- Add your connector config under sdk/<connector>.go (also add it to `ConnectorConfig` interface under `sdk/loader.go`)
- Add your connector ID to `sdk/loader.go` consts (same as others), add  it to `validConnectorTypes` map in `sdk/validate.go`;
- Add your connector case to `InitDefault()`, `PatchConfig()` ;
- Add required files to `sdk/connectors/<connector>`:
    - `connector.yaml`: describe the connector (name, description, mitigation_info_type, setup_steps,launch_steps) ;
    - `logo.png`: your connector's logo ;
    - `docker-compose.yaml`: optional, your connector docker compose file, templated with console info (url, apikey) ;
    - `helm/`: optional, folder containing your connector helm chart and values, templated with console info (url, apikey) ;

## Usage

```go
// Initialize the client
client := sdk.NewConnectorManagerClient(ctx, sdk.ConnectorManagerClientConfig{
    URL:    "https://manager.example.com",
    APIKey: "your-api-key",
})

// Register with the manager
registerInfo := &sdk.RegistrationInfo{
    Config: yourConfig,
}
err := client.Register(ctx, "1.0.0", registerInfo)

// Start processing tasks
client.Start(ctx, yourConnector)

// Initialize event handler
eventHandler := client.NewConsoleEventHandler(slog.LevelDebug, registerInfo.UnresolvedErrors)

// Initialize console logger
consoleLogger = slog.New(eventHandler.GetLogHandler())

// Initialize metric collecter
metricCollecter := client.NewMetricCollecter(detectClient)
```
