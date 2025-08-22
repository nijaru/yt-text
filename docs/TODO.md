# TODO: Migration Tasks

## Immediate (Phase 2: Setup)

- [ ] Create new project structure under `src/`
- [ ] Write `pyproject.toml` with all dependencies
- [ ] Initialize Litestar app with basic routes
- [ ] Set up pytest and testing structure
- [ ] Configure ruff for linting
- [ ] Add pre-commit hooks

## Core Implementation (Phase 3)

### API Layer
- [ ] Create `src/api/app.py` with Litestar setup
- [ ] Port `/api/transcribe` endpoint
- [ ] Port `/api/jobs/{id}` endpoint  
- [ ] Add WebSocket support for live updates
- [ ] Implement rate limiting middleware
- [ ] Add CORS and security headers

### Services
- [ ] Create `TranscriptionService` with backend detection
- [ ] Implement `WhisperCPPBackend` class
- [ ] Implement `MLXWhisperBackend` for dev
- [ ] Add `OpenAIBackend` with usage limits
- [ ] Create `DownloadService` with streaming
- [ ] Add `CacheService` with diskcache

### Database
- [ ] Convert models to SQLModel
- [ ] Set up Alembic for migrations
- [ ] Create repository pattern classes
- [ ] Add database connection pooling

### Testing
- [ ] Unit tests for each service
- [ ] Integration tests for API endpoints
- [ ] Mock external dependencies
- [ ] Add performance benchmarks

## Optimization (Phase 4)

- [ ] Implement audio streaming pipeline
- [ ] Add job queue for background processing
- [ ] Optimize whisper.cpp model loading
- [ ] Add request/response compression
- [ ] Implement cache warming strategies
- [ ] Profile memory usage

## Deployment (Phase 5)

- [ ] Create multi-stage Dockerfile
- [ ] Write docker-compose for dev
- [ ] Add health check endpoints
- [ ] Configure for Railway/Fly
- [ ] Set up GitHub Actions CI/CD
- [ ] Add monitoring with Prometheus metrics

## Documentation

- [ ] API documentation with examples
- [ ] Deployment guide
- [ ] Development setup guide
- [ ] Performance tuning guide
- [ ] Troubleshooting guide

## Future Enhancements

- [ ] Add subtitle generation
- [ ] Support batch processing
- [ ] Add language detection
- [ ] Implement speaker diarization
- [ ] Add export formats (SRT, VTT, JSON)
- [ ] Create better frontend UI

## Notes

**Priority Order**:
1. Get basic Litestar app running
2. Implement whisper.cpp backend
3. Port existing functionality
4. Then optimize and enhance

**Testing Strategy**:
- Test each component in isolation
- Use mocks for external services
- Benchmark performance vs current system

**Risk Mitigation**:
- Keep old code until new version is stable
- Test on both M3 Max and 4090 systems
- Monitor memory usage carefully