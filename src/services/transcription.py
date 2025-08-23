"""Transcription service orchestrating download and transcription."""

import asyncio
import logging
from datetime import datetime
from typing import AsyncIterator, Optional
from uuid import UUID

logger = logging.getLogger(__name__)

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from src.core.config import settings
from src.core.models import JobStatus, JobPhase, TranscriptionJob
from src.lib.backends.base import TranscriptionBackend
from src.lib.backends.factory import BackendFactory
from src.services.cache import CacheService
from src.services.download import DownloadService


class TranscriptionService:
    """Main service for managing transcription jobs."""

    def __init__(
        self,
        db_session: AsyncSession,
        download_service: DownloadService,
        cache_service: CacheService,
        backend_factory: BackendFactory,
    ):
        self.db_session = db_session
        self.download_service = download_service
        self.cache_service = cache_service
        self.backend_factory = backend_factory
        self._running_jobs: dict[UUID, asyncio.Task] = {}

    async def create_job(
        self,
        url: str,
        model: str = "base",
        language: Optional[str] = None,
        ip_address: Optional[str] = None,
        user_agent: Optional[str] = None,
        start_processing: bool = True,
    ) -> TranscriptionJob:
        """Create a new transcription job."""
        # Validate URL
        if not self.download_service.is_supported_url(url):
            raise ValueError(f"Unsupported URL: {url}")

        # Check cache first
        if settings.cache_enabled:
            cached_result = await self.cache_service.get_transcription(url, model)
            if cached_result:
                # Create job with cached result
                job = TranscriptionJob(
                    url=url,
                    model_requested=model,
                    language=language,
                    status=JobStatus.COMPLETED,
                    text=cached_result["text"],
                    title=cached_result.get("title"),
                    duration=cached_result.get("duration"),
                    model_used=cached_result.get("model_used", model),
                    detected_language=cached_result.get("language"),
                    word_count=len(cached_result["text"].split()),
                    ip_address=ip_address,
                    user_agent=user_agent,
                    completed_at=datetime.utcnow(),
                    processing_time_ms=0,  # Cached result
                )
                self.db_session.add(job)
                await self.db_session.commit()
                await self.db_session.refresh(job)
                return job

        # Create pending job
        job = TranscriptionJob(
            url=url,
            model_requested=model,
            language=language,
            ip_address=ip_address,
            user_agent=user_agent,
        )
        self.db_session.add(job)
        await self.db_session.commit()
        await self.db_session.refresh(job)

        # Start background processing if requested
        if start_processing and len(self._running_jobs) < settings.max_concurrent_jobs:
            # Pass the session factory to create a new session for background task
            from src.api.app import app
            task = asyncio.create_task(self._process_job_with_new_session(job.id, app.state.db_session))
            self._running_jobs[job.id] = task
        
        return job

    async def get_job(self, job_id: UUID) -> Optional[TranscriptionJob]:
        """Get job by ID."""
        result = await self.db_session.execute(
            select(TranscriptionJob).where(TranscriptionJob.id == job_id)
        )
        return result.scalar_one_or_none()

    async def retry_job(self, job_id: UUID) -> Optional[TranscriptionJob]:
        """Retry a failed job."""
        job = await self.get_job(job_id)
        if not job or job.status != JobStatus.FAILED:
            return None

        # Reset job status
        job.status = JobStatus.PENDING
        job.error = None
        job.progress = 0
        job.started_at = None
        job.completed_at = None
        job.processing_time_ms = None
        
        await self.db_session.commit()

        # Start processing if slot available
        if len(self._running_jobs) < settings.max_concurrent_jobs:
            # Pass the session factory to create a new session for background task
            from src.api.app import app
            task = asyncio.create_task(self._process_job_with_new_session(job.id, app.state.db_session))
            self._running_jobs[job.id] = task

        return job

    async def check_backends(self) -> bool:
        """Check if transcription backends are available."""
        backend = await self.backend_factory.get_best_backend()
        return backend is not None

    async def stream_job_updates(
        self, job_id: UUID
    ) -> AsyncIterator[TranscriptionJob]:
        """Stream real-time job updates."""
        # Import here to avoid circular dependency
        from src.api.app import app
        
        # Use a new session for each query to see updates from other sessions
        while True:
            async with app.state.db_session() as session:
                result = await session.execute(
                    select(TranscriptionJob).where(TranscriptionJob.id == job_id)
                )
                job = result.scalar_one_or_none()
                
                if not job:
                    break
                
                yield job
                
                if job.status in [JobStatus.COMPLETED, JobStatus.FAILED]:
                    break
                    
            await asyncio.sleep(1)

    async def _process_job_with_new_session(self, job_id: UUID, session_factory) -> None:
        """Process a transcription job with a new database session."""
        async with session_factory() as session:
            # Create new service instances with the new session
            download_service = DownloadService()
            cache_service = CacheService()
            backend_factory = BackendFactory()
            await backend_factory.initialize()
            
            # Create a new service instance with its own session
            service = TranscriptionService(
                db_session=session,
                download_service=download_service,
                cache_service=cache_service,
                backend_factory=backend_factory,
            )
            
            # Store the session factory for progress updates
            service._session_factory = session_factory
            
            await service._process_job(job_id)
    
    async def _process_job(self, job_id: UUID) -> None:
        """Process a transcription job in the background."""
        try:
            job = await self.get_job(job_id)
            if not job:
                return

            start_time = datetime.utcnow()
            
            # Update to processing
            job.status = JobStatus.PROCESSING
            job.started_at = start_time
            job.phase = JobPhase.DOWNLOADING
            job.progress = 0
            await self.db_session.commit()

            # Download audio
            try:
                logger.info(f"Starting download for job {job_id}: {job.url}")
                
                # Simply download without progress tracking
                audio_info = await self.download_service.download_audio(job.url)
                logger.info(f"Download complete for job {job_id}: {audio_info}")
                
                # Update after download
                job.title = audio_info.get("title")
                job.duration = audio_info.get("duration")
                job.progress = 100  # Download complete
                await self.db_session.commit()
                logger.info(f"Download phase complete for job {job_id}")
            except Exception as e:
                await self._mark_job_failed(job_id, f"Download failed: {str(e)}")
                return

            # Get transcription backend
            backend = await self.backend_factory.get_best_backend()
            if not backend:
                await self._mark_job_failed(job_id, "No transcription backend available")
                return

            # Transcribe audio
            try:
                # Switch to transcription phase
                job.phase = JobPhase.TRANSCRIBING
                job.progress = 0
                await self.db_session.commit()
                logger.info(f"Starting transcription for job {job_id} with backend {backend.__class__.__name__}")
                
                # Simply transcribe without progress tracking
                result = await backend.transcribe(
                    audio_info["audio_path"],
                    model=job.model_requested,
                    language=job.language,
                )
                logger.info(f"Transcription complete for job {job_id}")
                
                # Update job with results
                job.text = result.text
                job.model_used = result.model_used
                job.detected_language = result.language
                job.word_count = len(result.text.split())
                job.progress = 100  # Transcription complete
                await self.db_session.commit()
                logger.info(f"Transcription phase complete for job {job_id}")

            except Exception as e:
                await self._mark_job_failed(job_id, f"Transcription failed: {str(e)}")
                return

            # Cache result if enabled
            if settings.cache_enabled:
                await self.cache_service.store_transcription(
                    job.url,
                    job.model_requested,
                    {
                        "text": job.text,
                        "title": job.title,
                        "duration": job.duration,
                        "model_used": job.model_used,
                        "language": job.detected_language,
                    },
                )

            # Finalize
            job.phase = JobPhase.FINALIZING
            job.progress = 0
            await self.db_session.commit()
            
            # Mark completed
            job.status = JobStatus.COMPLETED
            job.phase = JobPhase.COMPLETE
            job.progress = 100
            job.completed_at = datetime.utcnow()
            job.processing_time_ms = int(
                (job.completed_at - start_time).total_seconds() * 1000
            )
            logger.info(f"Marking job {job_id} as completed with {job.word_count} words")
            await self.db_session.commit()
            logger.info(f"Job {job_id} successfully marked as completed")

        except Exception as e:
            await self._mark_job_failed(job_id, f"Unexpected error: {str(e)}")
        finally:
            # Clean up
            if job_id in self._running_jobs:
                del self._running_jobs[job_id]
            
            # Clean up audio file
            if 'audio_info' in locals() and audio_info.get("audio_path"):
                await self.download_service.cleanup_file(audio_info["audio_path"])


    async def _mark_job_failed(self, job_id: UUID, error: str) -> None:
        """Mark job as failed with error message."""
        job = await self.get_job(job_id)
        if job:
            job.status = JobStatus.FAILED
            job.error = error
            job.completed_at = datetime.utcnow()
            if job.started_at:
                job.processing_time_ms = int(
                    (job.completed_at - job.started_at).total_seconds() * 1000
                )
            await self.db_session.commit()