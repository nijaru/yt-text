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


@app.function(
    image=image,
    gpu="L4",
    timeout=1800,  # 30 minutes max
    volumes={"/models": model_volume},
    secrets=[modal.Secret.from_name("yt-text-secrets", required=False)],
)
def transcribe(
    url: str,
    job_id: str | None = None,
    callback_url: str | None = None,
    language: str = "en",
) -> dict:
    """
    Download and transcribe audio from a URL.

    Args:
        url: URL to download audio from (YouTube, etc.)
        job_id: Optional job ID for tracking
        callback_url: Optional webhook URL for completion notification
        language: Language code (default: en)

    Returns:
        dict with text, duration, word_count, and metadata
    """
    import httpx
    import nemo.collections.asr as nemo_asr

    # Set model cache directory
    os.environ["NEMO_CACHE_DIR"] = "/models"

    with tempfile.TemporaryDirectory() as tmpdir:
        audio_path = Path(tmpdir) / "audio.wav"

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

        # Load Parakeet model
        model_name = "nvidia/parakeet-tdt-0.6b-v2"
        if language != "en":
            model_name = "nvidia/parakeet-tdt-0.6b-v3"  # Multilingual

        model = nemo_asr.models.ASRModel.from_pretrained(model_name)

        # Transcribe
        transcription = model.transcribe([str(audio_path)])
        text = transcription[0].text if hasattr(transcription[0], "text") else str(transcription[0])

        # Get audio duration
        import wave

        with wave.open(str(audio_path), "r") as wav:
            frames = wav.getnframes()
            rate = wav.getframerate()
            duration = frames / float(rate)

        result = {
            "job_id": job_id,
            "status": "complete",
            "text": text,
            "duration": int(duration),
            "word_count": len(text.split()),
            "model": model_name,
            "language": language,
        }

        # Send callback if provided
        if callback_url:
            try:
                httpx.post(callback_url, json=result, timeout=30)
            except Exception as e:
                result["callback_error"] = str(e)

        return result


@app.function(image=image, gpu="L4", timeout=600, volumes={"/models": model_volume})
def transcribe_file(audio_bytes: bytes, filename: str = "audio.wav") -> dict:
    """
    Transcribe audio from raw bytes (for direct file uploads).

    Args:
        audio_bytes: Raw audio file bytes
        filename: Original filename for format detection

    Returns:
        dict with text, duration, word_count, and metadata
    """
    import wave

    import nemo.collections.asr as nemo_asr

    os.environ["NEMO_CACHE_DIR"] = "/models"

    with tempfile.TemporaryDirectory() as tmpdir:
        # Write audio to temp file
        audio_path = Path(tmpdir) / filename
        audio_path.write_bytes(audio_bytes)

        # Convert to WAV if needed
        if not filename.endswith(".wav"):
            wav_path = Path(tmpdir) / "audio.wav"
            subprocess.run(
                ["ffmpeg", "-i", str(audio_path), "-ar", "16000", "-ac", "1", str(wav_path)],
                capture_output=True,
                check=True,
            )
            audio_path = wav_path

        # Load and transcribe
        model = nemo_asr.models.ASRModel.from_pretrained("nvidia/parakeet-tdt-0.6b-v2")
        transcription = model.transcribe([str(audio_path)])
        text = transcription[0].text if hasattr(transcription[0], "text") else str(transcription[0])

        # Get duration
        with wave.open(str(audio_path), "r") as wav:
            duration = wav.getnframes() / float(wav.getframerate())

        return {
            "status": "complete",
            "text": text,
            "duration": int(duration),
            "word_count": len(text.split()),
            "model": "nvidia/parakeet-tdt-0.6b-v2",
        }


@app.local_entrypoint()
def main(url: str = ""):
    """CLI entrypoint for testing."""
    if not url:
        print("Usage: modal run app.py --url <youtube-url>")
        return

    print(f"Transcribing: {url}")
    result = transcribe.remote(url)
    print("\n--- Result ---")
    print(f"Duration: {result['duration']}s")
    print(f"Words: {result['word_count']}")
    print(f"\n{result['text']}")
