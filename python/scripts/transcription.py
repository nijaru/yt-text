import os
import tempfile
import time
from typing import Dict, Optional

import torch
import yt_dlp
from faster_whisper import WhisperModel


class TranscriptionError(Exception):
    """Base exception for transcription errors"""

    pass


class NullLogger:
    """A logger class that does nothing. Used to suppress yt_dlp output."""

    def debug(self, msg):
        pass

    def warning(self, msg):
        pass

    def error(self, msg):
        pass


class Transcriber:
    def __init__(
        self,
        model_name: str = "base.en",
        device: Optional[str] = None,
        compute_type: Optional[str] = None,
        max_video_duration: Optional[int] = None,
        max_file_size: Optional[int] = None,
    ):
        self.model_name = model_name
        self.device = device or ("cuda" if torch.cuda.is_available() else "cpu")
        self.compute_type = compute_type or (
            "float16" if self.device == "cuda" else "float32"
        )
        self.max_video_duration = max_video_duration
        self.max_file_size = max_file_size

        self.model = WhisperModel(
            self.model_name,
            device=self.device,
            compute_type=self.compute_type,
            download_root="/tmp/models",
        )

    def process_url(self, url: str) -> Dict:
        """Process a single URL and return transcription result."""
        try:
            with tempfile.TemporaryDirectory() as temp_dir:
                # Download audio and retrieve media title
                audio_path, media_title = self._download_audio(url, temp_dir)

                # Transcribe
                transcription = self._transcribe(audio_path)

                # Include title and URL in the result
                transcription["title"] = media_title
                transcription["url"] = url

                return transcription

        except TranscriptionError as te:
            return {
                "error": str(te),
                "text": None,
                "model_name": self.model_name,
                "duration": 0,
                "title": None,
                "url": url,
            }
        except Exception as e:
            return {
                "error": f"Unexpected error: {e}",
                "text": None,
                "model_name": self.model_name,
                "duration": 0,
                "title": None,
                "url": url,
            }

    def _download_audio(self, url: str, temp_dir: str) -> tuple[str, str]:
        """Download audio from URL and retrieve media title."""
        ydl_opts = {
            "format": "bestaudio/best",
            "outtmpl": os.path.join(temp_dir, "%(id)s.%(ext)s"),
            "quiet": True,
            "no_warnings": True,
            "extractaudio": True,
            "logger": NullLogger(),  # Suppress yt_dlp logs
        }

        try:
            with yt_dlp.YoutubeDL(ydl_opts) as ydl:
                # Extract info without downloading
                info = ydl.extract_info(url, download=False)
                if not isinstance(info, dict):
                    raise TranscriptionError("Failed to extract video information.")

                # Retrieve media title
                media_title = info.get("title") or url

                # Validate duration if constraint is set
                duration = info.get("duration", 0)
                if self.max_video_duration and duration > self.max_video_duration:
                    raise TranscriptionError(
                        f"Media duration ({duration}s) exceeds maximum allowed ({self.max_video_duration}s)"
                    )

                # Download and process audio
                info = ydl.extract_info(url, download=True)
                if not isinstance(info, dict):
                    raise TranscriptionError("Failed to download audio.")

                downloaded_file = ydl.prepare_filename(info)

                # Handle file size constraints
                file_size = os.path.getsize(downloaded_file)
                if self.max_file_size and file_size > self.max_file_size:
                    raise TranscriptionError(
                        f"Downloaded file size ({file_size} bytes) exceeds maximum allowed ({self.max_file_size} bytes)"
                    )

                return downloaded_file, media_title

        except TranscriptionError:
            raise
        except Exception as e:
            raise TranscriptionError(f"Failed to download audio: {e}")

    def _transcribe(self, audio_path: str) -> Dict:
        """Transcribe audio file."""
        try:
            start_time = time.time()
            segments, info = self.model.transcribe(
                audio_path,
                beam_size=5,
                temperature=0.2,
                best_of=1,
                condition_on_previous_text=True,
                vad_filter=True,
                vad_parameters=dict(min_silence_duration_ms=500),
                language="en",
            )

            segments = list(segments)
            if not segments:
                raise TranscriptionError("No speech detected")

            # Combine segments into single text
            text = " ".join(seg.text.strip() for seg in segments if seg.text.strip())

            return {
                "text": text,
                "model_name": self.model_name,
                "duration": time.time() - start_time,
                "error": None,
            }

        except Exception as e:
            raise TranscriptionError(f"Transcription failed: {e}")

    def close(self):
        """Clean up resources if necessary."""
        del self.model
        if torch.cuda.is_available():
            torch.cuda.empty_cache()
        import gc

        gc.collect()
