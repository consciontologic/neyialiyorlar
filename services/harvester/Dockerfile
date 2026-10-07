# Multi-stage build for Harvester service
# Stage 1: builder
FROM golang:1.25-alpine AS builder
WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy modules first (for layer caching)
COPY shared/ shared/
COPY harvester/ harvester/

# Build static binary with trimpath
RUN cd harvester && CGO_ENABLED=0 GOFLAGS=-trimpath go build \
    -ldflags="-s -w" \
    -o /tmp/harvester ./cmd/harvester

# Stage 2: runtime
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /tmp/harvester /app/harvester
ENTRYPOINT ["/app/harvester"]
