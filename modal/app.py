"""
yt-text Modal Worker

GPU-accelerated transcription using NVIDIA Parakeet.
Designed to be triggered by Cloudflare Queue or direct invocation.
"""

import os
import subprocess
import tempfile
from pathlib import Path

import modal

# Modal app configuration
app = modal.App("yt-text")

# Container image with all dependencies
image = (
    modal.Image.debian_slim(python_version="3.12")
    .apt_install("ffmpeg", "curl")
    .pip_install(
        "yt-dlp>=2024.11.18",
        "httpx>=0.27.0",
        "nemo_toolkit[asr]>=2.0.0",
    )
)

# Persistent volume for model cache
model_volume = modal.Volume.from_name("parakeet-models", create_if_missing=True)


@app.cls(
    image=image,
    gpu="L4",
    timeout=1800,  # 30 minutes max
    volumes={"/models": model_volume},
    secrets=[modal.Secret.from_name("yt-text-secrets", required=False)],
)
class Transcriber:
    """Transcription service with cached model loading."""

    @modal.enter()
    def load_models(self):
        """Load models once per container startup."""
        import nemo.collections.asr as nemo_asr

        os.environ["NEMO_CACHE_DIR"] = "/models"

        # Pre-load both models
        self.model_en = nemo_asr.models.ASRModel.from_pretrained("nvidia/parakeet-tdt-0.6b-v2")
        self.model_multi = nemo_asr.models.ASRModel.from_pretrained("nvidia/parakeet-tdt-0.6b-v3")

    @modal.method()
    def transcribe(
        self,
        url: str,
        job_id: str | None = None,
        callback_url: str | None = None,
        callback_secret: str | None = None,
        language: str = "en",
    ) -> dict:
        """
        Download and transcribe audio from a URL.

        Args:
            url: URL to download audio from (YouTube, etc.)
            job_id: Optional job ID for tracking
            callback_url: Optional webhook URL for completion notification
            callback_secret: Secret for authenticating callbacks
            language: Language code (default: en)

        Returns:
            dict with text, duration, word_count, and metadata
        """

        # Validate URL format
        if not url.startswith(("http://", "https://")):
            raise ValueError("URL must start with http:// or https://")

        with tempfile.TemporaryDirectory() as tmpdir:
            audio_path = Path(tmpdir) / "audio.wav"

            # Send downloading status
            if callback_url:
                self._send_callback(
                    callback_url,
                    callback_secret,
                    {"status": "downloading", "progress": 10},
                )

            # Download audio with yt-dlp
            try:
                result = subprocess.run(
                    [
                        "yt-dlp",
                        "-x",
                        "--audio-format",
                        "wav",
                        "--audio-quality",
                        "0",
                        "--postprocessor-args",
                        "ffmpeg:-ar 16000 -ac 1",  # 16kHz mono for Parakeet
                        "-o",
                        str(audio_path.with_suffix(".%(ext)s")),
                        url,
                    ],
                    capture_output=True,
                    text=True,
                    timeout=300,  # 5 min download timeout
                )
                if result.returncode != 0:
                    raise RuntimeError(f"yt-dlp failed: {result.stderr}")
            except subprocess.TimeoutExpired as e:
                raise RuntimeError("Download timed out") from e

            # Find the actual output file (yt-dlp may change extension)
            wav_files = list(Path(tmpdir).glob("*.wav"))
            if not wav_files:
                raise RuntimeError("No audio file produced")
            audio_path = wav_files[0]

            # Send transcribing status
            if callback_url:
                self._send_callback(
                    callback_url,
                    callback_secret,
                    {"status": "transcribing", "progress": 50},
                )

            # Select model based on language
            model = self.model_en if language == "en" else self.model_multi
            model_name = (
                "nvidia/parakeet-tdt-0.6b-v2" if language == "en" else "nvidia/parakeet-tdt-0.6b-v3"
            )

            # Transcribe
            transcription = model.transcribe([str(audio_path)])
            if not transcription:
                raise RuntimeError("Transcription returned empty result")

            text = (
                transcription[0].text
                if hasattr(transcription[0], "text")
                else str(transcription[0])
            )

            # Get audio duration
            import wave

            with wave.open(str(audio_path), "r") as wav:
                frames = wav.getnframes()
                rate = wav.getframerate()
                duration = round(frames / float(rate), 1)

            result = {
                "job_id": job_id,
                "status": "complete",
                "text": text,
                "duration": duration,
                "word_count": len(text.split()),
                "model": model_name,
                "language": language,
            }

            # Send completion callback
            if callback_url:
                self._send_callback(callback_url, callback_secret, result)

            return result

    @modal.method()
    def transcribe_file(
        self,
        audio_bytes: bytes,
        filename: str = "audio.wav",
        language: str = "en",
    ) -> dict:
        """
        Transcribe audio from raw bytes (for direct file uploads).

        Args:
            audio_bytes: Raw audio file bytes
            filename: Original filename for format detection
            language: Language code (default: en)

        Returns:
            dict with text, duration, word_count, and metadata
        """
        import wave

        os.environ["NEMO_CACHE_DIR"] = "/models"

        with tempfile.TemporaryDirectory() as tmpdir:
            # Sanitize filename to prevent path traversal
            safe_filename = Path(filename).name
            audio_path = Path(tmpdir) / safe_filename
            audio_path.write_bytes(audio_bytes)

            # Convert to WAV if needed
            if not safe_filename.endswith(".wav"):
                wav_path = Path(tmpdir) / "audio.wav"
                try:
                    result = subprocess.run(
                        [
                            "ffmpeg",
                            "-i",
                            str(audio_path),
                            "-ar",
                            "16000",
                            "-ac",
                            "1",
                            str(wav_path),
                        ],
                        capture_output=True,
                        text=True,
                        timeout=120,
                    )
                    if result.returncode != 0:
                        raise RuntimeError(f"ffmpeg conversion failed: {result.stderr}")
                except subprocess.TimeoutExpired as e:
                    raise RuntimeError("Audio conversion timed out") from e
                audio_path = wav_path

            # Select model
            model = self.model_en if language == "en" else self.model_multi
            model_name = (
                "nvidia/parakeet-tdt-0.6b-v2" if language == "en" else "nvidia/parakeet-tdt-0.6b-v3"
            )

            # Transcribe
            transcription = model.transcribe([str(audio_path)])
            if not transcription:
                raise RuntimeError("Transcription returned empty result")

            text = (
                transcription[0].text
                if hasattr(transcription[0], "text")
                else str(transcription[0])
            )

            # Get duration
            with wave.open(str(audio_path), "r") as wav:
                duration = round(wav.getnframes() / float(wav.getframerate()), 1)

            return {
                "status": "complete",
                "text": text,
                "duration": duration,
                "word_count": len(text.split()),
                "model": model_name,
                "language": language,
            }

    def _send_callback(
        self,
        url: str,
        secret: str | None,
        data: dict,
    ) -> None:
        """Send callback with optional authentication."""
        import httpx

        headers = {}
        if secret:
            headers["Authorization"] = f"Bearer {secret}"

        try:
            httpx.post(url, json=data, headers=headers, timeout=30)
        except Exception:
            # Don't fail the transcription if callback fails
            pass


# Legacy function interface for backwards compatibility
@app.function(
    image=image,
    gpu="L4",
    timeout=1800,
    volumes={"/models": model_volume},
    secrets=[modal.Secret.from_name("yt-text-secrets", required=False)],
)
def transcribe(
    url: str,
    job_id: str | None = None,
    callback_url: str | None = None,
    callback_secret: str | None = None,
    language: str = "en",
) -> dict:
    """Wrapper for backwards compatibility."""
    return Transcriber().transcribe.remote(
        url=url,
        job_id=job_id,
        callback_url=callback_url,
        callback_secret=callback_secret,
        language=language,
    )


@app.local_entrypoint()
def main(url: str = "", language: str = "en"):
    """CLI entrypoint for testing."""
    if not url:
        print("Usage: modal run app.py --url <youtube-url> [--language en]")
        return

    print(f"Transcribing: {url}")
    transcriber = Transcriber()
    result = transcriber.transcribe.remote(url=url, language=language)
    print("\n--- Result ---")
    print(f"Duration: {result['duration']}s")
    print(f"Words: {result['word_count']}")
    print(f"Model: {result['model']}")
    print(f"\n{result['text']}")
