# CLAUDE.md - Project Reference Guide

## Build & Run Commands
- Go build: `cd app && CGO_ENABLED=0 go build -v -o yt-text ./...`
- Go test: `cd app && go test -v -race ./...`
- Run specific test: `cd app && go test -v -run TestName ./path/to/package`
- Start dev server: `docker-compose -f docker/local/docker-compose.yml up --build`
- Start prod server locally: `docker-compose -f docker/fly/docker-compose.yml up`
- Python CLI: `cd python && uv run scripts/ytext.py <youtube-url>`
- Deploy to Fly.io: `fly deploy --config docker/fly/fly.toml`

## Code Style Guidelines
- Go: Use error wrapping with context (`fmt.Errorf("operation: %w", err)`)
- Python: Type hints required, Google-style docstrings
- Naming: camelCase for Go methods, snake_case for Python
- Imports: Standard library first, then third-party, then local
- Error handling: Custom AppError struct in Go, structured try/except in Python
- Logging: Use structured logging via logger package
- Comments: Document complex logic and public interfaces

## Repository Structure
- `/app`: Go backend server with WebSockets for real-time updates
- `/docker`: Docker configuration files
  - `/local`: Development Docker configuration
  - `/fly`: Fly.io deployment configuration
- `/python`: Python scripts for transcription (gRPC service)
- `/static`: Frontend assets with WebSocket UI integration
- `/todo.md`: Current development priorities and future enhancements