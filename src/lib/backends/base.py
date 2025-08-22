"""Base classes for transcription backends."""

from abc import ABC, abstractmethod
from dataclasses import dataclass
from pathlib import Path
from typing import Callable, Optional


@dataclass
class TranscriptionResult:
    """Result from transcription operation."""
    
    text: str
    language: str
    model_used: str
    confidence: Optional[float] = None
    segments: Optional[list] = None
    processing_time_ms: Optional[int] = None


class TranscriptionBackend(ABC):
    """Abstract base class for transcription backends."""

    @abstractmethod
    async def is_available(self) -> bool:
        """Check if this backend is available and ready to use."""
        pass

    @abstractmethod
    async def transcribe(
        self,
        audio_path: Path,
        model: str = "base",
        language: Optional[str] = None,
        progress_callback: Optional[Callable[[int], None]] = None,
    ) -> TranscriptionResult:
        """Transcribe audio file to text."""
        pass

    @abstractmethod
    def get_priority(self) -> int:
        """Get backend priority (lower = higher priority)."""
        pass

    @abstractmethod
    def get_name(self) -> str:
        """Get backend name for identification."""
        pass

    async def cleanup(self) -> None:
        """Clean up resources (optional)."""
        pass

    def supports_model(self, model: str) -> bool:
        """Check if backend supports the specified model."""
        return True  # Default: assume all models supported

    def supports_language(self, language: str) -> bool:
        """Check if backend supports the specified language."""
        return True  # Default: assume all languages supported

    async def estimate_processing_time(
        self, audio_path: Path, model: str = "base"
    ) -> Optional[int]:
        """Estimate processing time in seconds (optional)."""
        return None

    def get_supported_models(self) -> list[str]:
        """Get list of supported model names."""
        return ["tiny", "base", "small", "medium", "large"]

    def get_supported_languages(self) -> list[str]:
        """Get list of supported language codes."""
        return ["auto"]  # Auto-detect by default