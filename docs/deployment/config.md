# Configuration

All configuration via environment variables with sensible defaults.

## Required Variables

```bash
# None - all have defaults
```

## Core Settings

```bash
# API Server
APP_HOST=0.0.0.0
APP_PORT=8000
APP_WORKERS=1  # Keep at 1 for small VPS
APP_ENV=production  # production, development, testing

# Database
APP_DATABASE_URL=sqlite+aiosqlite:///data/db.sqlite
APP_DATABASE_POOL_SIZE=5
APP_DATABASE_POOL_TIMEOUT=30

# Paths
APP_DATA_DIR=/app/data
APP_TEMP_DIR=/tmp/yt-text
APP_MODEL_DIR=/app/models
```

## Transcription Settings

```bash
# Model Selection
APP_WHISPER_MODEL=base  # tiny, base, small, medium, large
APP_WHISPER_LANGUAGE=auto  # or specific like 'en'
APP_WHISPER_DEVICE=auto  # auto, cpu, cuda, mps

# Backend Priority (comma-separated)
APP_TRANSCRIPTION_BACKENDS=whisper_cpp,mlx,openai

# API Fallback
APP_OPENAI_API_KEY=sk-...
APP_OPENAI_DAILY_LIMIT=100
APP_OPENAI_MODEL=whisper-1
```

## Performance Settings

```bash
# Rate Limiting
APP_RATE_LIMIT_ENABLED=true
APP_RATE_LIMIT_RPM=10  # Per IP
APP_RATE_LIMIT_DAILY=1000

# Timeouts
APP_REQUEST_TIMEOUT=30  # seconds
APP_DOWNLOAD_TIMEOUT=300
APP_TRANSCRIPTION_TIMEOUT=1800
APP_CLEANUP_INTERVAL=3600

# Resource Limits
APP_MAX_VIDEO_DURATION=14400  # 4 hours in seconds
APP_MAX_FILE_SIZE=2147483648  # 2GB in bytes
APP_MAX_CONCURRENT_JOBS=3
```

## Cache Settings

```bash
# DiskCache
APP_CACHE_ENABLED=true
APP_CACHE_DIR=/app/cache
APP_CACHE_SIZE_LIMIT=1073741824  # 1GB
APP_CACHE_TTL=604800  # 1 week

# Redis (optional)
APP_REDIS_URL=redis://localhost:6379/0
APP_REDIS_TTL=86400
```

## Security Settings

```bash
# CORS
APP_CORS_ORIGINS=*  # Comma-separated
APP_CORS_CREDENTIALS=true

# Security Headers
APP_SECURITY_HEADERS=true
APP_TRUSTED_HOSTS=*

# URL Filtering
APP_URL_ALLOWLIST=  # Comma-separated domains
APP_URL_BLOCKLIST=  # Comma-separated domains
```

## Logging

```bash
# Logging
APP_LOG_LEVEL=INFO  # DEBUG, INFO, WARNING, ERROR
APP_LOG_FORMAT=json  # json, console
APP_LOG_FILE=/app/logs/app.log
APP_LOG_MAX_SIZE=10485760  # 10MB
APP_LOG_BACKUP_COUNT=5
```

## Development Settings

```bash
# Development Only
APP_DEBUG=false
APP_RELOAD=false
APP_DOCS_ENABLED=true  # OpenAPI docs at /docs
```

## Docker Compose Example

```yaml
services:
  app:
    image: yt-text:latest
    environment:
      APP_ENV: production
      APP_PORT: 8000
      APP_WHISPER_MODEL: base
      APP_RATE_LIMIT_RPM: 10
      APP_CACHE_ENABLED: true
      APP_LOG_LEVEL: INFO
    volumes:
      - ./data:/app/data
      - ./cache:/app/cache
      - ./logs:/app/logs
    ports:
      - "8000:8000"
```

## Railway/Fly.io Variables

```bash
# Railway auto-injects
PORT=$PORT  # Use their port
DATABASE_URL=$DATABASE_URL  # If using Railway Postgres

# Fly.io
FLY_APP_NAME=$FLY_APP_NAME
FLY_REGION=$FLY_REGION
```

## Model Download

Models are downloaded on first use or pre-downloaded in Docker:

```bash
# Pre-download in Dockerfile
RUN python -c "from whisper_cpp import download_model; download_model('base')"
```

## Health Check

Health endpoint uses these to determine status:
- Database connection
- Available disk space
- Model availability
- Memory usage