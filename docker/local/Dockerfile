FROM golang:1.24-alpine AS builder
WORKDIR /src

ENV GO111MODULE=on
ENV CGO_ENABLED=1

# Install required dependencies for Go build with CGO
RUN apk add --no-cache gcc musl-dev

COPY app/go.mod app/go.sum ./
RUN go mod download

COPY app/ ./
RUN go build -o /bin/main .

# Python dependencies stage - separate to improve caching
FROM python:3.12-slim-bookworm AS python-deps

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /deps

# Copy uv from its official image
COPY --from=ghcr.io/astral-sh/uv:latest /uv /bin/

# Copy requirements only
COPY python/pyproject.toml ./

# Use uv to install requirements in development mode
RUN uv pip install --system -r <(uv pip compile --extra=dev pyproject.toml)

FROM python:3.12-slim-bookworm

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PYTHONPATH=/app \
    GO_ENV=development \
    WHISPER_BEAM_SIZE=5 \
    WHISPER_BEST_OF=1 \
    WHISPER_TEMPERATURE=0.0 \
    WHISPER_VAD_FILTER=true \
    WHISPER_DOWNLOAD_ROOT=/tmp/models \
    ENABLE_CHUNK_CACHE=true \
    WHISPER_CACHE_DIR=/tmp/audio_cache \
    MAX_CACHE_SIZE_GB=2.0

# Install required system dependencies (minimal set for development)
RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    curl \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -r appuser && useradd -r -g appuser appuser

WORKDIR /app

# Copy Python dependencies from deps stage
COPY --from=python-deps /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages

# Create and set up directories
RUN mkdir -p /app/logs /app/data /tmp/transcribe /tmp/models /tmp/audio_cache \
    && chown -R appuser:appuser /app /tmp/transcribe /tmp/models /tmp/audio_cache \
    && chmod 1777 /tmp

# Copy application files
COPY python/scripts/ /app/scripts/
COPY --from=builder /bin/main /usr/local/bin/main
COPY static/ /app/static/

RUN chmod -R 755 /app/scripts/

# Add development tools for debugging
COPY --from=ghcr.io/astral-sh/uv:latest /uv /bin/

USER appuser

# Set up healthcheck for development
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/main"]
