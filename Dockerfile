# syntax=docker/dockerfile:1.7
FROM golang:1.23 AS builder

WORKDIR /src

# Install build dependencies for SQLite
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    libc6-dev \
    && rm -rf /var/lib/apt/lists/*

COPY app/go.mod app/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY app/ ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux go build -ldflags='-w -s' -o /bin/main .

FROM python:3.12-slim-bookworm AS runner

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

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

COPY python/pyproject.toml ./
RUN --mount=type=cache,target=/root/.cache/uv \
    uv sync

COPY python/scripts ./scripts/
COPY --from=builder /bin/main /usr/local/bin/main
COPY static/ /app/static/

RUN mkdir -p /app/logs /app/data /tmp/transcribe \
    && chown -R appuser:appuser /app /tmp/transcribe \
    && chmod -R 755 /app/scripts/*.py \
    && chmod 1777 /tmp

USER appuser

# Set cache directory environment variables
ENV UV_CACHE_DIR=/tmp/uv-cache \
    XDG_CACHE_HOME=/tmp/cache \
    TRANSFORMERS_CACHE=/tmp \
    HF_HOME=/tmp \
    XDG_CACHE_HOME=/tmp

# Create cache directories with appropriate permissions
USER root
RUN mkdir -p /tmp/uv-cache && \
    mkdir -p /tmp/cache && \
    chown -R appuser:appuser /tmp/uv-cache && \
    chown -R appuser:appuser /tmp/cache && \
    chmod -R 755 /tmp/uv-cache && \
    chmod -R 755 /tmp/cache
USER appuser

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/main"]
