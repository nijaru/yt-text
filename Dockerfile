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
    VIRTUAL_ENV=/app/.venv \
    PATH="/app/.venv/bin:$PATH" \
    UV_CACHE_DIR=/tmp/uv-cache

# Install required system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    git \
    curl \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -r appuser && useradd -r -g appuser appuser

# Copy uv from its official image
COPY --from=ghcr.io/astral-sh/uv:latest /uv /bin/

WORKDIR /app

# Create and set up directories
RUN mkdir -p /tmp/uv-cache /app/logs /app/data /tmp/transcribe \
    && chown -R appuser:appuser /app /tmp/transcribe /tmp/uv-cache \
    && chmod -R 755 /tmp/transcribe

# Install dependencies in venv using uv
COPY python/pyproject.toml ./
RUN uv sync

# Copy application files
COPY python/scripts/*.py /app/scripts/
COPY --from=builder /bin/main /usr/local/bin/main
COPY static/ /app/static/

RUN chmod -R 755 /app/scripts/*.py

USER appuser

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/main"]
