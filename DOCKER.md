# Docker Deployment Guide

This guide explains how to deploy ChainIndexor using Docker.

## Quick Start

### Using Docker Compose (Recommended)

1. **Prepare your configuration:**

```bash
# Copy example config
cp config.example.yaml config.yaml

# Edit with your settings
nano config.yaml
```

1. **Start the service:**

```bash
docker-compose up -d
```

1. **View logs:**

```bash
docker-compose logs -f chainindexor
```

1. **Stop the service:**

```bash
docker-compose down
```

### Using Docker Directly

**Build the image:**

```bash
docker build -t chainindexor:latest .
```

**Run the container:**

```bash
docker run -d \
  --name chainindexor \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  -v chainindexor-data:/app/data \
  -p 9090:9090 \
  chainindexor:latest
```

**View logs:**

```bash
docker logs -f chainindexor
```

**Stop and remove:**

```bash
docker stop chainindexor
docker rm chainindexor
```

## Configuration

### Environment Variables

You can override configuration using environment variables in `docker-compose.yml`:

```yaml
services:
  chainindexor:
    environment:
      - TZ=America/New_York
```

### Volumes

The Docker setup uses two types of volumes:

1. **Configuration** (read-only):

   ```yaml
   volumes:
     - ./config.yaml:/app/config.yaml:ro
   ```

2. **Data persistence**:

   ```yaml
   volumes:
     - chainindexor-data:/app/data
   ```

### Resource Limits

Adjust resource limits in `docker-compose.yml`:

```yaml
deploy:
  resources:
    limits:
      cpus: '4'
      memory: 4G
    reservations:
      cpus: '2'
      memory: 1G
```

## Using the Code Generator

Generate indexers using the included `indexer-gen` tool:

```bash
# Create output directory
mkdir -p ./indexers

# Generate indexer
docker run --rm \
  -v $(pwd)/indexers:/app/indexers \
  chainindexor:latest \
  /app/indexer-gen \
    --name ERC721 \
    --event "Transfer(address indexed from, address indexed to, uint256 indexed tokenId)" \
    --output /app/indexers/erc721
```

## Multi-Indexer Setup

Run multiple indexers in separate containers:

```yaml
version: '3.8'

services:
  erc20-indexer:
    build: .
    container_name: erc20-indexer
    volumes:
      - ./config-erc20.yaml:/app/config.yaml:ro
      - erc20-data:/app/data
    restart: unless-stopped

  erc721-indexer:
    build: .
    container_name: erc721-indexer
    volumes:
      - ./config-erc721.yaml:/app/config.yaml:ro
      - erc721-data:/app/data
    restart: unless-stopped

volumes:
  erc20-data:
  erc721-data:
```

## Production Deployment

### Health Checks

Add health checks to `docker-compose.yml`:

```yaml
services:
  chainindexor:
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:9090/metrics"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
```

Note: You'll need to add `wget` to the Dockerfile if using health checks.

### Logging

Configure logging driver:

```yaml
services:
  chainindexor:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

### Restart Policy

The default restart policy is `unless-stopped`. Other options:

```yaml
restart: "no"           # Never restart
restart: "always"       # Always restart
restart: "on-failure"   # Restart only on failure
```

## Monitoring

### Prometheus Metrics

Expose metrics port and configure Prometheus:

```yaml
services:
  chainindexor:
    ports:
      - "9090:9090"
```

Add to Prometheus config:

```yaml
scrape_configs:
  - job_name: 'chainindexor'
    static_configs:
      - targets: ['chainindexor:9090']
```

## Troubleshooting

### View logs

```bash
docker-compose logs -f chainindexor
```

### Execute commands in container

```bash
docker-compose exec chainindexor /bin/sh
```

### Check resource usage

```bash
docker stats chainindexor
```

### Inspect configuration

```bash
docker-compose exec chainindexor cat /app/config.yaml
```

### Reset data

```bash
docker-compose down -v  # Warning: deletes all data!
docker-compose up -d
```

## Security Best Practices

1. **Non-root user**: The image runs as a non-root user (`indexer:indexer`)
2. **Read-only config**: Mount config files as read-only (`:ro`)
3. **Network isolation**: Use Docker networks to isolate services
4. **Secrets management**: Use Docker secrets or environment files for sensitive data
5. **Regular updates**: Keep the image updated with latest security patches

## Advanced: Custom Build

### Build with specific Go version

```dockerfile
FROM golang:1.24-alpine AS builder
# ... rest of Dockerfile
```

### Build for different architectures

```bash
docker buildx build --platform linux/amd64,linux/arm64 -t chainindexor:latest .
```

### Multi-stage optimization

The Dockerfile uses multi-stage builds to minimize image size:

- Build stage: ~500MB (includes Go compiler and build tools)
- Runtime stage: ~30MB (only runtime dependencies)
