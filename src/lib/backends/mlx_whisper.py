"""MLX Whisper backend for Apple Silicon."""

import asyncio
from pathlib import Path
from typing import Callable, Optional

from src.lib.backends.base import TranscriptionBackend, TranscriptionResult


class MLXWhisperBackend(TranscriptionBackend):
    """Backend using MLX Whisper for Apple Silicon."""

    def __init__(self):
        self._model_cache = {}
        # Map simple model names to MLX community model repositories
        self._model_mapping = {
            "tiny": "mlx-community/whisper-tiny-mlx",
            "base": "mlx-community/whisper-base-mlx", 
            "small": "mlx-community/whisper-small-mlx",
            "medium": "mlx-community/whisper-medium-mlx",
            "large": "mlx-community/whisper-large-v3-mlx",
            "large-v1": "mlx-community/whisper-large-v1-mlx",
            "large-v2": "mlx-community/whisper-large-v2-mlx",
            "large-v3": "mlx-community/whisper-large-v3-mlx",
        }

    async def is_available(self) -> bool:
        """Check if MLX Whisper is available."""
        try:
            import platform
            # Only available on Apple Silicon
            if not (platform.machine() == "arm64" and platform.system() == "Darwin"):
                return False
                
            import mlx_whisper
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
        """Transcribe audio using MLX Whisper."""
        
        try:
            import mlx_whisper
        except ImportError:
            raise RuntimeError("MLX Whisper not available")

        if progress_callback:
            progress_callback(10)

        # Run in thread pool since MLX operations can be blocking
        loop = asyncio.get_event_loop()
        result = await loop.run_in_executor(
            None,
            self._transcribe_sync,
            str(audio_path),
            model,
            language,
            progress_callback
        )

        return TranscriptionResult(
            text=result["text"].strip(),
            language=result.get("language", "unknown"),
            model_used=f"mlx-whisper-{model}",
            segments=result.get("segments"),
        )

    def _transcribe_sync(
        self,
        audio_path: str,
        model: str,
        language: Optional[str],
        progress_callback: Optional[Callable[[int], None]]
    ) -> dict:
        """Run transcription synchronously."""
        import mlx_whisper

        if progress_callback:
            progress_callback(30)

        # Map model name to MLX community repository
        mlx_model = self._model_mapping.get(model, f"mlx-community/whisper-{model}")

        # Transcribe with MLX
        kwargs = {}
        if language and language != "auto":
            kwargs["language"] = language

        result = mlx_whisper.transcribe(
            audio_path,
            path_or_hf_repo=mlx_model,
            **kwargs
        )

        if progress_callback:
            progress_callback(90)

        return result

    def get_priority(self) -> int:
        """High priority for development on Apple Silicon."""
        return 5

    def get_name(self) -> str:
        """Backend name."""
        return "mlx_whisper"

    def supports_model(self, model: str) -> bool:
        """Check if model is supported."""
        return model in self._model_mapping

    def get_supported_models(self) -> list[str]:
        """Get supported models."""
        return list(self._model_mapping.keys())