# Technical Specification

## System Architecture

### Stack Decision
**Pure Python with Litestar** - Removing Go layer for simplicity since heavy computation happens in Python.

### Core Technologies
- **Web Framework**: Litestar (async, fast, modern)
- **Package Manager**: uv (fast, modern Python tooling)
- **Transcription**: whisper.cpp (production), mlx-whisper (M3 dev), OpenAI API (fallback)
- **Database**: SQLite with SQLModel
- **Cache**: diskcache (simple, file-based)
- **Queue**: Background tasks via Litestar's built-in task queue
- **Deployment**: Docker on Railway/Fly.io

## Architecture Patterns

### Project Structure
```
yt-text/
├── src/
│   ├── api/          # HTTP layer
│   ├── core/         # Config, models, deps
│   ├── services/     # Business logic
│   └── lib/          # External integrations
├── tests/
├── static/           # Frontend files
└── docs/
```

### Service Layer Pattern
Each service handles one domain:
- `TranscriptionService` - Manages transcription backends
- `DownloadService` - yt-dlp operations
- `CacheService` - Result caching

### Dependency Injection
```python
# Use Litestar's DI
async def get_transcriber(state: State) -> TranscriptionService:
    return state.transcriber
```

## Transcription Strategy

### Backend Priority
1. **Development** (M3 Max): mlx-whisper for speed
2. **Production** (VPS): whisper.cpp with quantized models
3. **Fallback**: OpenAI API with rate limits

### Memory Optimization
- Stream audio processing (no full file in memory)
- Quantized models (ggml format)
- Process cleanup after each request
- 2GB RAM target for small VPS

## API Design

### Endpoints
- `POST /api/transcribe` - Submit URL, returns job ID
- `GET /api/jobs/{id}` - Check job status
- `GET /api/jobs/{id}/result` - Get transcription
- `WebSocket /ws/jobs/{id}` - Real-time updates

### Rate Limiting
- IP-based: 10 requests/minute
- API fallback: 100 requests/day total
- Configurable via environment

## Database Schema

### Tables (SQLModel)
```python
class TranscriptionJob:
    id: UUID
    url: str
    status: JobStatus
    model_used: str
    text: Optional[str]
    error: Optional[str]
    created_at: datetime
    completed_at: Optional[datetime]
    processing_time_ms: Optional[int]
```

## Performance Targets

- **Startup**: < 5 seconds
- **Memory**: < 2GB steady state
- **Latency**: < 200ms API response
- **Transcription**: ~1x realtime for base model
- **Concurrent Jobs**: 3 (limited by RAM)

## Security

- Input validation on all endpoints
- URL allowlist/blocklist support
- No direct file system access from API
- Sanitized filenames
- Rate limiting
- CORS configuration

## Error Handling

### Strategy
1. Graceful degradation (fallback transcription methods)
2. User-friendly error messages
3. Detailed logging for debugging
4. Automatic retry with exponential backoff

## Deployment

### Environments
- **Local**: Direct uv run
- **Development**: Docker with hot reload
- **Production**: Multi-stage Docker, minimal image

### Configuration
All config via environment variables with sensible defaults.
See `docs/deployment/config.md` for full list.