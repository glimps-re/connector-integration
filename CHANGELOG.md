# Changelog

## [Unreleased]

### Added

* `sdk/telemetry`: shared, logger-agnostic OpenTelemetry provider package (traces, metrics with a Prometheus `/metrics` endpoint, logs) with OTLP export and W3C trace-context propagation, configured from `OTEL_*`. Opt-in import: connectors that skip it pull in no OpenTelemetry dependency.
* `sdk/telemetry`: process-local trace correlation (`WithCorrelation`, `NewCorrelationSpanProcessor`).
* `sdk/telemetry/slogotel`: slog bridge — trace-id fields on stdout plus an OTLP log export.
* `sdk/gmconv`: canonical `gmalware.*` attribute keys and constructors, shared as a cross-service contract.

## [v0.9.0]

### Added

* HostConnector config : if size at or below `extract_min_size` (default to 8KB) file is directly send to analyse without extraction 

### Changed

* /metrics now return new quota

## [v0.8.3]

### Added

* custom enum validators

## [v0.8.2]

### Changed

* docs(host): clarify extract_workers field

## [v0.8.1]

### Fixed

* metrics:
  * add noop collecter
  * guard against nil detect client

## [v0.8.0]

### Added

* field AdditionalInfo in mitigation

## [v0.7.0]

### Added

* Connector metrics:
  * number of items processed
  * total size processed
  * number of mitigated items
  * number of items in error
  * daily quota
  * available daily quota
  * running since

## [v0.6.7]

### Fixed

* Correct host config fields descriptions

## [v0.6.6]

### Changed

* Update sharepoint config

## [v0.6.5]

### Added

* Cancelled status for archived pending task after a cancel

## [v0.6.4] 

### Added

* Console logger handle log attributes and groups

## [v0.6.3]

## Fixes

* fix host connector config

## [v0.6.2]

### Added

* Analysis error in mitigation

## [v0.6.1]

### Added

* host-connector: recursive extraction fields in config

## [v0.6.0]

### Added

* GMalware Expert URL in expert config

## [v0.5.1]

### Added

* SHA256 in mitigation info

## [v0.5.0] - 12-01-2026

### Added

* ICAP sampling configuration parameters (threshold, head size, tail size)

## [v0.4.1] - 12-01-2026

### Fixed

* fix get config

## [v0.4.0] - 09/01/2026

### Changed

* Need connector-manager > v0.4.0
* UpdateConfig task do not contains config anymore. Client will fetch config from console when task is received.

## [v0.3.1] - 23/12/2025

### Fixed

* Field "file" in mitigation renamed to be displayed by front

## [v0.3.0] - 12/12/2025

### Added

* Array object type in connectors config fields

## [v0.2.0] - 12/12/2025

### Added

* Filepath mitigation reason

## [v0.1.2] - 26/11/2025

### Fixed

* password config field

## [v0.1.1] - 19/11/2025

### Fixed

* log message when connector is already started

## [v0.1.0] - 19/11/2025

### Added

* Init with ICAP, SharePoint, Host, M365 (dev only)
