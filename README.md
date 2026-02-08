# GeoFence Service

An IP geolocation service that validates user login locations by IP against customer-defined country whitelists.

## Overview

GeoFence is a microservice designed for Kubernetes deployment that provides REST and gRPC APIs for IP-based geolocation validation. The service uses MaxMind's GeoLite2 database to determine the country of origin for IP addresses and validates them against configurable country whitelists.

## Prerequisites

### Development Environment

- **Go**: >= 1.22 ([download](https://golang.org/dl/))
- **Git**: for version control
- **Make** (optional): for build automation

### Runtime Dependencies

- **MaxMind GeoLite2 Database**: Country-level IP geolocation database
  - For development/testing, download the test database:
    ```bash
    curl -L -o testdata/GeoLite2-Country-Test.mmdb \
      https://github.com/maxmind/MaxMind-DB/raw/main/test-data/GeoLite2-Country-Test.mmdb
    ```
  - For production, sign up for a free MaxMind account at https://www.maxmind.com/en/geolite2/signup
  - Download the GeoLite2-Country database

## Project Structure

```
.
├── cmd/
│   └── geofence/          # Main application entry point
├── internal/
│   ├── api/               # API handlers (future: REST endpoints)
│   ├── app/
│   │   └── health/        # Health check endpoints
│   └── data/              # Data access layer (future: MaxMind integration)
├── deployments/
│   └── k8s/               # Kubernetes manifests (future phases)
├── testdata/              # Test data (MMDB files)
├── go.mod                 # Go module definition
├── go.sum                 # Go dependencies lock file
├── PLAN.md                # Implementation roadmap
└── README.md              # This file
```

## Running Locally

### 1. Clone and Setup

```bash
git clone https://github.com/TomasB/geofence.git
cd geofence
```

### 2. Download Test Database

```bash
mkdir -p testdata
curl -L -o testdata/GeoLite2-Country-Test.mmdb \
  https://github.com/maxmind/MaxMind-DB/raw/main/test-data/GeoLite2-Country-Test.mmdb
```

### 3. Install Dependencies

```bash
go mod download
```

### 4. Run the Service

```bash
# Run with default settings (port 8080, info log level)
go run cmd/geofence/main.go

# Or with custom configuration
LOG_LEVEL=debug PORT=3000 go run cmd/geofence/main.go
```

### 5. Test the Endpoints

```bash
# Health check (liveness probe)
curl http://localhost:8080/health

# Readiness check
curl http://localhost:8080/ready
```

Expected responses:
```json
{"status":"ok"}
{"status":"ready"}
```

## Configuration

The service is configured via environment variables following the [12-factor app](https://12factor.net/) methodology:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `LOG_LEVEL` | `info` | Logging level: `debug`, `info`, `warn`, `error` |
| `MMDB_PATH` | _(future)_ | Path to MaxMind MMDB file |

## Building

### Build Binary

```bash
go build -o bin/geofence ./cmd/geofence
./bin/geofence
```

### Run Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with verbose output
go test -v ./...
```

## Logging

The service uses structured logging (Go's `log/slog`) with JSON output to stdout/stderr:

- **Timestamps**: Included in all log entries
- **Log Levels**: debug, info, warn, error
- **Structured Fields**: All logs include relevant contextual data
- **No File Sinks**: All output to stdout/stderr only (12-factor compliant)

Example log entry:
```json
{"time":"2026-02-08T10:30:45Z","level":"INFO","msg":"request completed","method":"GET","path":"/health","status":200,"duration_ms":2}
```

## Graceful Shutdown

The service implements graceful shutdown:

- Listens for `SIGTERM` and `SIGINT` signals
- Allows in-flight requests to complete
- 30-second timeout for graceful shutdown
- Logs shutdown events for observability

## Development Status

**Current Phase**: Phase 1 - Project Setup & Health Endpoints ✅

- [x] Project initialization
- [x] Health endpoints (`/health`, `/ready`)
- [x] Structured logging with slog
- [x] Graceful shutdown
- [x] Unit tests

**Next Phase**: Phase 2 - REST API & MaxMind Integration

See [PLAN.md](PLAN.md) for the complete implementation roadmap.

## API Documentation

### Health Endpoints

#### GET /health
Liveness probe endpoint for Kubernetes.

**Response**:
```json
{
  "status": "ok"
}
```

#### GET /ready
Readiness probe endpoint for Kubernetes. Currently a placeholder; will verify MMDB availability in Phase 2.

**Response**:
```json
{
  "status": "ready"
}
```

## License

Copyright © 2026 TomasB

## Support

For issues and questions, please open a GitHub issue.
