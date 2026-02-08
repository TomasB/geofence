# GeoFence Service - Implementation Plan

## Overview
Build a production-ready IP geolocation service that validates user login locations by IP against customer-defined country whitelists.

## Architecture
It's a microservice that is going to be deployed into an existing k8s cluster.
Follow 12factor.net for best practices

## Phase 1: Project Setup & Health Endpoints
- [ ] Initialize Go module (`go mod init github.com/TomasB/geofence`)
- [ ] Create standard project layout (`cmd/`, `internal/`, `deployments/`)
- [ ] Set up Gin HTTP framework with structured slog logging
- [ ] Implement `GET /health` (liveness) endpoint
- [ ] Implement `GET /ready` (readiness) endpoint (a placeholder for future)
- [ ] Implement unit test that tests apis
- [ ] Implement structured logging (slog), must contain at least timestamps and log levels; all output to **stdout/stderr only** (12-factor XI); write entries when the service is starting, started, shutting down; no file sinks, rotating logs, or syslog
- [ ] Implement graceful shutdown: handle SIGTERM, allow in-flight requests to complete within 30s timeout
- [ ] Create README.md including pre-requisites for dev environment, with instructions how to run the project locally

**Deliverables**: Structured project ready for REST API work, health endpoints functional

## Phase 2: REST API & MaxMind Integration
- [ ] Define `CountryLookup` interface in `internal/data/lookup.go` for composable data layer.
- [ ] Implement `MmdbReader` struct in `internal/data/mmdb_reader.go` satisfying `CountryLookup` interface.
- [ ] For development and testing purposes, load the sample MaxMind DB from `testdata/` (e.g. `GeoLite2-Country-Test.mmdb`). Do not commit MaxMind to repository. Add to README instructions where to download the file from. (https://github.com/maxmind/MaxMind-DB/blob/main/test-data/GeoLite2-Country-Test.mmdb)
- [ ] Use IP-Country MMDB and ISO-3166 country codes.
- [ ] HTTP REST API with single IP check endpoint (`POST /api/v1/check`).
- [ ] `POST /api/v1/check` will accept JSON: `{ "ip": "214.78.120.0", "allowed_countries": ["US","CA"] }`.
    - Example response: `{ "allowed": true, "country": "US", "error": "" }`.
- [ ] REST handler in `internal/handler/check/handler.go` accepts `CountryLookup` interface (not concrete MMDB reader).
- [ ] Log incoming request body only when `LOG_LEVEL=debug` (do not log request bodies at higher log levels).
- [ ] exports environment variables to enable configuration in Dockerfile:
    - `LOG_LEVEL`
    - `MMDB_PATH`
    - `PORT`
- [ ] Docker compose will specify `GeoLite2-Country-Test.mmdb` for `MMDB_PATH`
- [ ] Implement MaxMind MMDB integration & lookup (suggested library: `github.com/oschwald/geoip2-golang`).
- [ ] Handle MMDB access errors: log the error and return `500` for operational failures; return `400` for malformed input.
- [ ] Unit tests for core logic: write mock `CountryLookup` implementation; test handler with mock (no MMDB required). Integration tests should use a real MMDB (downloaded by a developer to /testdata).
- [ ] Docker containerization: multi-stage Go build, expose `PORT`, include `/ready` for probes.

**Deliverables**: REST API with composition-based data layer, unit tests pass (no DB required), Docker image, and CI checks

## Phase 3: gRPC & Kubernetes

- [ ] Create `pkg/geofence/v1/geofence.proto` with CheckRequest, CheckResponse, and GeofenceService with Check() RPCs.
- [ ] Generate gRPC code: `protoc --go_out=. --go-grpc_out=. pkg/geofence/v1/geofence.proto`.
- [ ] gRPC handler in `internal/handler/grpc/handler.go` accepts same `CountryLookup` interface (reuse from Phase 2; no code duplication).
- [ ] gRPC will launch on specified port in `GRPC_PORT` env variable
- [ ] Update `cmd/geofence/main.go`: start both REST (port 8080) and gRPC (port 50051) servers concurrently, handle graceful shutdown for both.
- [ ] Update `/ready` endpoint to succeed only when database connection is successfully created, and we are able to access data; return 503 if missing/corrupt.
- [ ] Create K8s manifests in `deployments/k8s/`:
  - `secret.yaml`: contains sensitive configuration: `MMDB_PATH` (path where init-container downloads MMDB), MaxMind license key/account ID for geoipupdate.
  - `deployment.yaml`: 3 replicas, init-container will download `GeoLite2-Country.mmdb` to a shared volume location, it will use Kubernetes Secret to retrieve any authentication keys required by MaxMind to access their lite version of database, liveness/readiness probes on `/health` and `/ready`.
  - `service.yaml`: expose port 8080 (REST, ClusterIP) and 50051 (gRPC, ClusterIP).
  - `configmap.yaml`: `LOG_LEVEL=info`, `PORT=8080`, `GRPC_PORT=50051`. Non-sensitive configuration only.
- [ ] Create `docker-compose.yaml` for local testing: both REST (8080) and gRPC (50051) using shared `./testdata/GeoLite2-Country-Test.mmdb`, loglevel set to info.
- [ ] Unit tests: verify both REST and gRPC APIs return identical results on mock `CountryLookup` and error handling.

**Deliverables**: K8s-ready service with gRPC support (shared `CountryLookup` logic with REST), both protocols tested and operational


## Phase 4: Database Hot Reload — In-Process Atomic

- [ ] Refactor `internal/data/mmdb_reader.go` to use `atomic.Pointer[*geoip2.Reader]` for thread-safe hot-swap (zero downtime).
- [ ] Implement file watcher in `internal/data/watcher.go`: monitor MMDB file on shared volume, trigger atomic reload on change.
- [ ] Create K8s manifest `deployments/k8s/cronjob.yaml`: geoipupdate container runs daily, writes fresh MMDB to shared PVC. MaxMind license key stored in K8s Secret.
- [ ] Update `internal/handler/health/handler.go`: `/ready` endpoint reports MMDB version/timestamp so ops can verify updates completed.
- [ ] Documentation in `docs/database-updates.md`: setup MaxMind credentials, CronJob configuration, verification procedures, rollback mechanism.

**Deliverables**: Automated daily database updates via CronJob, zero-downtime in-process reload, operational documentation

## Phase 5: Operational Excellence

- [ ] Load testing target baseline: **10 REQ/s, p99 <10ms, 1% error rate, 1000 concurrent users**.
  - Create `tests/load/check-api.js` (k6 script) for REST `/api/v1/check`.
  - Optionally add `tests/load/check-grpc.js` (ghz) for gRPC protocol.
  - Baseline test runs 50 concurrent users for 30s; measures latency histogram and error counts.
- [ ] No need to implementing metrics capturing, like prometheus and/or grafana
- [ ] Documentation in `docs/`:
  - Architecture diagram (REST + gRPC + K8s + database update flow)
  - API reference (all endpoints, auth requirements, error codes)
  - Operational runbooks: database update failure, high latency troubleshooting, pod churn investigation
  - On-call troubleshooting guide and escalation procedures

**Deliverables**: Production-hardened service with observability (metrics, dashboards), manual/automated database updates, operational runbooks, load test validated at baseline targets

## Dependencies
- Go >= 1.25
- Gin web framework (`github.com/gin-gonic/gin`)
- Docker & Docker Compose (local; for Phase 2+)
- Kubernetes cluster (staging/prod; for Phase 3+)
- `protoc` compiler for gRPC (Phase 3+)
- MaxMind license key (Phase 2+)
