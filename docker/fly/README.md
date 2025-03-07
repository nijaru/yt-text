# Fly.io Deployment Configuration

This directory contains the Docker configuration for deploying the application to Fly.io.

## Configuration

The production environment is optimized for:
- **Resource Efficiency**: Memory and CPU constraints appropriate for Fly.io
- **Performance**: Optimized for reliable transcription with reasonable accuracy
- **Stability**: Health checks and automatic restarts
- **Security**: Read-only filesystem and security constraints

## Deployment

For detailed deployment instructions and commands, see [DEPLOYMENT.md](../../DEPLOYMENT.md).

Quick deployment:

```bash
make deploy
# or
fly deploy --config docker/fly/fly.toml
```

This will deploy the application using the configuration in `fly.toml`.

## Transcription Settings

Production environment uses optimized Whisper settings balancing accuracy and resource constraints:

- **Model**: `base.en` - efficient for production deployments on Fly.io
- **Beam Size**: 5 for improved accuracy while maintaining performance
- **VAD Filter**: Enabled with optimized parameters
  - `min_silence_duration_ms`: 300
  - `speech_pad_ms`: 100
  - `threshold`: 0.35
- **Temperature**: Dynamic fallback mechanism (0.0, 0.2, 0.4, 0.6, 0.8)
- **Memory Optimizations**:
  - `PYTORCH_CUDA_ALLOC_CONF=max_split_size_mb:32`
  - `MALLOC_TRIM_THRESHOLD_=100000`
  - `MALLOC_MMAP_THRESHOLD_=100000`
  - `MALLOC_ARENA_MAX=1`

## Resource Constraints

Fly.io deployment is configured with appropriate resource limits:
- **Memory**: 1.8GB maximum with 512MB reservation
- **CPU**: 1.0 CPU maximum with 0.25 CPU reservation
- **Storage**: Persistent volume for database storage

## Architecture

The production setup uses:
1. **Combined Service**: Both the Go application and Python transcription service run in the same container
2. **Persistent Volume**: Database is stored on a persistent volume
3. **HTTP Service**: Configured with health checks and HTTPS

## Scaling

The application is configured to scale based on demand:
- **Auto-start/stop**: Machines start and stop automatically
- **Min Running**: 1 machine minimum to ensure availability
- **Max Running**: 3 machines maximum for cost control
- **Concurrency**: 400-500 connections per machine

## Health Checks

Health check is configured to ensure the application is running properly:
- **Endpoint**: `/health`
- **Interval**: 30 seconds
- **Timeout**: 10 seconds
- **Grace Period**: 40 seconds for initial startup

## Security

The production container is configured with security in mind:
- **Read-only filesystem**: Prevents runtime modifications
- **No new privileges**: Limits potential security issues
- **Tmpfs**: Temporary filesystem with size and execution limits
- **User**: Runs as non-root user

## Monitoring and Troubleshooting

- View logs with `fly logs`
- Access console with `fly ssh console`
- Check status with `fly status`
- Scale manually with `fly scale count`