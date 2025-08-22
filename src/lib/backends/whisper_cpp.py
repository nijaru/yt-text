"""Whisper.cpp backend for efficient transcription."""

import asyncio
import subprocess
from pathlib import Path
from typing import Callable, Optional

from src.core.config import settings
from src.lib.backends.base import TranscriptionBackend, TranscriptionResult


class WhisperCPPBackend(TranscriptionBackend):
    """Backend using whisper.cpp for efficient transcription."""

    def __init__(self):
        self.whisper_cpp_path = self._find_whisper_cpp()
        self.models_dir = settings.model_dir

    async def is_available(self) -> bool:
        """Check if whisper.cpp is available."""
        if not self.whisper_cpp_path:
            return False
            
        try:
            # Test if whisper.cpp works
            result = await asyncio.create_subprocess_exec(
                self.whisper_cpp_path, "--help",
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE
            )
            await result.communicate()
            return result.returncode == 0
        except Exception:
            return False

    async def transcribe(
        self,
        audio_path: Path,
        model: str = "base",
        language: Optional[str] = None,
        progress_callback: Optional[Callable[[int], None]] = None,
    ) -> TranscriptionResult:
        """Transcribe audio using whisper.cpp."""
        
        if progress_callback:
            progress_callback(10)

        # Get model path
        model_path = await self._get_model_path(model)
        if not model_path:
            raise RuntimeError(f"Model {model} not available")

        if progress_callback:
            progress_callback(20)

        # Build command
        cmd = [
            self.whisper_cpp_path,
            "-m", str(model_path),
            "-f", str(audio_path),
            "--output-txt",  # Output as text
            "--output-file", str(audio_path.with_suffix("")),  # Output filename
        ]
        
        # Add language if specified
        if language and language != "auto":
            cmd.extend(["-l", language])

        if progress_callback:
            progress_callback(30)

        # Run whisper.cpp
        try:
            process = await asyncio.create_subprocess_exec(
                *cmd,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE
            )
            
            stdout, stderr = await process.communicate()
            
            if process.returncode != 0:
                error_msg = stderr.decode() if stderr else "Unknown error"
                raise RuntimeError(f"whisper.cpp failed: {error_msg}")

            if progress_callback:
                progress_callback(80)

            # Read output file
            output_file = audio_path.with_suffix(".txt")
            if not output_file.exists():
                raise RuntimeError("whisper.cpp did not produce output file")

            text = output_file.read_text(encoding="utf-8").strip()
            
            # Clean up output file
            try:
                output_file.unlink()
            except Exception:
                pass

            if progress_callback:
                progress_callback(90)

            # Extract language from stderr if available
            detected_language = self._extract_language(stderr.decode() if stderr else "")

            return TranscriptionResult(
                text=text,
                language=detected_language or language or "unknown",
                model_used=f"whisper.cpp-{model}",
            )

        except Exception as e:
            raise RuntimeError(f"Transcription failed: {str(e)}")

    async def _get_model_path(self, model: str) -> Optional[Path]:
        """Get path to whisper.cpp model file."""
        model_filename = f"ggml-{model}.bin"
        model_path = self.models_dir / model_filename
        
        if model_path.exists():
            return model_path
            
        # Try to download model (simplified)
        # In production, models should be pre-downloaded
        return None

    def _find_whisper_cpp(self) -> Optional[str]:
        """Find whisper.cpp executable."""
        # Check common locations
        candidates = [
            "/usr/local/bin/whisper.cpp",
            "/usr/bin/whisper.cpp",
            "whisper.cpp",  # In PATH
        ]
        
        for candidate in candidates:
            try:
                result = subprocess.run(
                    [candidate, "--help"], 
                    capture_output=True, 
                    timeout=5
                )
                if result.returncode == 0:
                    return candidate
            except Exception:
                continue
                
        return None

    def _extract_language(self, stderr: str) -> Optional[str]:
        """Extract detected language from whisper.cpp output."""
        # whisper.cpp usually outputs detected language in stderr
        for line in stderr.split('\n'):
            if 'detected language:' in line.lower():
                parts = line.split(':')
                if len(parts) > 1:
                    return parts[1].strip()
        return None

    def get_priority(self) -> int:
        """High priority - efficient backend."""
        return 10

    def get_name(self) -> str:
        """Backend name."""
        return "whisper_cpp"

    def supports_model(self, model: str) -> bool:
        """Check if model is supported."""
        return model in ["tiny", "base", "small", "medium", "large"]

    def get_supported_models(self) -> list[str]:
        """Get supported models."""
        return ["tiny", "base", "small", "medium", "large"]