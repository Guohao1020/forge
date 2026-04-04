# forge-core production Dockerfile
# Multi-stage build: compile in Go image, run in minimal Alpine
#
# Build: docker build -t forge-core:latest .
# Run:   docker run -p 8080:8080 --env-file .env forge-core:latest

# ---- Build stage ----
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

ENV GOPROXY=https://goproxy.cn,direct
ENV CGO_ENABLED=0
ENV GOFLAGS="-trimpath"

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN go build \
    -ldflags="-s -w -X main.version=$(date +%Y%m%d)" \
    -o /forge-core \
    ./cmd/forge-core

# ---- Runtime stage ----
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata git

# Create non-root user
RUN adduser -D -h /app forge
USER forge
WORKDIR /app

# Copy binary and migrations
COPY --from=builder /forge-core /usr/local/bin/forge-core
COPY --from=builder /app/migrations ./migrations

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s \
    CMD wget -qO- http://localhost:8080/api/auth/me || exit 1

ENTRYPOINT ["forge-core"]
