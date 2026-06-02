# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v1.1.0] - 2026-06-02

### Added
- **Service interface & UUID service management**: both the gRPC and HTTP servers
  now expose `Name()` and `SetServiceID()`, implementing the `goloader/service`
  interface so they can be managed by the service loader (#11).
- New `WithServiceName` option for configuring the gRPC server's service name (#11).
- `grpc.Config` now carries `ServiceName`, `ServiceVersion` and an embedded
  `telemetry.Config`, mirroring the HTTP server configuration (#12).

### Changed
- **Telemetry provider is now a process-wide singleton**. `NewProvider` is
  replaced by an idempotent `telemetry.Init(...)`, so multiple components (http,
  grpc, ...) can safely initialize telemetry — only the first caller's arguments
  take effect (#12).
- Global telemetry state access is now synchronized, and shutdown is guarded by a
  `sync.Once` (#12).
- Renamed the gRPC server's `Shutdown` method to `Stop` (#11).

### Fixed
- Avoid a panic when telemetry initialization fails; initialization failures are
  now returned as errors instead of crashing the process (#12).
- Corrected the wrong telemetry import path in `http/config.go` (#10).

[v1.1.0]: https://github.com/zixyos/giniservice/compare/v1.0.2...v1.1.0
