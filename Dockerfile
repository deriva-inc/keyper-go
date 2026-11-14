# syntax=docker/dockerfile:1

# ==============================================================================
# Stage 1: Build static binary
# ==============================================================================
FROM golang:1.26-alpine AS builder

ARG VERSION=dev
ARG GIT_COMMIT=none
ARG BUILD_DATE=unknown

# Install build essentials and certs
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Cache Go dependencies
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy application source code (including embedded migrations)
COPY . .

# Build statically compiled binary without CGO, stripping debug information (-s -w) and baking version info
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -extldflags '-static' \
              -X 'github.com/deriva-inc/keyper-go/config.Version=${VERSION}' \
              -X 'github.com/deriva-inc/keyper-go/config.GitCommit=${GIT_COMMIT}' \
              -X 'github.com/deriva-inc/keyper-go/config.BuildDate=${BUILD_DATE}'" \
    -trimpath \
    -o /app/keyper .

# ==============================================================================
# Stage 2: Distroless static runtime (minimal footprint & attack surface)
# ==============================================================================
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# Copy root CA certificates & timezone data from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy compiled self-contained binary (migrations are embedded)
COPY --from=builder /app/keyper /app/keyper

# Standard HTTP port
EXPOSE 8085

# Run with non-root security context (UID 65532)
USER nonroot:nonroot

ENTRYPOINT ["/app/keyper"]
