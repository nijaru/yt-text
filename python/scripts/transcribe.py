import argparse
import gc
import json
import os
import random
import re
import shutil
import subprocess
import sys
import tempfile
import time
import uuid
from dataclasses import asdict, dataclass
from typing import Any

import torch
import yt_dlp
from faster_whisper import WhisperModel
from pydub import AudioSegment

# System Constants
TEMP_DIR = "/tmp/transcribe"
MAX_WORKERS = 4

# Video Constraints
MAX_VIDEO_DURATION = 4 * 3600  # 4 hours in seconds
MAX_FILE_SIZE = 100 * 1024 * 1024  # 100MB in bytes


class TranscriptionError(Exception):
    """Base exception for transcription errors"""

    pass


class VideoValidationError(TranscriptionError):
    """Video validation errors"""

    pass


class AudioProcessingError(TranscriptionError):
    """Audio processing errors"""

    pass


class ModelError(TranscriptionError):
    """Model-related errors"""

    pass


class OutputError(TranscriptionError):
    """Output handling errors"""

    pass


class ResourceManager:
    def __init__(self):
        self.temp_files = []
        self.temp_dirs = []

    def add_file(self, path: str):
        self.temp_files.append(path)

    def add_dir(self, path: str):
        self.temp_dirs.append(path)

    def cleanup(self):
        for file in self.temp_files:
            try:
                if file and os.path.exists(file):
                    os.remove(file)
            except OSError as e:
                print(f"Warning: Failed to remove {file}: {e}", file=sys.stderr)

        for dir in self.temp_dirs:
            try:
                if dir and os.path.exists(dir):
                    shutil.rmtree(dir)
            except OSError as e:
                print(f"Warning: Failed to remove {dir}: {e}", file=sys.stderr)

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.cleanup()


@dataclass
class TranscriptionConfig:
    url: str
    model_name: str = "base.en"
    temperature: float = 0.2
    beam_size: int = 2
    best_of: int = 1
    json_output: bool = False
    temp_dir: str = TEMP_DIR
    download_dir: str | None = None

    def __post_init__(self):
        """Validate configuration on creation"""
        # Set download_dir if not provided
        if not self.download_dir:
            self.download_dir = os.path.join(self.temp_dir, "downloads")
        # Ensure directories exist
        os.makedirs(self.temp_dir, mode=0o755, exist_ok=True)
        os.makedirs(self.download_dir, mode=0o755, exist_ok=True)
        # Validate config
        self.validate()

    def validate(self):
        """Validate all configuration settings"""
        # Model settings validation
        if not 0.0 <= self.temperature <= 1.0:
            raise ValueError("Temperature must be between 0.0 and 1.0")
        if not 0 <= self.beam_size <= 5:
            raise ValueError("Beam size must be between 0 and 5")
        if not 1 <= self.best_of <= 5:
            raise ValueError("Best of must be between 1 and 5")

        # Path validation
        if not os.path.isdir(self.temp_dir):
            raise ValueError(f"Temp directory does not exist: {self.temp_dir}")
        if not os.access(self.temp_dir, os.W_OK):
            raise ValueError(f"Temp directory not writable: {self.temp_dir}")

        # Video constraints validation
        self.validate_video_constraints()

    def validate_video_constraints(self):
        """Validate video length and size"""
        try:
            validator = VideoValidator()
            validator.validate(self.url)
        except Exception as e:
            raise ValueError(f"Failed to validate video constraints: {str(e)}") from e

    @classmethod
    def from_args(cls, args: argparse.Namespace) -> "TranscriptionConfig":
        """Create config from command line arguments"""
        return cls(
            url=args.url,
            model_name=args.model,
            temperature=args.temperature,
            beam_size=args.beam_size,
            best_of=args.best_of,
            json_output=args.json,
            temp_dir=args.temp_dir if hasattr(args, "temp_dir") else TEMP_DIR,
            download_dir=(
                args.download_dir if hasattr(args, "download_dir") else TEMP_DIR
            ),
        )


class AudioProcessor:
    """Handles audio format conversion for transcription"""

    SUPPORTED_FORMATS = {".wav", ".mp3", ".ogg", ".flac"}

    def __init__(self, temp_dir: str = TEMP_DIR):
        self.temp_files: list[str] = []
        self.temp_dir = temp_dir

    def _create_temp_path(self, file_path: str) -> str:
        """Create unique temporary file path"""
        base_name = os.path.splitext(os.path.basename(file_path))[0]
        unique_id = str(uuid.uuid4())[:8]
        return os.path.join(self.temp_dir, f"{base_name}_{unique_id}_proc.ogg")

    def process(self, file_path: str) -> str:
        """Convert audio only if format not supported by Whisper"""
        try:
            ext = os.path.splitext(file_path)[1].lower()

            # If format is supported and sample rate is correct, return as-is
            if ext in self.SUPPORTED_FORMATS:
                audio = AudioSegment.from_file(file_path)
                if audio.frame_rate == 16000 and audio.channels == 1:
                    return file_path

            # Otherwise convert
            temp_path = self._create_temp_path(file_path)
            self.temp_files.append(temp_path)

            # Load and convert audio
            audio = AudioSegment.from_file(file_path)
            audio = audio.set_frame_rate(16000).set_channels(1)

            # Export
            audio.export(
                temp_path,
                format="webm",
                parameters=["-threads", "auto"],
            )
            os.chmod(temp_path, 0o644)

            return temp_path

        except Exception as e:
            print(
                json.dumps(
                    {"status": "error", "stage": "audio_processing", "error": str(e)}
                ),
                file=sys.stderr,
            )
            return file_path

    def cleanup(self) -> None:
        """Clean up temporary files"""
        for file in self.temp_files:
            try:
                if file and os.path.exists(file):
                    os.remove(file)
            except OSError as e:
                print(
                    json.dumps(
                        {
                            "status": "warning",
                            "stage": "cleanup",
                            "error": f"Failed to remove temporary file {file}: {e}",
                        }
                    ),
                    file=sys.stderr,
                )


class Transcriber:
    """Handles the transcription process using Whisper model"""

    def __init__(self, config: TranscriptionConfig):
        self.config = config
        self.model = self._setup_model()  # Initialize model in constructor
        self.progress: float = 0.0

    def transcribe(self, audio_file: str) -> str:
        """Transcribe audio file to text"""
        try:
            # Log transcription start
            print(
                json.dumps(
                    {
                        "status": "starting",
                        "stage": "transcription",
                        "model": self.config.model_name,
                    }
                ),
                file=sys.stderr,
            )

            text = self._perform_transcription(audio_file)

            # Log completion
            print(
                json.dumps(
                    {
                        "status": "complete",
                        "stage": "transcription",
                        "model": self.config.model_name,
                    }
                ),
                file=sys.stderr,
            )

            return text

        except Exception as e:
            print(
                json.dumps(
                    {"status": "error", "stage": "transcription", "error": str(e)}
                ),
                file=sys.stderr,
            )
            raise
        finally:
            self._cleanup()

    def _setup_model(self) -> WhisperModel:
        """Initialize the Whisper model with optimal settings"""
        try:
            # Log model loading
            print(
                json.dumps(
                    {
                        "status": "loading",
                        "stage": "model_setup",
                        "model": self.config.model_name,
                    }
                ),
                file=sys.stderr,
            )

            # Select device and compute type
            device = "cuda" if torch.cuda.is_available() else "cpu"
            compute_type = "float16" if device == "cuda" else "float32"

            # Set number of CPU threads if using CPU
            num_threads = os.cpu_count() or 4
            if device == "cpu":
                torch.set_num_threads(num_threads)

            # Initialize model
            model = WhisperModel(
                self.config.model_name,
                device=device,
                compute_type=compute_type,
                cpu_threads=num_threads if device == "cpu" else 0,
                download_root=os.path.join(self.config.temp_dir, "models"),
            )

            # Log successful model loading
            print(
                json.dumps(
                    {
                        "status": "ready",
                        "stage": "model_setup",
                        "device": device,
                        "compute_type": compute_type,
                    }
                ),
                file=sys.stderr,
            )

            return model

        except Exception as e:
            raise RuntimeError(f"Failed to setup model: {e}") from e

    def _perform_transcription(self, audio_file: str) -> str:
        """Perform the actual transcription with progress tracking"""
        try:
            segments, info = self.model.transcribe(
                audio_file,
                beam_size=self.config.beam_size,
                temperature=self.config.temperature,
                best_of=self.config.best_of,
                language="en",  # Explicitly set language
                vad_filter=True,  # Enable voice activity detection
                vad_parameters=dict(
                    min_silence_duration_ms=500
                ),  # Adjust silence detection
            )

            # Convert segments to list and process
            segments = list(segments)
            if not segments:
                raise TranscriptionError("No speech segments detected")

            transcribed_text = []
            total_segments = len(segments)

            for idx, segment in enumerate(segments, 1):
                # Skip empty segments
                if not segment.text or not segment.text.strip():
                    continue

                # Report progress
                progress = (idx / total_segments) * 100
                print(
                    json.dumps(
                        {
                            "status": "transcribing",
                            "progress": round(progress, 2),
                            "segment": idx,
                            "total": total_segments,
                        }
                    ),
                    file=sys.stderr,
                )

                transcribed_text.append(segment.text.strip())

            return " ".join(transcribed_text)

        except Exception as e:
            raise TranscriptionError(f"Transcription failed: {e}") from e

    def _cleanup(self) -> None:
        """Clean up model resources"""
        try:
            if self.model:
                del self.model
                if torch.cuda.is_available():
                    torch.cuda.empty_cache()
                gc.collect()

            print(
                json.dumps(
                    {
                        "status": "cleanup",
                        "stage": "transcription",
                        "message": "Resources released",
                    }
                ),
                file=sys.stderr,
            )

        except Exception as e:
            print(
                json.dumps(
                    {
                        "status": "warning",
                        "stage": "cleanup",
                        "error": f"Cleanup error: {e}",
                    }
                ),
                file=sys.stderr,
            )


@dataclass
class VideoMetadata:
    duration: float


class VideoValidator:
    def __init__(self, max_duration: int = MAX_VIDEO_DURATION):
        self.max_duration = max_duration

    def validate(self, url: str) -> VideoMetadata:
        metadata = self._fetch_metadata(url)
        self._check_duration(metadata)
        return metadata

    def _fetch_metadata(self, url: str) -> VideoMetadata:
        try:
            with yt_dlp.YoutubeDL({"quiet": True, "no_warnings": True}) as ydl:
                info = ydl.extract_info(url, download=False)
                if not info:
                    raise VideoValidationError("Could not fetch video information")
                if "duration" not in info:
                    raise VideoValidationError("Could not determine video duration")
                return VideoMetadata(duration=info["duration"])
        except yt_dlp.utils.DownloadError as e:
            raise VideoValidationError(f"Invalid or unsupported URL: {e}") from e
        except Exception as e:
            raise VideoValidationError(f"Failed to fetch video metadata: {e}") from e

    def _check_duration(self, metadata: VideoMetadata) -> None:
        if metadata.duration > self.max_duration:
            raise VideoValidationError(
                f"Video duration ({metadata.duration:.1f}s) exceeds maximum allowed ({self.max_duration}s)"
            )


class YoutubeDownloader:
    def __init__(self, base_dir: str = "/tmp/transcribe"):
        self.base_dir = base_dir

    def download(self, url: str) -> str:
        temp_dir = self._create_temp_dir()
        try:
            return self._download_audio(url, temp_dir)
        except Exception as e:
            self._cleanup(temp_dir)
            raise RuntimeError(f"Download failed: {e}") from e

    def _create_temp_dir(self) -> str:
        if not os.path.exists(self.base_dir):
            os.makedirs(self.base_dir, mode=0o755)
        temp_dir = tempfile.mkdtemp(dir=self.base_dir)
        os.chmod(temp_dir, 0o755)
        return temp_dir

    def _download_audio(self, url: str, temp_dir: str) -> str:
        ydl_opts = {
            "format": "bestaudio/best",
            "outtmpl": os.path.join(temp_dir, "%(id)s.%(ext)s"),
            "quiet": True,
            "no_warnings": True,
            "cachedir": False,
        }
        with yt_dlp.YoutubeDL(ydl_opts) as ydl:
            info = ydl.extract_info(url, download=True)
            fname = ydl.prepare_filename(info)
            os.chmod(fname, 0o644)
            return fname

    def _cleanup(self, temp_dir: str) -> None:
        if temp_dir and os.path.exists(temp_dir):
            shutil.rmtree(temp_dir)


class TextProcessor:
    def __init__(self):
        self.multiple_spaces = re.compile(r"\s+")
        self.period_space = re.compile(r"\.\s+([a-z])")
        self.repeated_punct = re.compile(r"([.!?,]){2,}")

    def process(self, text: str) -> str:
        text = self._cleanup(text)
        return self._organize(text)

    def _cleanup(self, text: str) -> str:
        text = text.strip()
        text = self.multiple_spaces.sub(" ", text)
        text = text.replace(" .", ".").replace(" ,", ",")
        text = text.replace(" ?", "?").replace(" !", "!")
        text = self.period_space.sub(lambda m: ". " + m.group(1).upper(), text)
        text = self.repeated_punct.sub(r"\1", text)
        if text and text[0].isalpha():
            text = text[0].upper() + text[1:]
        return text

    def _organize(self, text: str) -> str:
        sentences = re.split(r"(?<=[.!?])\s+", text)
        paragraphs: list[list[str]] = [[]]
        for sentence in sentences:
            if len(paragraphs[-1]) >= 5:
                paragraphs.append([])
            paragraphs[-1].append(sentence)
        return "\n\n".join(" ".join(para) for para in paragraphs)


class OutputHandler:
    def __init__(self, config: TranscriptionConfig):
        self.config = config

    def output(self, text: str, temp_dir: str) -> None:
        try:
            if self.config.json_output:
                self._output_json(text)
            else:
                self._output_file(text, temp_dir)
        except Exception as e:
            raise RuntimeError(f"Output failed: {e}") from e

    def _output_json(self, text: str) -> None:
        response = {
            "transcription": text,
            "model_name": self.config.model_name,
            "settings": asdict(self.config),
        }
        print(json.dumps(response))

    def _output_file(self, text: str, temp_dir: str) -> None:
        temp_output = os.path.join(temp_dir, f"transcript_{str(uuid.uuid4())[:8]}.txt")
        with open(temp_output, "w", encoding="utf-8") as f:
            f.write(text)
        print(temp_output)


class Application:
    def __init__(self, config: TranscriptionConfig):
        self.config = config
        self.validator = VideoValidator()
        self.downloader = YoutubeDownloader()
        self.audio_processor = AudioProcessor(temp_dir=config.temp_dir)
        self.transcriber = Transcriber(config)
        self.text_processor = TextProcessor()
        self.output_handler = OutputHandler(config)

    def run(self) -> None:
        with ResourceManager() as resources:
            try:
                # Validate video
                self.validator.validate(self.config.url)

                # Create temporary directory
                temp_dir = tempfile.mkdtemp(dir=self.config.temp_dir)
                resources.add_dir(temp_dir)

                # Download and process
                downloaded_file = retry_with_backoff(
                    lambda: self.downloader.download(self.config.url)
                )
                resources.add_file(downloaded_file)

                processed_file = self.audio_processor.process(downloaded_file)
                resources.add_file(processed_file)

                # Transcribe and process text
                text = retry_with_backoff(
                    lambda: self.transcriber.transcribe(processed_file)
                )
                text = self.text_processor.process(text)

                # Output results
                self.output_handler.output(text, temp_dir)

            except TranscriptionError as e:
                print(json.dumps({"error": str(e)}), file=sys.stderr)
                sys.exit(1)


def check_dependencies():
    try:
        subprocess.run(["ffmpeg", "-version"], capture_output=True, check=True)
    except (subprocess.SubprocessError, FileNotFoundError) as err:
        raise RuntimeError("ffmpeg is not installed or not accessible") from err


def retry_with_backoff(
    func,
    max_retries: int = 3,
    initial_backoff: int = 2,
    max_backoff: int = 30,
    backoff_factor: float = 2.0,
) -> Any:
    for attempt in range(1, max_retries + 1):
        try:
            return func()
        except Exception:
            if attempt == max_retries:
                raise
            backoff = min(
                initial_backoff * (backoff_factor ** (attempt - 1)), max_backoff
            )
            time.sleep(backoff + random.uniform(0, backoff / 2))


def parse_args():
    parser = argparse.ArgumentParser(
        description="Download audio from youtube video and convert it to text"
    )
    parser.add_argument("url", type=str, help="URL of the youtube video")
    parser.add_argument(
        "--model", type=str, default="base.en", help="Name of the Whisper model to use"
    )
    parser.add_argument(
        "--json", action="store_true", help="Return the transcription as a JSON object"
    )
    parser.add_argument(
        "--temperature",
        type=float,
        default=0.2,
        help="Model temperature (0.0-1.0). Higher = more creative, Lower = more deterministic",
    )
    parser.add_argument(
        "--beam-size",
        type=int,
        default=3,
        help="Beam search size (0-5). Higher = more accurate but slower. 0 disables beam search",
    )
    parser.add_argument(
        "--best-of",
        type=int,
        default=1,
        help="Number of transcription attempts (1-5). Higher = more accurate but slower",
    )
    parser.add_argument(
        "--temp-dir", type=str, default=TEMP_DIR, help="Directory for temporary files"
    )
    parser.add_argument(
        "--download-dir",
        type=str,
        help="Directory for downloaded files (default: <temp_dir>/downloads)",
    )
    parser.add_argument(
        "--max-duration",
        type=int,
        default=MAX_VIDEO_DURATION,
        help="Maximum video duration in seconds (default: 14400, 4 hours)",
    )
    return parser.parse_args()


def main():
    check_dependencies()
    args = parse_args()
    config = TranscriptionConfig.from_args(args)
    app = Application(config)
    app.run()


if __name__ == "__main__":
    main()
