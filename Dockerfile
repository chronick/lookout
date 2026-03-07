# Multi-stage build for lookout — OTEL trace collector for AI workflows
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git

# Copy source
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build binary — TARGETARCH is set by buildx for multi-platform builds
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" \
    -o lookout ./cmd/lookout

# Runtime image — minimal Alpine
FROM alpine:3.19

LABEL org.opencontainers.image.title="Lookout" \
      org.opencontainers.image.description="OTEL trace collector for AI workflows" \
      org.opencontainers.image.version="1.0.0"

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/lookout /usr/local/bin/lookout

# Create data directory for SQLite
RUN mkdir -p /data

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD lookout query stats >/dev/null 2>&1 || exit 1

# Expose ports
# 4317 — OTLP gRPC receiver
# 4318 — OTLP HTTP receiver (traces + metrics)
# 4320 — Analytics API + Web UI
EXPOSE 4317 4318 4320

# Default to serve mode
ENV LOOKOUT_DB_PATH=/data/lookout.db \
    LOOKOUT_GRPC_ADDR=0.0.0.0:4317 \
    LOOKOUT_HTTP_ADDR=0.0.0.0:4318 \
    LOOKOUT_API_ADDR=0.0.0.0:4320

ENTRYPOINT ["lookout", "serve"]
