# Implementation Patterns

## Service Layer Pattern

All business logic lives in services, not routes.

```python
# src/services/transcription.py
class TranscriptionService:
    def __init__(self, backends: list[TranscriptionBackend]):
        self.backends = backends
    
    async def transcribe(self, audio_path: Path) -> str:
        for backend in self.backends:
            if backend.is_available():
                return await backend.transcribe(audio_path)
        raise NoBackendAvailable()
```

## Backend Strategy Pattern

Swap transcription implementations without changing service code.

```python
# src/lib/backends/base.py
class TranscriptionBackend(ABC):
    @abstractmethod
    async def is_available(self) -> bool: ...
    
    @abstractmethod
    async def transcribe(self, audio_path: Path) -> str: ...

# src/lib/backends/whisper_cpp.py
class WhisperCPPBackend(TranscriptionBackend):
    async def is_available(self) -> bool:
        return Path("/usr/local/bin/whisper.cpp").exists()
```

## Dependency Injection

Use Litestar's DI for clean testing and configuration.

```python
# src/core/deps.py
async def get_db(state: State) -> AsyncSession:
    async with state.db() as session:
        yield session

# src/api/routes.py
@post("/api/transcribe")
async def create_job(
    request: TranscribeRequest,
    service: Annotated[TranscriptionService, Dependency()],
) -> JobResponse:
    return await service.create_job(request)
```

## Repository Pattern

Abstract database operations.

```python
# src/repository/base.py
class Repository(ABC, Generic[T]):
    @abstractmethod
    async def get(self, id: UUID) -> Optional[T]: ...
    
    @abstractmethod
    async def create(self, obj: T) -> T: ...

# src/repository/jobs.py
class JobRepository(Repository[Job]):
    def __init__(self, db: AsyncSession):
        self.db = db
    
    async def get_pending(self) -> list[Job]:
        return await self.db.scalars(
            select(Job).where(Job.status == "pending")
        )
```

## Error Handling

Consistent error handling with custom exceptions.

```python
# src/core/exceptions.py
class AppException(Exception):
    status_code: int = 500
    code: str = "INTERNAL_ERROR"
    
class RateLimitExceeded(AppException):
    status_code = 429
    code = "RATE_LIMIT_EXCEEDED"

# src/api/app.py
app = Litestar(
    exception_handlers={
        AppException: handle_app_exception,
    }
)
```

## Async Context Managers

Clean resource management.

```python
# src/services/download.py
class AudioDownloader:
    async def download_stream(self, url: str) -> AsyncIterator[bytes]:
        async with self._create_process(url) as proc:
            async for chunk in proc.stdout:
                yield chunk
```

## Configuration

Pydantic settings for type-safe config.

```python
# src/core/config.py
class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=".env",
        env_prefix="APP_",
    )
    
    database_url: str = "sqlite+aiosqlite:///db.sqlite"
    whisper_model: str = "base"
    rate_limit_rpm: int = 10
    
    @computed_field
    @property
    def redis_url(self) -> str:
        return f"redis://{self.redis_host}:{self.redis_port}"
```

## Testing Patterns

```python
# tests/conftest.py
@pytest.fixture
async def client(app: Litestar) -> AsyncIterator[AsyncTestClient]:
    async with AsyncTestClient(app=app) as client:
        yield client

# tests/test_api.py
async def test_create_job(client: AsyncTestClient, mock_service):
    mock_service.create_job.return_value = Job(id=...)
    response = await client.post("/api/transcribe", json={...})
    assert response.status_code == 201
```

## Streaming Response

For large transcriptions.

```python
@get("/api/jobs/{job_id}/stream")
async def stream_result(job_id: UUID) -> Stream:
    async def generator():
        async for chunk in service.stream_transcription(job_id):
            yield chunk
    
    return Stream(generator(), media_type="text/plain")
```