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
err := client.Register(ctx, "1.0.0", &sdk.RegistrationInfo{
    Config: yourConfig,
})

// Start processing tasks
client.Start(ctx, yourConnector)
```
