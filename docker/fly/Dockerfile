# Production multi-stage Docker build
# Note: git is required for go mod download to work with Alpine

FROM golang:1.24-alpine AS builder
WORKDIR /src

ARG GOMEMLIMIT
ARG GOGC
ARG GOMAXPROCS

# Production build optimizations
ENV GOMAXPROCS=${GOMAXPROCS:-2}
ENV GOPROXY=direct
ENV GO111MODULE=on
ENV CGO_ENABLED=1
ENV GOGC=${GOGC:-10}
ENV GOMEMLIMIT=${GOMEMLIMIT:-1024MiB}
ENV GOAMD64=v3

# Install required dependencies for Go build with CGO
RUN apk add --no-cache gcc musl-dev git

COPY app/go.mod app/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY app/ ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build \
    -ldflags='-w -s' \
    -gcflags='-m=2' \
    -tags 'netgo osusergo static_build' \
    -trimpath \
    -mod=mod \
    -o /bin/main .

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

# Use uv to generate and install requirements
RUN --mount=type=cache,target=/root/.cache/uv \
    uv pip install --system -r <(uv pip compile pyproject.toml)

FROM python:3.12-slim-bookworm

# Set production environment variables
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    WHISPER_DOWNLOAD_ROOT=/tmp/models \
    PYTHONPATH=/app \
    MALLOC_TRIM_THRESHOLD_=100000 \
    MALLOC_MMAP_THRESHOLD_=100000 \
    PYTHONMALLOC=malloc \
    PYTORCH_CUDA_ALLOC_CONF=max_split_size_mb:32 \
    GOMAXPROCS=1 \
    GOGC=10 \
    WHISPER_BEAM_SIZE=5 \
    WHISPER_BEST_OF=1 \
    WHISPER_TEMPERATURE=0.0 \
    WHISPER_VAD_FILTER=true \
    WHISPER_VAD_MIN_SILENCE_MS=300 \
    WHISPER_VAD_SPEECH_PAD_MS=100 \
    WHISPER_VAD_THRESHOLD=0.35 \
    WHISPER_BATCH_SIZE=8 \
    ENABLE_CHUNK_CACHE=true \
    WHISPER_CACHE_DIR=/tmp/audio_cache \
    MAX_CACHE_SIZE_GB=5.0

# Install runtime dependencies only
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    curl \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -r appuser && useradd -r -g appuser appuser

WORKDIR /app

# Copy Python dependencies
COPY --from=python-deps /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages

# Copy only necessary application files
COPY python/scripts ./scripts/
COPY --from=builder /bin/main /usr/local/bin/main
COPY static/ /app/static/

# Create necessary directories with proper permissions
RUN mkdir -p /app/logs /app/data /tmp/transcribe /tmp/models /tmp/audio_cache \
    && chown -R appuser:appuser /app /tmp/transcribe /tmp/models /tmp/audio_cache \
    && chmod -R 755 /app/scripts/*.py \
    && chmod 1777 /tmp

USER appuser

# Add healthcheck
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Pre-download model in a separate layer to improve caching
RUN mkdir -p ${WHISPER_DOWNLOAD_ROOT} && \
    python -c "from faster_whisper import WhisperModel; WhisperModel('large-v3-turbo', device='cpu', download_root='${WHISPER_DOWNLOAD_ROOT}')"

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/main"]
