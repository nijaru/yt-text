import argparse
import json
import os
import sys
import tempfile
import time

import torch
import yt_dlp
from faster_whisper import WhisperModel
from pydub import AudioSegment

# Constants
MAX_VIDEO_DURATION = 4 * 3600  # 4 hours in seconds
MAX_FILE_SIZE = 100 * 1024 * 1024  # 100MB


class TranscriptionError(Exception):
    """Base exception for transcription errors"""

    pass


class Transcriber:
    def __init__(self, url: str, model_name: str = "base.en"):
        self.url = url
        self.model_name = model_name
        self.device = "cuda" if torch.cuda.is_available() else "cpu"
        self.compute_type = "float16" if self.device == "cuda" else "float32"

    def process(self) -> dict:
        """Main processing pipeline"""
        try:
            # Create temp directory for processing
            with tempfile.TemporaryDirectory() as temp_dir:
                # Download audio
                audio_path = self._download_audio(temp_dir)

                # Process audio file
                audio_path = self._process_audio(audio_path)

                # Transcribe
                return self._transcribe(audio_path)

        except Exception as e:
            return {
                "error": str(e),
                "text": None,
                "model_name": self.model_name,
                "duration": 0,
            }

    def _download_audio(self, temp_dir: str) -> str:
        """Download audio from YouTube URL"""
        ydl_opts = {
            "format": "bestaudio/best",
            "outtmpl": os.path.join(temp_dir, "%(id)s.%(ext)s"),
            "quiet": True,
            "no_warnings": True,
            "extract_audio": True,
        }

        try:
            with yt_dlp.YoutubeDL(ydl_opts) as ydl:
                # First extract info without downloading
                info = ydl.extract_info(self.url, download=False)

                # Validate duration
                duration = info.get("duration", 0)
                if duration > MAX_VIDEO_DURATION:
                    raise TranscriptionError(
                        f"Video duration ({duration}s) exceeds maximum allowed ({MAX_VIDEO_DURATION}s)"
                    )

                # Download the audio
                info = ydl.extract_info(self.url, download=True)
                return ydl.prepare_filename(info)
        except Exception as e:
            raise TranscriptionError(f"Failed to download audio: {e}")

    def _process_audio(self, file_path: str) -> str:
        """Convert audio to required format"""
        try:
            # Load audio
            audio = AudioSegment.from_file(file_path)

            # Convert to mono and set sample rate
            audio = audio.set_frame_rate(16000).set_channels(1)

            # Create processed file path
            temp_path = f"{file_path}_processed.wav"

            # Export
            audio.export(
                temp_path, format="wav", parameters=["-ac", "1", "-ar", "16000"]
            )

            return temp_path
        except Exception as e:
            raise TranscriptionError(f"Failed to process audio: {e}")

    def _transcribe(self, audio_path: str) -> dict:
        """Transcribe audio file"""
        try:
            # Initialize model
            model = WhisperModel(
                self.model_name,
                device=self.device,
                compute_type=self.compute_type,
                download_root="/tmp/models",
            )

            # Transcribe
            start_time = time.time()
            segments, info = model.transcribe(
                audio_path,
                beam_size=5,
                vad_filter=True,
                vad_parameters=dict(min_silence_duration_ms=500),
            )

            # Process segments
            segments = list(segments)  # Convert generator to list
            if not segments:
                raise TranscriptionError("No speech detected")

            # Combine segments
            text = " ".join(seg.text.strip() for seg in segments if seg.text.strip())

            return {
                "text": text,
                "model_name": self.model_name,
                "duration": time.time() - start_time,
                "error": None,
            }

        except Exception as e:
            raise TranscriptionError(f"Transcription failed: {e}")


def main():
    parser = argparse.ArgumentParser(description="Transcribe YouTube video")
    parser.add_argument("url", help="YouTube URL")
    parser.add_argument("--model", default="base.en", help="Whisper model to use")
    parser.add_argument("--json", action="store_true", help="Output in JSON format")
    args = parser.parse_args()

    try:
        transcriber = Transcriber(args.url, args.model)
        result = transcriber.process()

        if args.json:
            print(json.dumps(result))
        else:
            if result.get("error"):
                print(f"Error: {result['error']}", file=sys.stderr)
                sys.exit(1)
            print(result["text"])

    except Exception as e:
        error_response = {
            "error": str(e),
            "text": None,
            "model_name": args.model,
            "duration": 0,
        }
        if args.json:
            print(json.dumps(error_response))
        else:
            print(f"Error: {str(e)}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
