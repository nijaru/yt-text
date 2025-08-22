"""Database models using SQLModel."""

from datetime import datetime
from enum import StrEnum
from typing import Optional
from uuid import UUID, uuid4

from sqlmodel import Field, SQLModel


class JobStatus(StrEnum):
    """Job status enumeration."""

    PENDING = "pending"
    PROCESSING = "processing"
    COMPLETED = "completed"
    FAILED = "failed"


class TranscriptionJob(SQLModel, table=True):
    """Transcription job model."""

    __tablename__ = "transcription_jobs"

    id: UUID = Field(default_factory=uuid4, primary_key=True)
    url: str = Field(index=True)
    status: JobStatus = Field(default=JobStatus.PENDING, index=True)
    
    # Request parameters
    model_requested: str = Field(default="base")
    language: Optional[str] = Field(default=None)
    
    # Results
    text: Optional[str] = Field(default=None)
    title: Optional[str] = Field(default=None)
    duration: Optional[int] = Field(default=None)  # seconds
    word_count: Optional[int] = Field(default=None)
    model_used: Optional[str] = Field(default=None)
    detected_language: Optional[str] = Field(default=None)
    
    # Metadata
    error: Optional[str] = Field(default=None)
    progress: int = Field(default=0)  # percentage
    processing_time_ms: Optional[int] = Field(default=None)
    ip_address: Optional[str] = Field(default=None)
    user_agent: Optional[str] = Field(default=None)
    
    # Timestamps
    created_at: datetime = Field(default_factory=datetime.utcnow, index=True)
    started_at: Optional[datetime] = Field(default=None)
    completed_at: Optional[datetime] = Field(default=None)

    class Config:
        """Model configuration."""
        
        json_encoders = {
            datetime: lambda v: v.isoformat(),
            UUID: lambda v: str(v),
        }


class TranscribeRequest(SQLModel):
    """Request model for transcription."""

    url: str = Field(min_length=1, max_length=2048)
    model: str = Field(default="base")
    language: Optional[str] = Field(default=None, max_length=10)


class JobResponse(SQLModel):
    """Response model for job creation."""

    job_id: UUID
    status: JobStatus
    created_at: datetime


class JobStatusResponse(SQLModel):
    """Response model for job status check."""

    job_id: UUID
    status: JobStatus
    progress: int
    created_at: datetime
    started_at: Optional[datetime] = None
    completed_at: Optional[datetime] = None
    processing_time_ms: Optional[int] = None
    error: Optional[str] = None


class TranscriptionResult(SQLModel):
    """Response model for transcription result."""

    job_id: UUID
    url: str
    title: Optional[str]
    duration: Optional[int]
    text: str
    model_used: str
    word_count: int
    language: str
    created_at: datetime
    completed_at: datetime
    processing_time_ms: int


class ErrorResponse(SQLModel):
    """Standard error response."""

    error: dict[str, str | dict]

    @classmethod
    def create(cls, code: str, message: str, details: Optional[dict] = None) -> "ErrorResponse":
        """Create an error response."""
        error = {"code": code, "message": message}
        if details:
            error["details"] = details
        return cls(error=error)