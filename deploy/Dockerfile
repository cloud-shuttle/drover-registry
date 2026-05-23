# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o /out/drover-registry ./cmd/server

# Final minimal image
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata curl

WORKDIR /app
COPY --from=builder /out/drover-registry /app/drover-registry

# Default envs (override in compose / prod)
ENV DROVER_REGISTRY_SERVER_HOST=0.0.0.0 \
    DROVER_REGISTRY_SERVER_PORT=8080 \
    DROVER_REGISTRY_STORAGE_BACKEND=local \
    DROVER_REGISTRY_STORAGE_LOCAL_ROOT=/data/storage

VOLUME ["/data"]
EXPOSE 8080

ENTRYPOINT ["/app/drover-registry"]
CMD []
