# CLAUDE.md - Project Reference Guide

## Build & Run Commands
- Go build: `cd app && CGO_ENABLED=0 go build -v -o yt-text ./...`
- Go test: `cd app && go test -v -race ./...`
- Run specific test: `cd app && go test -v -run TestName ./path/to/package`
- Start dev server: `docker-compose up --build`
- Start prod server: `docker-compose -f docker-compose.prod.yml up`
- Python CLI: `cd python && uv run scripts/ytext.py <youtube-url>`

## Code Style Guidelines
- Go: Use error wrapping with context (`fmt.Errorf("operation: %w", err)`)
- Python: Type hints required, Google-style docstrings
- Naming: camelCase for Go methods, snake_case for Python
- Imports: Standard library first, then third-party, then local
- Error handling: Custom AppError struct in Go, structured try/except in Python
- Logging: Use structured logging via logger package
- Comments: Document complex logic and public interfaces

## Repository Structure
- `/app`: Go backend server
- `/python`: Python scripts for transcription
- `/static`: Frontend assets