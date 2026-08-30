# syntax=docker/dockerfile:1
ARG GO_VERSION=1.27
ARG ALPINE_VERSION=3.21

# ---- build ----
FROM golang:${GO_VERSION}-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Pure-Go sqlite, no CGO; migrations embedded
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -trimpath -o /gosift ./cmd/app

# ---- runtime ----
FROM alpine:${ALPINE_VERSION}

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S app && adduser -S app -G app && \
    mkdir -p /data && chown app:app /data

COPY --from=builder /gosift /usr/local/bin/gosift

USER app
WORKDIR /app

# SQLite file lives on the /data volume
VOLUME ["/data"]

CMD ["gosift"]
