FROM golang:1.23 AS builder
WORKDIR /src

ENV GO111MODULE=on
ENV CGO_ENABLED=1

COPY app/go.mod app/go.sum ./
RUN go mod download

COPY app/ ./
RUN go build -o /bin/main .

FROM python:3.12-slim-bookworm AS runner

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PYTHONPATH=/app \
    UV_CACHE_DIR=/tmp/uv-cache \
    XDG_CACHE_HOME=/tmp/cache \
    TRANSFORMERS_CACHE=/tmp \
    HF_HOME=/tmp

# Install required system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    git \
    curl \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -r appuser && useradd -r -g appuser appuser

COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/

WORKDIR /app

# Create cache directories
RUN mkdir -p /tmp/uv-cache && \
    mkdir -p /tmp/cache && \
    chmod -R 755 /tmp/uv-cache && \
    chmod -R 755 /tmp/cache

COPY python/pyproject.toml ./
RUN uv sync

COPY python/scripts ./scripts/
COPY --from=builder /bin/main /usr/local/bin/main
COPY static/ /app/static/

RUN mkdir -p /app/logs /app/data /tmp/transcribe \
    && chown -R appuser:appuser /app /tmp/transcribe \
    && chmod -R 755 /app/scripts/*.py \
    && chmod 1777 /tmp \
    && chown -R appuser:appuser /tmp/uv-cache \
    && chown -R appuser:appuser /tmp/cache

USER appuser

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/main"]
