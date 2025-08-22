"""Dependency injection providers for Litestar."""

from typing import AsyncIterator

from litestar.datastructures import State
from litestar.di import Provide
from sqlalchemy.ext.asyncio import AsyncSession

from src.lib.backends.factory import BackendFactory
from src.services.cache import CacheService
from src.services.download import DownloadService
from src.services.transcription import TranscriptionService


async def provide_db_session(state: State) -> AsyncIterator[AsyncSession]:
    """Provide database session."""
    async with state.db_session() as session:
        try:
            yield session
        finally:
            await session.close()


async def provide_download_service() -> DownloadService:
    """Provide download service."""
    return DownloadService()


async def provide_cache_service() -> CacheService:
    """Provide cache service."""
    return CacheService()


async def provide_backend_factory() -> BackendFactory:
    """Provide backend factory."""
    factory = BackendFactory()
    await factory.initialize()
    return factory


async def provide_transcription_service(
    db_session: AsyncSession = Provide(provide_db_session),
    download_service: DownloadService = Provide(provide_download_service),
    cache_service: CacheService = Provide(provide_cache_service),
    backend_factory: BackendFactory = Provide(provide_backend_factory),
) -> TranscriptionService:
    """Provide transcription service with all dependencies."""
    return TranscriptionService(
        db_session=db_session,
        download_service=download_service,
        cache_service=cache_service,
        backend_factory=backend_factory,
    )


# Dependency configuration for Litestar
dependencies = {
    "db_session": Provide(provide_db_session),
    "download_service": Provide(provide_download_service),
    "cache_service": Provide(provide_cache_service),
    "backend_factory": Provide(provide_backend_factory),
    "transcription_service": Provide(provide_transcription_service),
}