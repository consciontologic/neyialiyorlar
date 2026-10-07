# Multi-stage build for Analytic service
# Stage 1: builder
FROM golang:1.23-alpine AS builder
WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy modules first (for layer caching)
COPY shared/ shared/
COPY analytic/ analytic/

# Build static binary with trimpath
RUN cd analytic && CGO_ENABLED=0 GOFLAGS=-trimpath go build \
    -ldflags="-s -w" \
    -o /tmp/analytic ./cmd/analytic

# Stage 2: runtime
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /tmp/analytic /app/analytic
ENTRYPOINT ["/app/analytic"]
