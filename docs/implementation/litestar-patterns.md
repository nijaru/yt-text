# Litestar Implementation Patterns

Reference patterns from Litestar documentation for our implementation.

## Database Dependency Injection

### Async Session Provider
```python
from sqlalchemy.ext.asyncio import AsyncSession
from litestar.di import Provide, dependency_cache
from typing import AsyncIterator

@dependency_cache
async def provide_session(state: State) -> AsyncIterator[AsyncSession]:
    async with state.db_session() as session:
        yield session

# Usage in routes
@post("/api/jobs")
async def create_job(
    data: TranscribeRequest,
    db_session: AsyncSession = Provide(provide_session)
) -> JobResponse:
    # Use session here
    pass
```

### SQLAlchemy Init Plugin Pattern
```python
from litestar.contrib.sqlalchemy.plugins import SQLAlchemyInitPlugin
from sqlalchemy.ext.asyncio import create_async_engine

# In app.py
app = Litestar(
    plugins=[
        SQLAlchemyInitPlugin(
            connection_url="sqlite+aiosqlite:///data/db.sqlite",
            create_all=True,  # Creates tables on startup
        )
    ]
)
```

### Transaction Management
```python
async def provide_transaction(db_session: AsyncSession) -> AsyncIterator[AsyncSession]:
    try:
        yield db_session
        await db_session.commit()
    except Exception:
        await db_session.rollback()
        raise
    finally:
        await db_session.close()
```

## Repository Pattern with Litestar

### Base Repository
```python
from litestar.repository.sqlalchemy import SQLAlchemyAsyncRepository

class JobRepository(SQLAlchemyAsyncRepository[TranscriptionJob]):
    model = TranscriptionJob
    
    async def get_pending_jobs(self) -> list[TranscriptionJob]:
        result = await self.session.execute(
            select(TranscriptionJob).where(
                TranscriptionJob.status == JobStatus.PENDING
            )
        )
        return result.scalars().all()
```

### Repository DI
```python
def provide_job_repository(db_session: AsyncSession) -> JobRepository:
    return JobRepository(session=db_session)

# In controller
class TranscriptionController(Controller):
    path = "/api"
    dependencies = {"repo": Provide(provide_job_repository)}
    
    @post("/transcribe")
    async def create_job(
        self, 
        data: TranscribeRequest,
        repo: JobRepository
    ) -> JobResponse:
        job = await repo.create(TranscriptionJob(**data.model_dump()))
        return JobResponse.from_orm(job)
```

## Lifespan Management

### Database Setup in Lifespan
```python
@asynccontextmanager
async def lifespan(app: Litestar) -> AsyncIterator[None]:
    # Startup
    engine = create_async_engine(settings.database_url)
    
    # Create tables
    async with engine.begin() as conn:
        await conn.run_sync(SQLModel.metadata.create_all)
    
    # Store in app state
    app.state.db_engine = engine
    app.state.db_session = sessionmaker(engine, class_=AsyncSession)
    
    yield
    
    # Shutdown
    await engine.dispose()
```

## Error Handling

### Custom Exception Handler
```python
def handle_app_exception(request: Request, exc: HTTPException) -> Response:
    error_response = ErrorResponse.create(
        code=exc.detail or "ERROR",
        message=str(exc),
        details={"status_code": exc.status_code},
    )
    return Response(
        content=error_response.model_dump(),
        status_code=exc.status_code,
    )

app = Litestar(
    exception_handlers={HTTPException: handle_app_exception}
)
```

## WebSocket Integration

### WebSocket with DB Access
```python
@websocket("/ws/jobs/{job_id:uuid}")
async def job_updates(
    socket: WebSocket,
    job_id: UUID,
    db_session: AsyncSession = Provide(provide_session)
) -> None:
    await socket.accept()
    
    # Send updates for job
    job = await db_session.get(TranscriptionJob, job_id)
    if job:
        while job.status in [JobStatus.PENDING, JobStatus.PROCESSING]:
            await socket.send_json({
                "status": job.status,
                "progress": job.progress
            })
            await asyncio.sleep(1)
            await db_session.refresh(job)
```

## Background Tasks

### Async Task Pattern
```python
from litestar.background_tasks import BackgroundTask

@post("/api/transcribe")
async def create_job(
    data: TranscribeRequest,
    repo: JobRepository
) -> JobResponse:
    job = await repo.create(TranscriptionJob(**data.model_dump()))
    
    # Start background transcription
    task = BackgroundTask(process_transcription, job_id=job.id)
    
    return JobResponse.from_orm(job)
```

## Configuration with Pydantic

### Settings Integration
```python
from pydantic_settings import BaseSettings

class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="APP_")
    
    database_url: str = "sqlite+aiosqlite:///data/db.sqlite"
    
    @computed_field
    @property 
    def is_production(self) -> bool:
        return self.env == "production"

settings = Settings()
```

## Key Differences from FastAPI

1. **Route Registration**: Pass handlers to Litestar constructor vs decorators
2. **Dependency Injection**: Use `Provide()` wrapper vs `Depends()`
3. **State Access**: Inject `State` vs accessing through request
4. **Plugins**: Rich plugin ecosystem vs middleware approach
5. **Type Safety**: Built-in serialization vs Pydantic integration