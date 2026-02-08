# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy dependency files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /geofence ./cmd/geofence

# Runtime stage
FROM alpine:3.21

RUN apk --no-cache add ca-certificates wget

WORKDIR /app

COPY --from=builder /geofence /app/geofence

# Create data directory for MMDB
RUN mkdir -p /data

# Non-root user
RUN addgroup -g 1000 geofence && \
    adduser -D -u 1000 -G geofence geofence

USER geofence

# Default environment values
ENV PORT=8080 \
    GRPC_PORT=50051 \
    LOG_LEVEL=info \
    MMDB_PATH=/data/GeoLite2-Country.mmdb

# Health check
HEALTHCHECK --interval=10s --timeout=5s --retries=3 --start-period=5s \
    CMD wget --no-verbose --tries=1 --method=GET -O /dev/null http://localhost:${PORT}/health || exit 1

EXPOSE 8080 50051

ENTRYPOINT ["/app/geofence"]
