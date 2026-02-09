# GeoFence Service

An IP geolocation service that validates user login locations by IP against customer-defined country whitelists.

## Overview

GeoFence is a microservice designed for Kubernetes deployment that provides REST and gRPC APIs for IP-based geolocation validation. The service uses MaxMind's GeoLite2 database to determine the country of origin for IP addresses and validates them against configurable country whitelists.

## Prerequisites

### Development Environment

- **Go**: >= 1.25.7 ([download](https://golang.org/dl/))
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
│   ├── data/              # Data access layer (MaxMind integration)
│   │   ├── lookup.go      # CountryLookup interface
│   │   └── mmdb_reader.go # MaxMind MMDB reader implementation
│   └── handler/           # REST and gRPC handlers
│       ├── health/        # Health check endpoints
│       ├── check/         # IP country check endpoints
│       └── grpc/          # gRPC service handler
├── deployments/
│   └── k8s/               # Kubernetes manifests
├── pkg/
│   └── geofence/
│       └── v1/            # Protobuf definitions and generated code
├── testdata/              # Test data (MMDB files)
├── Dockerfile             # Multi-stage Docker build
├── docker-compose.yaml    # Docker Compose for local testing
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
# Run with required MMDB path and defaults (HTTP 8080, gRPC 50051)
MMDB_PATH=./testdata/GeoLite2-Country-Test.mmdb go run cmd/geofence/main.go

# Or with custom configuration
LOG_LEVEL=debug PORT=3000 GRPC_PORT=50051 MMDB_PATH=./testdata/GeoLite2-Country-Test.mmdb \
  go run cmd/geofence/main.go
```

### 5. Test the Endpoints

#### Health Endpoints

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

#### API Endpoints

```bash
# Check if an IP is allowed for given countries
curl -X POST http://localhost:8080/api/v1/check \
  -H "Content-Type: application/json" \
  -d '{"ip":"216.160.83.56","allowed_countries":["US","CA"]}'
```

Example response (IP is from US, allowed):
```json
{
  "allowed": true,
  "country": "US",
  "error": ""
}
```

Example response (IP is from GB, not allowed for US/CA):
```json
{
  "allowed": false,
  "country": "GB",
  "error": ""
}
```

## Make Commands

A `Makefile` is provided for common development and deployment tasks. Run `make help` to see all available commands:

| Command | Description |
|---------|-------------|
| `make build` | Build the geofence binary (static Linux binary) |
| `make run` | Run the service locally with testdata MMDB pre-configured |
| `make test` | Run all tests with coverage reporting |
| `make test-unit` | Run unit tests only (fast, excludes integration tests) |
| `make test-integration` | Run integration tests only |
| `make coverage` | Generate HTML coverage report |
| `make fmt` | Format Go code with gofmt |
| `make vet` | Run go vet for code quality checks |
| `make clean` | Remove built artifacts and coverage files |
| `make docker-build` | Build Docker image (requires Docker) |
| `make docker-run` | Start service via docker-compose (requires Docker) |
| `make docker-down` | Stop docker-compose services (requires Docker) |
| `make proto` | Regenerate Go code from .proto files (requires: protoc, protoc-gen-go, protoc-gen-go-grpc) |
| `make deps-download` | Download Go module dependencies |
| `make deps-tidy` | Tidy Go module dependencies |
| `make all` | Full pipeline: fmt, vet, test, and build |

**Example workflow:**
```bash
make fmt        # Format code
make vet        # Check for issues
make test       # Run tests
make build      # Build binary
make docker-build  # Build Docker image
```

## Configuration

The service is configured via environment variables following the [12-factor app](https://12factor.net/) methodology:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `GRPC_PORT` | `50051` | gRPC server port |
| `LOG_LEVEL` | `info` | Logging level: `debug`, `info`, `warn`, `error` |
| `MMDB_PATH` | _(required)_ | Path to MaxMind MMDB file |

## Docker

### Build the Docker Image

```bash
docker build -t geofence:latest .
```

### Run with Docker Compose (Recommended for Development)

```bash
# Start the service with test database
docker-compose up --build

# Test the API
curl -X POST http://localhost:8080/api/v1/check \
  -H "Content-Type: application/json" \
  -d '{"ip":"216.160.83.56","allowed_countries":["US"]}'

# Stop the service
docker-compose down
```

### Run Docker Container Directly

```bash
docker run -d \
  -p 8080:8080 \
  -p 50051:50051 \
  -e PORT=8080 \
  -e GRPC_PORT=50051 \
  -e LOG_LEVEL=info \
  -e MMDB_PATH=/data/GeoLite2-Country-Test.mmdb \
  -v $(pwd)/testdata/GeoLite2-Country-Test.mmdb:/data/GeoLite2-Country-Test.mmdb:ro \
  geofence:latest
```

## Kubernetes Deployment

### Required Secrets

The service requires the following secrets to be provided as Kubernetes Secrets:

| Variable | Required | Description |
|----------|----------|-------------|
| `MAXMIND_ACCOUNT_ID` | Yes | Your MaxMind account ID (get from https://www.maxmind.com) |
| `MAXMIND_LICENSE_KEY` | Yes | Your MaxMind license key |

Configuration values (like `MMDB_PATH`, `LOG_LEVEL`, ports) are provided via ConfigMap.

### Creating and Applying Secrets

Create the secret using `kubectl`:

```bash
kubectl create secret generic geofence-secret \
  --from-literal=MAXMIND_ACCOUNT_ID=your-account-id \
  --from-literal=MAXMIND_LICENSE_KEY=your-license-key
```

Or define a manifest (note: **never commit to git**):

### Kubernetes Deployment

⚠️ Before deploying, review [docs/database-updates.md](docs/database-updates.md) for complete database and secret setup instructions.

1. **Create MaxMind Kubernetes Secret**

Use one of the recommended approaches below:

- **External Secrets Operator** (Recommended for production)
- **Sealed Secrets** (Good for git-based workflows)
- **kubectl create secret** (Development only)

See [database-updates.md: Initial Deployment - Step 1](docs/database-updates.md#step-1-create-kubernetes-secret-external-secrets-management) for detailed instructions.

2. **Apply Kubernetes manifests:**

```bash
kubectl apply -f deployments/k8s/configmap.yaml   # Create/update ConfigMap
kubectl apply -f deployments/k8s/deployment.yaml  # Deploy the application
kubectl apply -f deployments/k8s/service.yaml     # Expose the service
kubectl apply -f deployments/k8s/cronjob.yaml     # Deploy database auto-updater
```

**⚠️ Security Notice**: Never commit secrets to version control. Use external secret management (External Secrets Operator, Sealed Secrets, Vault, etc.) for production environments.

## API Reference

### POST /api/v1/check

Check if an IP address is from an allowed country.

**Request:**
```json
{
  "ip": "216.160.83.56",
  "allowed_countries": ["US", "CA"]
}
```

**Success Response (200 OK):**
```json
{
  "allowed": true,
  "country": "US",
  "error": ""
}
```

**Error Responses:**

- **400 Bad Request**: Invalid IP or missing/empty allowed_countries
```json
{
  "allowed": false,
  "country": "",
  "error": "invalid IP address"
}
```

- **500 Internal Server Error**: MMDB lookup failure
```json
{
  "allowed": false,
  "country": "",
  "error": "lookup failed"
}
```

## gRPC Reference

Service: `geofence.v1.GeofenceService`

### Check

**Request:**
```json
{
  "ip": "216.160.83.56",
  "allowed_countries": ["US", "CA"]
}
```

**Response:**
```json
{
  "allowed": true,
  "country": "US",
  "error": ""
}
```

**Example (grpcurl):**
```bash
grpcurl -plaintext \
  -proto pkg/geofence/v1/geofence.proto \
  -d '{"ip":"216.160.83.56","allowed_countries":["US"]}' \
  localhost:50051 geofence.v1.GeofenceService/Check
```

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

**Current Phase**: Phase 3 - gRPC & Kubernetes ✅

- [x] gRPC service definition and generated stubs
- [x] gRPC handler using CountryLookup
- [x] gRPC server with GRPC_PORT configuration
- [x] /ready endpoint validates MMDB access
- [x] Docker Compose exposes gRPC port
- [x] Kubernetes manifests (ConfigMap, Secret, Deployment, Service)

**Next Phase**: Phase 4 - Database Hot Reload — In-Process Atomic

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
Readiness probe endpoint for Kubernetes. Returns 503 if the MMDB is missing or cannot be read.

**Response**:
```json
{
  "status": "ready"
}
```

### POST /api/v1/check
IP geolocation validation endpoint.

**Request**:
```json
{
  "ip": "216.160.83.56",
  "allowed_countries": ["US", "CA"]
}
```

**Response (200)**:
```json
{
  "allowed": true,
  "country": "US",
  "error": ""
}
```

**Error Responses**:
- `400 Bad Request`: Invalid IP format or missing allowed_countries array
- `500 Internal Server Error`: MMDB lookup failure (logged with details)

## License

Copyright © 2026 TomasB

## Support

For issues and questions, please open a GitHub issue.
