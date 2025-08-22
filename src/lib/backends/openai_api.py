"""OpenAI API backend for transcription."""

from pathlib import Path
from typing import Callable, Optional

import httpx

from src.core.config import settings
from src.lib.backends.base import TranscriptionBackend, TranscriptionResult


class OpenAIBackend(TranscriptionBackend):
    """Backend using OpenAI Whisper API."""

    def __init__(self):
        self.api_key = settings.openai_api_key
        self.model = settings.openai_model
        self.daily_limit = settings.openai_daily_limit
        self._usage_count = 0

    async def is_available(self) -> bool:
        """Check if OpenAI API is available."""
        if not self.api_key:
            return False
            
        if self._usage_count >= self.daily_limit:
            return False
            
        # Quick API check (optional)
        return True

    async def transcribe(
        self,
        audio_path: Path,
        model: str = "base",
        language: Optional[str] = None,
        progress_callback: Optional[Callable[[int], None]] = None,
    ) -> TranscriptionResult:
        """Transcribe audio using OpenAI API."""
        
        if not self.api_key:
            raise RuntimeError("OpenAI API key not configured")
            
        if self._usage_count >= self.daily_limit:
            raise RuntimeError("Daily API limit exceeded")

        if progress_callback:
            progress_callback(10)

        # Prepare file for upload
        if audio_path.stat().st_size > 25 * 1024 * 1024:  # 25MB limit
            raise RuntimeError("Audio file too large for OpenAI API (25MB limit)")

        if progress_callback:
            progress_callback(20)

        # Make API request
        async with httpx.AsyncClient(timeout=60.0) as client:
            with open(audio_path, "rb") as audio_file:
                files = {
                    "file": (audio_path.name, audio_file, "audio/wav"),
                    "model": (None, self.model),
                }
                
                if language and language != "auto":
                    files["language"] = (None, language)

                if progress_callback:
                    progress_callback(30)

                response = await client.post(
                    "https://api.openai.com/v1/audio/transcriptions",
                    headers={
                        "Authorization": f"Bearer {self.api_key}",
                    },
                    files=files,
                )

                if progress_callback:
                    progress_callback(80)

                if response.status_code != 200:
                    raise RuntimeError(f"OpenAI API error: {response.status_code} {response.text}")

                result = response.json()
                self._usage_count += 1

                if progress_callback:
                    progress_callback(90)

                return TranscriptionResult(
                    text=result["text"].strip(),
                    language=result.get("language", language or "unknown"),
                    model_used=f"openai-{self.model}",
                )

    def get_priority(self) -> int:
        """Medium priority - API fallback."""
        return 50

    def get_name(self) -> str:
        """Backend name."""
        return "openai_api"

    def supports_model(self, model: str) -> bool:
        """OpenAI API uses its own model."""
        return True

    def get_supported_models(self) -> list[str]:
        """OpenAI API model."""
        return [self.model]

    def get_usage_info(self) -> dict:
        """Get API usage information."""
        return {
            "usage_count": self._usage_count,
            "daily_limit": self.daily_limit,
            "remaining": max(0, self.daily_limit - self._usage_count),
        }