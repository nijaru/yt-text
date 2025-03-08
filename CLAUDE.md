# CLAUDE.md - Project Reference Guide

*Note: For the current status of in-progress build fixes, see [progress.md](progress.md)*

## Build & Run Commands
- Go build: `cd app && CGO_ENABLED=1 go build -v -o yt-text ./...` (using Go 1.24)
- Go test: `cd app && go test -v -race ./...`
- Run specific test: `cd app && go test -v -run TestName ./path/to/package`
- Build Docker image: `make docker-build` or `docker build --no-cache -t yt-text:latest -f docker/local/Dockerfile .`
- Start dev server: `make docker-run` or `docker-compose -f docker/local/docker-compose.yml up --build`
- Start prod server locally: `make docker-prod` or `docker-compose -f docker/fly/docker-compose.yml up`
- Python CLI: `cd python && uv run scripts/ytext.py <youtube-url>`
- View dev logs: `make docker-logs`
- Stop dev server: `make docker-stop`
- Install Python dependencies: `cd python && uv pip install -r requirements.txt`
- Install Python dev dependencies: `cd python && uv pip install -r dev-requirements.txt`
- Generate requirements file: `cd python && uv pip compile pyproject.toml -o requirements.txt`
- Generate dev requirements file: `cd python && uv pip compile pyproject.toml --extra dev -o dev-requirements.txt`
- For more uv commands and usage: See [uv-docs.md](../uv-docs.md)
- Run gRPC server: `make grpc-server` or `cd python && uv run scripts/grpc/start_server.py 50051`
- Generate Python gRPC files: `cd python && uv run -m grpc_tools.protoc -I./scripts/grpc --python_out=./scripts/grpc --grpc_python_out=./scripts/grpc ./scripts/grpc/transcribe.proto`
- Generate Go gRPC files: `cd app && go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest && protoc -I. --go_out=. --go-grpc_out=. ./protos/transcribe.proto`
- Run Python tests: `cd python && python -m pytest tests/`
- Run specific Python test: `cd python && python -m pytest tests/test_file.py::test_function -v`
- Generate test coverage: `cd python && python -m pytest tests/ --cov=scripts`
- Lint Python code: `cd python && uv run ruff check scripts/`
- Fix Python code style: `cd python && uv run ruff check --fix scripts/`
- Format Python code: `cd python && uv run ruff format scripts/`

See [DEPLOYMENT.md](DEPLOYMENT.md) for deployment-related commands.

## Code Style Guidelines
- Go: 
  - Use error wrapping with context (`fmt.Errorf("operation: %w", err)`)
  - Use structured logging with zerolog (`log.Error().Err(err).Str("key", "value").Msg("message")`)
  - Use Fiber's built-in error handling for HTTP errors (`return fiber.NewError(status, message)`)
- Python: Type hints required, Google-style docstrings
- Naming: camelCase for Go methods, snake_case for Python
- Imports: Standard library first, then third-party, then local
- Error handling: Custom AppError struct in Go, structured try/except in Python
- Logging: Use structured logging via logger package
- Comments: Document complex logic and public interfaces
- Python dependencies: Managed with uv and pyproject.toml
- Python code formatting: Using ruff format (configured in pyproject.toml)
- Python linting: Using ruff with comprehensive rules for error prevention
- Testing: Use pytest fixtures for test setup, mock external dependencies

## Repository Structure
- `/app`: Go backend server with WebSockets for real-time updates
- `/docker`: Docker configuration files
  - `/local`: Development Docker configuration (Python gRPC server only for now)
  - `/fly`: Fly.io deployment configuration
- `/python`: Python scripts for transcription (gRPC service)
  - `/python/pyproject.toml`: Python package configuration managed with uv
  - `/python/requirements.txt`: Generated dependencies file
  - `/python/scripts/`: Core Python implementation modules
  - `/python/tests/`: Test suite for Python modules
- `/static`: Frontend assets with WebSocket UI integration
- `/todo.md`: Current development priorities and future enhancements

## Docker Configuration
- Combined Docker container with both Go and Python services
- Go backend and Python gRPC server run as separate services in docker-compose
- Go uses Go 1.24 and requires CGO_ENABLED=1 for SQLite support
- Python uses Python 3.12 and the uv package manager
- The Go backend connects to the Python gRPC service using the container name "grpc" on port 50051

### Docker Build and Run Process
1. Build the Docker container with combined services:
   ```
   docker-compose -f docker/local/docker-compose.yml build
   ```

2. Run both services together:
   ```
   docker-compose -f docker/local/docker-compose.yml up
   ```

3. Access the web interface at http://localhost:8080

4. For development, you can run just one service if needed:
   ```
   docker-compose -f docker/local/docker-compose.yml up grpc
   ```
   or
   ```
   docker-compose -f docker/local/docker-compose.yml up app
   ```

### Important Build Configuration
- Go module uses a replace directive for protobuf imports: `replace yt-text/protos => ./protos`
- Docker builds use multi-stage approach with explicit `-mod=mod` flag for Go builds
- Python gRPC dependencies are installed explicitly during container build
- CGO is enabled for SQLite support in the Go application
- Git is required in the Alpine image for Go module downloads: `RUN apk add --no-cache gcc musl-dev git`

## Memory Optimization
- Audio processing uses pydub for streaming (5MB chunks)
- Resource cleanup happens after each chunk processing
- CUDA cache clearing implemented for GPU memory management
- Thread-safe progress tracking via queues
- Temporary files are cleaned up immediately after use