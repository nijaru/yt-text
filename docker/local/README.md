# Local Development Environment

This directory contains the Docker configuration for local development and testing.

## Configuration

The local environment is configured for faster iteration and development:

- **Model**: Uses `medium.en` Whisper model - balances speed and accuracy
- **Debug Mode**: Enabled for easier troubleshooting
- **Middleware**: Minimal middleware for faster response times
- **Resource Limits**: No strict resource limits to allow for easier debugging

## Running Locally

To start the development environment:

```bash
make docker-run
# or
docker-compose -f docker/local/docker-compose.yml up --build
```

This will build and start the containers defined in `docker-compose.yml`. The application will be available at http://localhost:8080.

See [DEPLOYMENT.md](../../DEPLOYMENT.md) for production deployment information.

## Transcription Settings

The local environment uses optimized transcription settings balancing speed and accuracy:

- **Model**: `medium.en` - faster than large models but more accurate than base
- **Beam Size**: 5 for improved transcription quality
- **VAD Filter**: Enabled to improve transcription of speech segments
- **Temperature**: Dynamic fallback system starting at 0.0 and increasing if needed
- **Batch Size**: Auto-optimized based on available system resources

## Container Architecture

The local setup consists of two main services:

1. **Main Application (app)**: 
   - The Go backend serving the HTTP API and WebSockets
   - Handles user requests and manages the database

2. **Transcription Service (grpc)**:
   - Python-based gRPC service for processing YouTube videos
   - Performs the transcription using Whisper

## Monitoring and Debugging

- Health checks are configured for both services
- Logs are available via `make docker-logs`
- Debug mode makes it easier to trace issues

## Customization

You can customize the environment by modifying the `docker-compose.yml` file. Common customizations:

- Change the Whisper model by updating the `WHISPER_MODEL` environment variable
- Adjust memory limits if you're experiencing performance issues
- Modify port mappings if you need to run the service on a different port