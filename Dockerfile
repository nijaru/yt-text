FROM golang:1.23 AS builder
WORKDIR /src

# More aggressive build-time memory limits
ENV GOMAXPROCS=2
ENV GOPROXY=direct
ENV GO111MODULE=on
ENV CGO_ENABLED=1
ENV GOGC=10
ENV GOMEMLIMIT=1024MiB

# Optimize dependency installation
COPY app/go.mod app/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Build with optimizations
COPY app/ ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build \
    -ldflags='-w -s' \
    -gcflags='-m=2' \
    -tags 'netgo osusergo static_build' \
    -trimpath \
    -o /bin/main .

FROM python:3.12-slim-bookworm AS runner

# Set environment variables for optimization
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    WHISPER_DOWNLOAD_ROOT=/tmp \
    NUMBA_CACHE_DIR=/tmp \
    PYTHONPATH=/app \
    MALLOC_TRIM_THRESHOLD_=100000 \
    MALLOC_MMAP_THRESHOLD_=100000 \
    PYTHONMALLOC=malloc \
    PYTORCH_CUDA_ALLOC_CONF=max_split_size_mb:32

RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    git \
    curl \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -r appuser && useradd -r -g appuser appuser

COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/

WORKDIR /app

# Install dependencies with caching
COPY python/pyproject.toml ./
RUN --mount=type=cache,target=/root/.cache/uv \
    uv pip install --system

# Copy application files
COPY python/scripts ./scripts/
COPY --from=builder /bin/main /usr/local/bin/main
COPY static/ /app/static/

# Set up directories and permissions
RUN mkdir -p /app/logs /app/data /tmp/transcribe \
    && chown -R appuser:appuser /app /tmp/transcribe \
    && chmod -R 755 /app/scripts/*.py \
    && chmod 1777 /tmp

USER appuser

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/main"]
