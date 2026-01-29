# Airgapper - Multi-stage build
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download || true

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /airgapper ./cmd/airgapper

# Runtime image
FROM alpine:3.19

RUN apk add --no-cache \
    ca-certificates \
    restic \
    curl

# Create non-root user
RUN adduser -D -u 1000 airgapper
USER airgapper

WORKDIR /home/airgapper

# Copy binary
COPY --from=builder /airgapper /usr/local/bin/airgapper

# Config volume
VOLUME /home/airgapper/.airgapper

# Data volume (for backups)
VOLUME /data

# API port
EXPOSE 8080

ENTRYPOINT ["airgapper"]
CMD ["help"]
