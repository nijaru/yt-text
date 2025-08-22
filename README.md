# yt-text

A high-performance video transcription service that downloads and transcribes videos from YouTube and other platforms. Built with Python, Litestar, and multiple Whisper backends for optimal accuracy and speed.

## Features

- 🎥 **Universal Video Support** - Download from YouTube, Vimeo, Twitter, TikTok, and 1000+ sites via yt-dlp
- 🚀 **Multiple Transcription Backends** - MLX (Apple Silicon), whisper.cpp, OpenAI API, and fallback Whisper
- ⚡ **Async Architecture** - Built on Litestar for high-performance async operations
- 💾 **Smart Caching** - Automatic caching of transcription results
- 🔄 **Real-time Updates** - WebSocket support for live transcription progress
- 🖥️ **CLI Tool** - Standalone command-line interface for quick transcriptions
- 📊 **Job Management** - Track transcription jobs with status, progress, and history

## Quick Start

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/nijaru/yt-text.git
   cd yt-text
   ```

2. Install dependencies with uv:
   ```bash
   uv sync
   ```

### Command Line Usage

Transcribe a video directly from the command line:

```bash
# Basic usage
uv run transcribe "https://www.youtube.com/watch?v=VIDEO_ID"

# Specify model and language
uv run transcribe "https://www.youtube.com/watch?v=VIDEO_ID" -m base -l en

# Save to file
uv run transcribe "https://www.youtube.com/watch?v=VIDEO_ID" -o transcript.txt
```

### Web Server

Start the web server for API access:

```bash
# Development server with auto-reload
uv run litestar --app src.api.app:app run --reload

# Production server
uv run litestar --app src.api.app:app run --host 0.0.0.0 --port 8000
```

## API Endpoints

### Submit Transcription Job
```http
POST /api/transcribe
Content-Type: application/json

{
  "url": "https://www.youtube.com/watch?v=VIDEO_ID",
  "model": "base",
  "language": "auto"
}
```

### Check Job Status
```http
GET /api/jobs/{job_id}
```

### Get Transcription Result
```http
GET /api/jobs/{job_id}/result
```

### WebSocket Updates
```javascript
ws://localhost:8000/ws/jobs/{job_id}
```

## Transcription Backends

The service automatically selects the best available backend:

1. **MLX Whisper** (Apple Silicon only)
   - Optimized for M1/M2/M3 Macs
   - Models: tiny, base, small, medium, large

2. **whisper.cpp** (Recommended for production)
   - High performance C++ implementation
   - Low memory usage with quantized models
   - Cross-platform support

3. **OpenAI API** (Cloud fallback)
   - Requires API key in environment
   - Usage limits configurable
   - High accuracy

4. **Fallback Whisper** (Reference implementation)
   - Original OpenAI Whisper
   - Always available as last resort

## Configuration

Create a `.env` file for custom configuration:

```env
# API Server
APP_PORT=8000
APP_ENV=production

# Database
APP_DATABASE_URL=sqlite+aiosqlite:///data/db.sqlite

# Transcription
APP_WHISPER_MODEL=base
APP_TRANSCRIPTION_BACKENDS=["whisper_cpp", "mlx", "openai"]

# OpenAI (optional)
APP_OPENAI_API_KEY=your-api-key
APP_OPENAI_DAILY_LIMIT=100

# Performance
APP_MAX_CONCURRENT_JOBS=3
APP_MAX_VIDEO_DURATION=14400  # 4 hours
APP_MAX_FILE_SIZE=2147483648  # 2GB
```

## Docker

Build and run with Docker:

```bash
# Build image
docker build -t yt-text .

# Run container
docker run -p 8000:8000 -v ./data:/app/data yt-text

# Or use docker-compose
docker-compose up
```

## Development

### Project Structure
```
yt-text/
├── src/
│   ├── api/          # Litestar web application
│   ├── core/         # Core models and configuration
│   ├── services/     # Business logic services
│   └── lib/          # Transcription backends
├── docs/             # Documentation
├── static/           # Frontend files
└── tests/            # Test suite
```

### Testing

Run the test suite:

```bash
# Basic functionality test
uv run python test_basic.py

# Full test suite (coming soon)
uv run pytest
```

## Performance

- **Transcription Speed**: ~7x realtime on Apple Silicon (19.7s for 142s video)
- **Memory Usage**: <2GB during transcription
- **Concurrent Jobs**: Configurable (default: 3)
- **Caching**: Automatic result caching with DiskCache

## Requirements

- Python 3.12+
- ffmpeg (for audio processing)
- 2GB+ RAM recommended
- Optional: Apple Silicon for MLX acceleration

## License

This project is licensed under the GNU Affero General Public License (AGPL) version 3. See the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Acknowledgements

- [yt-dlp](https://github.com/yt-dlp/yt-dlp) - Video downloading
- [OpenAI Whisper](https://github.com/openai/whisper) - Speech recognition models
- [MLX](https://github.com/ml-explore/mlx) - Apple Silicon acceleration
- [whisper.cpp](https://github.com/ggerganov/whisper.cpp) - High-performance C++ implementation
- [Litestar](https://github.com/litestar-org/litestar) - Modern async Python web framework