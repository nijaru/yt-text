"""API routes for transcription service."""

from uuid import UUID

from litestar import Controller, Request, WebSocket, get, post, websocket
from litestar.di import Provide
from litestar.exceptions import HTTPException, NotFoundException

from src.core.models import (
    ErrorResponse,
    JobResponse,
    JobStatusResponse,
    TranscribeRequest,
    TranscriptionResult,
)
from src.services.transcription import TranscriptionService


class TranscriptionController(Controller):
    """Controller for transcription API endpoints."""

    path = "/api"

    @post("/transcribe")
    async def create_transcription_job(
        self,
        request: Request,
        data: TranscribeRequest,
        service: TranscriptionService = Provide(),
    ) -> JobResponse:
        """Submit a URL for transcription."""
        try:
            job = await service.create_job(
                url=data.url,
                model=data.model,
                language=data.language,
                ip_address=request.client.host if request.client else None,
                user_agent=request.headers.get("user-agent"),
            )
            return JobResponse(
                job_id=job.id,
                status=job.status,
                created_at=job.created_at,
            )
        except ValueError as e:
            raise HTTPException(status_code=400, detail=str(e))
        except Exception as e:
            raise HTTPException(status_code=500, detail="Internal server error")

    @get("/jobs/{job_id:uuid}")
    async def get_job_status(
        self,
        job_id: UUID,
        service: TranscriptionService = Provide(),
    ) -> JobStatusResponse:
        """Get job status and progress."""
        job = await service.get_job(job_id)
        if not job:
            raise NotFoundException(detail=f"Job {job_id} not found")

        return JobStatusResponse(
            job_id=job.id,
            status=job.status,
            progress=job.progress,
            created_at=job.created_at,
            started_at=job.started_at,
            completed_at=job.completed_at,
            processing_time_ms=job.processing_time_ms,
            error=job.error,
        )

    @get("/jobs/{job_id:uuid}/result")
    async def get_transcription_result(
        self,
        job_id: UUID,
        service: TranscriptionService = Provide(),
    ) -> TranscriptionResult:
        """Get completed transcription result."""
        job = await service.get_job(job_id)
        if not job:
            raise NotFoundException(detail=f"Job {job_id} not found")

        if job.status != "completed":
            raise HTTPException(
                status_code=409,
                detail=f"Job is {job.status}, not completed"
            )

        if not job.text:
            raise HTTPException(
                status_code=500,
                detail="Job completed but no transcription text available"
            )

        return TranscriptionResult(
            job_id=job.id,
            url=job.url,
            title=job.title,
            duration=job.duration,
            text=job.text,
            model_used=job.model_used or job.model_requested,
            word_count=job.word_count or len(job.text.split()),
            language=job.detected_language or "unknown",
            created_at=job.created_at,
            completed_at=job.completed_at or job.created_at,
            processing_time_ms=job.processing_time_ms or 0,
        )

    @post("/jobs/{job_id:uuid}/retry")
    async def retry_job(
        self,
        job_id: UUID,
        service: TranscriptionService = Provide(),
    ) -> JobResponse:
        """Retry a failed job."""
        job = await service.retry_job(job_id)
        if not job:
            raise NotFoundException(detail=f"Job {job_id} not found")

        return JobResponse(
            job_id=job.id,
            status=job.status,
            created_at=job.created_at,
        )


class HealthController(Controller):
    """Health check endpoints."""

    path = "/health"

    @get("/")
    async def health_check(self) -> dict[str, str]:
        """Basic health check."""
        return {"status": "healthy", "service": "yt-text"}

    @get("/ready")
    async def readiness_check(
        self,
        service: TranscriptionService = Provide(),
    ) -> dict[str, str | bool]:
        """Readiness check including dependencies."""
        try:
            # Check if transcription backends are available
            backends_available = await service.check_backends()
            
            return {
                "status": "ready" if backends_available else "not_ready",
                "backends_available": backends_available,
            }
        except Exception:
            return {
                "status": "not_ready",
                "backends_available": False,
            }


# WebSocket endpoint for real-time job updates
@websocket("/ws/jobs/{job_id:uuid}")
async def job_updates_websocket(
    socket: WebSocket,
    job_id: UUID,
    service: TranscriptionService = Provide(),
) -> None:
    """WebSocket endpoint for real-time job updates."""
    await socket.accept()
    
    try:
        async for update in service.stream_job_updates(job_id):
            await socket.send_json({
                "type": "status_update",
                "job_id": str(job_id),
                "status": update.status,
                "progress": update.progress,
            })
            
            if update.status in ["completed", "failed"]:
                if update.status == "completed" and update.text:
                    await socket.send_json({
                        "type": "result",
                        "job_id": str(job_id),
                        "text": update.text,
                    })
                elif update.status == "failed" and update.error:
                    await socket.send_json({
                        "type": "error",
                        "job_id": str(job_id),
                        "error": update.error,
                    })
                break
                
    except Exception as e:
        await socket.send_json({
            "type": "error",
            "message": str(e),
        })
    finally:
        await socket.close()


# Export router for main app
transcription_router = [
    TranscriptionController,
    HealthController,
    job_updates_websocket,
]