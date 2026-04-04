FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o forge-bot ./cmd/forge-bot

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata && \
    adduser -D -u 1001 forge
USER forge
WORKDIR /app
COPY --from=builder /app/forge-bot .

EXPOSE 8085
HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://localhost:8085/health || exit 1
ENTRYPOINT ["./forge-bot"]
