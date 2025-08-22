"""Transcription service orchestrating download and transcription."""

import asyncio
import logging
from datetime import datetime
from typing import AsyncIterator, Optional
from uuid import UUID

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from src.core.config import settings
from src.core.models import JobStatus, TranscriptionJob
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
            task = asyncio.create_task(self._process_job(job.id))
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
            task = asyncio.create_task(self._process_job(job.id))
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
        while True:
            job = await self.get_job(job_id)
            if not job:
                break
                
            yield job
            
            if job.status in [JobStatus.COMPLETED, JobStatus.FAILED]:
                break
                
            await asyncio.sleep(1)

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
            job.progress = 5
            await self.db_session.commit()

            # Download audio
            try:
                download_callback = self._create_progress_callback(job_id, 5, 0.3)
                audio_info = await self.download_service.download_audio(
                    job.url, progress_callback=download_callback
                )
                job.title = audio_info.get("title")
                job.duration = audio_info.get("duration")
                job.progress = 35
                await self.db_session.commit()
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
                transcribe_callback = self._create_progress_callback(job_id, 35, 0.6)
                result = await backend.transcribe(
                    audio_info["audio_path"],
                    model=job.model_requested,
                    language=job.language,
                    progress_callback=transcribe_callback,
                )
                
                # Update job with results
                job.text = result.text
                job.model_used = result.model_used
                job.detected_language = result.language
                job.word_count = len(result.text.split())
                job.progress = 95
                await self.db_session.commit()

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

            # Mark completed
            job.status = JobStatus.COMPLETED
            job.progress = 100
            job.completed_at = datetime.utcnow()
            job.processing_time_ms = int(
                (job.completed_at - start_time).total_seconds() * 1000
            )
            await self.db_session.commit()

        except Exception as e:
            await self._mark_job_failed(job_id, f"Unexpected error: {str(e)}")
        finally:
            # Clean up
            if job_id in self._running_jobs:
                del self._running_jobs[job_id]
            
            # Clean up audio file
            if 'audio_info' in locals() and audio_info.get("audio_path"):
                await self.download_service.cleanup_file(audio_info["audio_path"])

    def _create_progress_callback(self, job_id: UUID, base_progress: int, scale: float) -> callable:
        """Create a sync progress callback that schedules async updates."""
        def sync_callback(progress: int) -> None:
            final_progress = base_progress + int(progress * scale)
            # Try to schedule the async update, but don't fail if no event loop
            try:
                loop = asyncio.get_running_loop()
                loop.create_task(self._update_progress_async(job_id, final_progress))
            except RuntimeError:
                # No event loop running, skip progress update
                logging.debug(f"Skipping progress update for job {job_id}: no event loop")
        return sync_callback

    async def _update_progress_async(self, job_id: UUID, progress: int) -> None:
        """Update job progress asynchronously."""
        try:
            job = await self.get_job(job_id)
            if job:
                job.progress = min(progress, 100)
                await self.db_session.commit()
        except Exception as e:
            logging.warning(f"Failed to update progress for job {job_id}: {e}")

    async def _update_progress(self, job_id: UUID, progress: int) -> None:
        """Update job progress."""
        job = await self.get_job(job_id)
        if job:
            job.progress = min(progress, 100)
            await self.db_session.commit()

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