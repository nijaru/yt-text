# Quick Start Guide

## 🚀 Fastest Way to Get Started

### 1. One-Command Setup
```bash
make setup
```
This will:
- Install `uv` if not present
- Install all dependencies
- Create necessary directories
- Ready to run!

### 2. Start the Server
```bash
make dev
```
Server will be running at http://localhost:8000

### 3. Transcribe a Video
```bash
make transcribe URL="https://www.youtube.com/watch?v=VIDEO_ID"
```

## 📋 Common Commands

| Command | Description |
|---------|-------------|
| `make help` | Show all available commands |
| `make dev` | Start development server (auto-reload) |
| `make serve` | Start production server |
| `make test` | Run tests |
| `make transcribe URL="..."` | Transcribe from command line |
| `make clean` | Clean temporary files |

## 🐳 Docker Quick Start

### Using Docker Compose (Recommended)
```bash
# Development
make docker-dev

# Production
make docker-prod
```

### Manual Docker
```bash
# Build
make docker-build

# Run
make docker-run
```

## 🎯 Direct Usage with uv

If you prefer using `uv` directly:

```bash
# Install dependencies
uv sync

# Start server
uv run litestar --app src.api.app:app run --reload

# Transcribe video
uv run transcribe "https://www.youtube.com/watch?v=VIDEO_ID"
```

## 🔧 Configuration

1. Copy the example environment file:
```bash
cp .env.example .env
```

2. Edit `.env` with your settings (optional - works with defaults)

## 📚 API Usage

### Submit a transcription job:
```bash
curl -X POST http://localhost:8000/api/transcribe \
  -H "Content-Type: application/json" \
  -d '{"url": "https://www.youtube.com/watch?v=VIDEO_ID"}'
```

### Check job status:
```bash
curl http://localhost:8000/api/jobs/{job_id}
```

### Get transcription result:
```bash
curl http://localhost:8000/api/jobs/{job_id}/result
```

## 🛠️ Troubleshooting

### Check if server is running:
```bash
make health
```

### View recent jobs:
```bash
make db-status
```

### Check configuration:
```bash
make config
```

### Performance test:
```bash
make perf-test
```

## 📦 Requirements

- Python 3.12+
- ffmpeg (for audio processing)
- 2GB+ RAM
- Optional: Apple Silicon Mac for MLX acceleration

## 🆘 Need Help?

- Run `make help` for all commands
- Check `docs/` for detailed documentation
- Open an issue on GitHub for bugs