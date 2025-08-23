# Multi-stage Dockerfile for yt-text
# Optimized for production deployment

# ========================================
# Stage 1: Builder
# ========================================
FROM python:3.12-slim AS builder

# Install uv for fast dependency installation
COPY --from=ghcr.io/astral-sh/uv:latest /uv /bin/uv

# Set working directory
WORKDIR /app

# Copy dependency files
COPY pyproject.toml uv.lock ./

# Install dependencies to a virtual environment
RUN uv sync --frozen --no-dev

# ========================================
# Stage 2: Runtime
# ========================================
FROM python:3.12-slim

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    curl \
    sqlite3 \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN useradd -m -u 1000 appuser && \
    mkdir -p /app /data /cache /models /logs /tmp/yt-text && \
    chown -R appuser:appuser /app /data /cache /models /logs /tmp/yt-text

# Set environment variables
ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PATH="/app/.venv/bin:$PATH" \
    PYTHONPATH=/app \
    APP_ENV=production \
    APP_HOST=0.0.0.0 \
    APP_PORT=8000

# Set working directory
WORKDIR /app

# Copy virtual environment from builder
COPY --from=builder --chown=appuser:appuser /app/.venv /app/.venv

# Copy application code
COPY --chown=appuser:appuser src/ /app/src/
COPY --chown=appuser:appuser static/ /app/static/
COPY --chown=appuser:appuser pyproject.toml /app/

# Switch to non-root user
USER appuser

# Create volume mount points
VOLUME ["/data", "/cache", "/models", "/logs"]

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8000/health/ || exit 1

# Expose port
EXPOSE 8000

# Run the application
CMD ["python", "-m", "litestar", "--app", "src.api.app:app", "run", "--host", "0.0.0.0", "--port", "8000"]