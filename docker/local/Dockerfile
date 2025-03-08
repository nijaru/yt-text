# Multi-stage build with both Go and Python services
# Note: git is required in the Alpine image for go mod download to work properly

# 1. Go builder stage
FROM golang:1.24-alpine AS go-builder

WORKDIR /src

# Set environment variables for Go build
ENV GO111MODULE=on \
    CGO_ENABLED=1 \
    GOPROXY=direct

# Install required dependencies for Go build with CGO
RUN apk add --no-cache gcc musl-dev git

# Copy and download Go dependencies first for better caching
COPY app/go.mod app/go.sum ./
RUN go mod download

# Copy Go application source code
COPY app/ ./

# Build the Go application with explicit module paths
RUN go build -v -mod=mod -o /bin/yt-text .

# 2. Python dependencies stage
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
RUN uv sync --dev && uv pip install grpcio grpcio-tools protobuf

# 3. Main application stage
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

# Install required system dependencies 
RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    curl \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -r appuser && useradd -r -g appuser appuser

WORKDIR /app

# Copy Python dependencies from deps stage
COPY --from=python-deps /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages

# Copy Go binary from go-builder stage
COPY --from=go-builder /bin/yt-text /usr/local/bin/yt-text

# Create and set up directories
RUN mkdir -p /app/logs /app/data /tmp/transcribe /tmp/models /tmp/audio_cache \
    && chown -R appuser:appuser /app /tmp/transcribe /tmp/models /tmp/audio_cache \
    && chmod 1777 /tmp

# Copy application files
COPY python/scripts/ /app/scripts/
COPY static/ /app/static/

RUN chmod -R 755 /app/scripts/

# Add development tools for debugging
COPY --from=ghcr.io/astral-sh/uv:latest /uv /bin/

USER appuser

# Set up healthcheck for development
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Expose both the default Go HTTP port and gRPC port
EXPOSE 8080 50051

# We'll use the docker-compose command to specify which service to start
