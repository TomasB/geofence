# GeoFence Architecture

## Overview

GeoFence is a production-ready IP geolocation service deployed on Kubernetes with dual REST and gRPC APIs. The architecture emphasizes zero-downtime database updates, scalability, and operational reliability.

## Architecture Diagram

```
                          ┌─────────────────┐
                          │ REST Clients    │
                          │ POST /api/v1/   │
                          │  check          │
                          └────────┬────────┘
                                   │
                          ┌────────▼────────┐
                          │   K8s Service   │
                          │ :8080 ClusterIP │
                          └────────┬────────┘
                                   │
              ┌────────────────────┼────────────────────┐
              │                    │                    │
         ┌────▼────┐          ┌────▼────┐          ┌────▼────┐
         │  Pod 1  │          │  Pod 2  │          │  Pod 3  │
         ├─────────┤          ├─────────┤          ├─────────┤
         │REST:8080│          │REST:8080│          │REST:8080│
         │gRPC:500 │          │gRPC:500 │          │gRPC:500 │
         │51       │          │51       │          │51       │
         └────┬────┘          └────┬────┘          └────┬────┘
              │                    │                    │
              └────────────────────┼────────────────────┘
                                   │
                          ┌────────▼────────────┐
                          │ CountryLookup       │
                          │ Interface           │
                          └────────┬────────────┘
                                   │
                          ┌────────▼────────────┐
                          │ MmdbReader          │
                          │ atomic.Pointer<     │
                          │ geoip2.Reader>      │
                          │ [Thread-Safe]       │
                          └────────┬────────────┘
                                   │
                ┌──────────────────┼──────────────────┐
                │                  │                  │
         ┌──────▼──────┐    ┌──────▼──────┐   ┌──────▼──────┐
         │   File      │    │  MMDB File  │   │     PVC     │
         │  Watcher    │    │  (ReadOnly  │   │ (Shared     │
         │  (fsnotify) │    │  Reference) │   │  Volume)    │
         └──────┬──────┘    └──────┬──────┘   └─────────────┘
                │                  │
                └──────────┬───────┘
                           │
                   ┌───────▼────────┐
                   │  Hot Reload    │
                   │ (Zero Downtime)│
                   └────────────────┘

                    ┌─────────────────────────┐
                    │  K8s CronJob (Daily)    │
                    │ geofence-mmdb-updater   │
                    ├─────────────────────────┤
                    │ geoipupdate container   │
                    │ Downloads fresh MMDB    │
                    │ Writes to PVC           │
                    └────────────┬────────────┘
                                 │
                    ┌────────────┴───────────┐
                    │                        │
              ┌─────▼───────┐      ┌─────────▼────┐
              │K8s Secret   │      │ MaxMind API  │
              │ACCOUNT_ID   │      │updates.      │
              │LICENSE_KEY  │      │maxmind.com   │
              └─────────────┘      └──────────────┘
```

## Components

### API Layer
- **REST Handler** (`internal/handler/check/handler.go`)
  - Exposes `POST /api/v1/check` endpoint
  - Runs on configurable port (default: 8080)
  - Accepts JSON: `{"ip": "...", "allowed_countries": [...]}`

- **gRPC Handler** (`internal/handler/grpc/handler.go`)
  - Implements `GeofenceService.Check()` RPC
  - Runs on configurable port (default: 50051)
  - Identical logic to REST handler via shared `CountryLookup` interface

### Data Layer
- **CountryLookup Interface** (`internal/data/lookup.go`)
  - Defines contract for IP-to-country lookup
  - Enables testing with mock implementations
  - Enables future data source alternatives

- **MmdbReader** (`internal/data/mmdb_reader.go`)
  - Implements `CountryLookup` interface
  - Uses `atomic.Pointer[geoip2.Reader]` for thread-safe hot reloads
  - No request interruption during database updates
  - Graceful degradation: old database remains active if reload fails

### File Watching (Hot Reload)
- **fsnotify Integration**
  - Monitors parent directory for MMDB file changes
  - Handles both in-place writes and atomic rename-into-place
  - Triggers automatic reload on file modification
  - Disabled gracefully if watcher initialization fails

### Storage & Updates
- **Persistent Volume (ReadWriteMany)**
  - Shared across all deployment pods
  - Stores `GeoLite2-Country.mmdb`
  - Mounted at configurable path (default: `/data`)

- **CronJob Automation**
  - Runs `geoipupdate` container daily (default: 3 AM UTC)
  - Retrieves MaxMind account ID and license key from K8s Secret
  - Downloads fresh database to PVC
  - Service detects and reloads automatically

### Configuration
- **ConfigMap** (`deployments/k8s/configmap.yaml`)
  - Non-sensitive configuration
  - `LOG_LEVEL` (debug, info, warn, error)
  - `PORT` for REST server
  - `GRPC_PORT` for gRPC server

- **Secret** (External Secrets Operator / Sealed Secrets / Vault)
  - `MAXMIND_ACCOUNT_ID`: MaxMind account identifier
  - `MAXMIND_LICENSE_KEY`: MaxMind API key (rotated regularly)

## Request Flow

### REST API Request
```
Client → K8s Service (8080) → Load Balancer → Pod (REST Handler)
  → CountryLookup interface → MmdbReader (atomic.Pointer)
  → MaxMind MMDB file → IP Country lookup → Response
```

### gRPC Request
```
Client → K8s Service (50051) → Load Balancer → Pod (gRPC Handler)
  → CountryLookup interface → MmdbReader (atomic.Pointer)
  → MaxMind MMDB file → IP Country lookup → Response
```

### Database Update Flow
```
MaxMind API → CronJob (geoipupdate) → PVC (GeoLite2-Country.mmdb)
  → File Watcher (fsnotify) → Triggers hot reload
  → MmdbReader (atomic swap) → Zero-downtime reload → All pods updated
```

## Isolation & Thread Safety

### Pod Isolation
- Each pod has independent in-memory MmdbReader
- Shared PVC provides eventual consistency across pods
- No pod-to-pod synchronization needed
- Each pod reloads independently when detecting file changes

### Thread Safety
- `atomic.Pointer[geoip2.Reader]` ensures lock-free atomic swaps
- Multiple goroutines can read concurrently (no contention)
- Reload operation doesn't block active requests
- Zero downtime for database updates

## Failure Modes & Graceful Degradation

| Scenario | Behavior |
|----------|----------|
| File watcher fails to start | Service logs warning; hot-reload disabled; manual restart needed for updates |
| MMDB file corrupted during reload | Old reader retained; error logged; service continues |
| CronJob fails to download | Existing database remains in use; retry on next scheduled run |
| Pod crashes | K8s Deployment restarts; latest MMDB loaded |
| Storage failure (PVC unavailable) | Init container fails; pod enters CrashLoopBackOff; requires manual intervention |

## Scaling

### Horizontal Scaling
```
Deployment:
  replicas: 3        # Kubernetes scales pods
  Pod Anti-affinity  # Spread pods across nodes
  
Result:
  - Traffic distributed via K8s Service
  - Each pod independently watches MMDB for changes
  - All pods converge on same database version
```

### No Shared State
- No pod-to-pod communication needed
- No database connection pools
- Each pod is fully independent
- Scales linearly with request volume

## Performance Characteristics

- **Lookup Latency**: ~0.5ms per request (in-memory MMDB)
- **Memory Per Pod**: ~20-30MB (MMDB + Go runtime)
- **Concurrent Requests**: No limitations (lock-free)
- **Hot Reload Duration**: <100ms (atomic pointer swap)
- **Zero Request Failures**: During database updates

## Disaster Recovery

### Scenario 1: Corrupt MMDB File
1. Service continues with previous working database
2. Manual trigger of CronJob to re-download
3. File watcher detects new file and reloads
4. Automatic recovery without pod restart

### Scenario 2: Pod Crash
1. Kubernetes Deployment replaces pod
2. Init container validates MMDB file
3. Service resumes with last known-good database
4. No manual intervention needed

### Scenario 3: CronJob Failures
1. Service continues with existing database
2. Manual remediation: fix MaxMind credentials
3. Manually trigger CronJob to download
4. Automatic reload detected

## Security

- **Credentials Management**: MaxMind keys in K8s Secret (not in image)
- **RBAC**: Pod service account restricted to read Secrets
- **Network Policies**: Restrict pod egress to MaxMind API only (if enforced)
- **No File Exposure**: MMDB on read-only shared volume
- **Graceful Shutdown**: 30s timeout for in-flight requests

## Monitoring Integration Points

- **Readiness Probe** (`GET /ready`): Validates MMDB accessibility
- **Liveness Probe** (`GET /health`): Basic health check
- **Structured Logging** (slog): All events to stdout (log aggregation)
- **No Prometheus Metrics**: By design (intentionally minimal surface area)

## References

- [database-updates.md](database-updates.md): Operational runbooks and troubleshooting
- [README.md](../README.md): API documentation and local setup
- [PLAN.md](../PLAN.md): Implementation roadmap
