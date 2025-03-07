# CLAUDE.md - Project Reference Guide

## Build & Run Commands
- Go build: `cd app && CGO_ENABLED=0 go build -v -o yt-text ./...`
- Go test: `cd app && go test -v -race ./...`
- Run specific test: `cd app && go test -v -run TestName ./path/to/package`
- Start dev server: `make docker-run` or `docker-compose -f docker/local/docker-compose.yml up --build`
- Start prod server locally: `make docker-prod` or `docker-compose -f docker/fly/docker-compose.yml up`
- Python CLI: `cd python && uv run scripts/ytext.py <youtube-url>`
- View dev logs: `make docker-logs`
- Stop dev server: `make docker-stop`
- Install Python dependencies: `cd python && uv sync`
- Update Python dependencies: `cd python && uv pip compile pyproject.toml -o requirements.txt`
- Run gRPC server: `make grpc-server` or `cd python && python -m scripts.api 50051`

See [DEPLOYMENT.md](DEPLOYMENT.md) for deployment-related commands.

## Code Style Guidelines
- Go: Use error wrapping with context (`fmt.Errorf("operation: %w", err)`)
- Python: Type hints required, Google-style docstrings
- Naming: camelCase for Go methods, snake_case for Python
- Imports: Standard library first, then third-party, then local
- Error handling: Custom AppError struct in Go, structured try/except in Python
- Logging: Use structured logging via logger package
- Comments: Document complex logic and public interfaces
- Python dependencies: Managed with uv and pyproject.toml

## Repository Structure
- `/app`: Go backend server with WebSockets for real-time updates
- `/docker`: Docker configuration files
  - `/local`: Development Docker configuration
  - `/fly`: Fly.io deployment configuration
- `/python`: Python scripts for transcription (gRPC service)
  - `/python/pyproject.toml`: Python package configuration managed with uv
  - `/python/requirements.txt`: Generated dependencies file
- `/static`: Frontend assets with WebSocket UI integration
- `/todo.md`: Current development priorities and future enhancements
- `/refactor.md`: Technical details of memory optimization implementation