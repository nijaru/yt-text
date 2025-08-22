"""Fallback whisper backend using openai-whisper."""

import asyncio
from pathlib import Path
from typing import Callable, Optional

import whisper

from src.lib.backends.base import TranscriptionBackend, TranscriptionResult


class FallbackWhisperBackend(TranscriptionBackend):
    """Fallback backend using openai-whisper library."""

    def __init__(self):
        self._model_cache = {}

    async def is_available(self) -> bool:
        """Check if whisper is available."""
        try:
            import whisper
            return True
        except ImportError:
            return False

    async def transcribe(
        self,
        audio_path: Path,
        model: str = "base",
        language: Optional[str] = None,
        progress_callback: Optional[Callable[[int], None]] = None,
    ) -> TranscriptionResult:
        """Transcribe audio using openai-whisper."""
        
        # Load model
        if progress_callback:
            progress_callback(10)
            
        whisper_model = await self._load_model(model)
        
        if progress_callback:
            progress_callback(20)

        # Run transcription in thread pool
        loop = asyncio.get_event_loop()
        result = await loop.run_in_executor(
            None, 
            self._transcribe_sync, 
            whisper_model, 
            str(audio_path), 
            language,
            progress_callback
        )
        
        return TranscriptionResult(
            text=result["text"].strip(),
            language=result["language"],
            model_used=f"whisper-{model}",
            segments=result.get("segments"),
        )

    def _transcribe_sync(
        self, 
        model, 
        audio_path: str, 
        language: Optional[str],
        progress_callback: Optional[Callable[[int], None]]
    ) -> dict:
        """Run transcription synchronously."""
        
        # Create progress callback wrapper
        def progress_wrapper(progress: float):
            if progress_callback:
                # Map whisper progress (0-1) to our range (20-90)
                our_progress = 20 + int(progress * 70)
                progress_callback(our_progress)
        
        # Transcribe with optional language
        kwargs = {}
        if language and language != "auto":
            kwargs["language"] = language
            
        result = model.transcribe(
            audio_path, 
            verbose=False,
            **kwargs
        )
        
        if progress_callback:
            progress_callback(90)
            
        return result

    async def _load_model(self, model_name: str):
        """Load whisper model with caching."""
        if model_name in self._model_cache:
            return self._model_cache[model_name]
            
        loop = asyncio.get_event_loop()
        whisper_model = await loop.run_in_executor(
            None, whisper.load_model, model_name
        )
        
        self._model_cache[model_name] = whisper_model
        return whisper_model

    def get_priority(self) -> int:
        """Low priority fallback backend."""
        return 100

    def get_name(self) -> str:
        """Backend name."""
        return "fallback_whisper"

    def supports_model(self, model: str) -> bool:
        """Check if model is supported."""
        return model in ["tiny", "base", "small", "medium", "large"]

    def get_supported_models(self) -> list[str]:
        """Get supported models."""
        return ["tiny", "base", "small", "medium", "large"]

    async def cleanup(self) -> None:
        """Clean up cached models."""
        self._model_cache.clear()