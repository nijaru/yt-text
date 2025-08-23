.PHONY: help install dev serve test transcribe lint format clean docker-build docker-run setup

# Default target - show help
help:
	@echo "yt-text - Video Transcription Service"
	@echo ""
	@echo "Quick Start:"
	@echo "  make setup      - First time setup (install uv and dependencies)"
	@echo "  make dev        - Start development server (auto-reload)"
	@echo "  make transcribe - Transcribe a video from CLI"
	@echo ""
	@echo "Available Commands:"
	@echo "  make install    - Install dependencies with uv"
	@echo "  make dev        - Run development server on localhost:8000"
	@echo "  make serve      - Run production server"
	@echo "  make test       - Run basic functionality tests"
	@echo "  make test-cli   - Test CLI transcription with sample video"
	@echo "  make lint       - Run linter (ruff)"
	@echo "  make format     - Format code with ruff"
	@echo "  make clean      - Remove cache and temporary files"
	@echo ""
	@echo "Docker Commands:"
	@echo "  make docker-build - Build Docker image"
	@echo "  make docker-run   - Run Docker container"
	@echo "  make docker-dev   - Run with docker-compose (development)"
	@echo ""
	@echo "Examples:"
	@echo "  make transcribe URL='https://youtube.com/watch?v=VIDEO_ID'"
	@echo "  make transcribe URL='https://youtube.com/watch?v=VIDEO_ID' MODEL=large"

# First time setup - install uv if needed and dependencies
setup:
	@echo "🚀 Setting up yt-text..."
	@command -v uv >/dev/null 2>&1 || (echo "Installing uv..." && curl -LsSf https://astral.sh/uv/install.sh | sh)
	@echo "📦 Installing dependencies..."
	@uv sync
	@echo "📁 Creating directories..."
	@mkdir -p data logs cache models
	@echo "✅ Setup complete! Run 'make dev' to start the server."

# Install dependencies
install:
	uv sync

# Development server with auto-reload
dev:
	uv run litestar --app src.api.app:app run --reload --host 127.0.0.1 --port 8000

# Production server
serve:
	uv run litestar --app src.api.app:app run --host 0.0.0.0 --port 8000 --workers 4

# Run basic tests
test:
	uv run python test_basic.py

# Test CLI with sample video
test-cli:
	uv run transcribe "https://www.youtube.com/watch?v=uHm6FEb2Re4" -m base

# Transcribe a video (usage: make transcribe URL="https://...")
transcribe:
	@if [ -z "$(URL)" ]; then \
		echo "Error: Please provide a URL"; \
		echo "Usage: make transcribe URL='https://youtube.com/watch?v=VIDEO_ID'"; \
		exit 1; \
	fi
	@MODEL=$${MODEL:-base}; \
	echo "🎙️ Transcribing: $(URL)"; \
	echo "📊 Model: $$MODEL"; \
	uv run transcribe "$(URL)" -m "$$MODEL"

# Lint code
lint:
	uv run ruff check src/

# Format code
format:
	uv run ruff format src/

# Clean temporary files and cache
clean:
	@echo "🧹 Cleaning temporary files..."
	@rm -rf __pycache__ .pytest_cache .ruff_cache
	@find . -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
	@find . -type f -name "*.pyc" -delete 2>/dev/null || true
	@rm -rf /tmp/yt-text/* 2>/dev/null || true
	@echo "✅ Clean complete"

# Docker: Build image
docker-build:
	docker build -t yt-text:latest .

# Docker: Run container
docker-run:
	docker run -p 8000:8000 \
		-v $(PWD)/data:/app/data \
		-v $(PWD)/cache:/app/cache \
		-v $(PWD)/models:/app/models \
		yt-text:latest

# Docker: Development with docker-compose
docker-dev:
	docker-compose up --build

# Docker: Production with docker-compose
docker-prod:
	docker-compose -f docker-compose.prod.yml up -d

# Docker: Stop containers
docker-stop:
	docker-compose down

# Docker: View logs
docker-logs:
	docker-compose logs -f

# Show current configuration
config:
	@echo "📋 Current Configuration:"
	@echo "------------------------"
	@uv run python -c "from src.core.config import settings; import json; print(json.dumps(settings.model_dump(), indent=2, default=str))"

# Database: Show recent jobs
db-status:
	@echo "📊 Recent Transcription Jobs:"
	@sqlite3 data/db.sqlite "SELECT id, url, status, created_at FROM transcriptionjob ORDER BY created_at DESC LIMIT 10;" 2>/dev/null || echo "No database found. Start the server first."

# Quick health check
health:
	@curl -s http://localhost:8000/health/ | python -m json.tool || echo "Server not running"

# Performance test
perf-test:
	@echo "🏃 Running performance test..."
	@time uv run transcribe "https://www.youtube.com/watch?v=uHm6FEb2Re4" -m base
