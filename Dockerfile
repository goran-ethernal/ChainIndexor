# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev sqlite-dev

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binaries
RUN make build-all

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite-libs tzdata

# Create non-root user
RUN addgroup -g 1000 indexer && \
    adduser -u 1000 -G indexer -s /bin/sh -D indexer

# Set working directory
WORKDIR /app

# Copy binaries from builder
COPY --from=builder /build/bin/indexer /app/indexer
COPY --from=builder /build/bin/indexer-gen /app/indexer-gen

# Copy example configs
COPY --from=builder /build/config.example.yaml /app/config.example.yaml
COPY --from=builder /build/config.example.json /app/config.example.json
COPY --from=builder /build/config.example.toml /app/config.example.toml

# Create data directory with proper permissions
RUN mkdir -p /app/data && chown -R indexer:indexer /app

# Switch to non-root user
USER indexer

# Expose metrics port (if enabled)
EXPOSE 9090

# Default command
ENTRYPOINT ["/app/indexer"]
CMD ["--config", "config.yaml"]
