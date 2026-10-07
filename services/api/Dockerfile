# Multi-stage build for API service
# Stage 1: builder
FROM golang:1.25-alpine AS builder
WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy modules first (for layer caching)
COPY shared/ shared/
COPY api/ api/

# Build static binary with trimpath
RUN cd api && CGO_ENABLED=0 GOFLAGS=-trimpath go build \
    -ldflags="-s -w" \
    -o /tmp/api ./cmd/api

# Stage 2: runtime
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /tmp/api /app/api
ENTRYPOINT ["/app/api"]
