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

import numpy as np
import torch
import yt_dlp
from faster_whisper import WhisperModel
from pydub import AudioSegment

# System Constants
TEMP_DIR = "/tmp/transcribe"
MAX_WORKERS = 4

# Audio Constants
AUDIO_SAMPLE_RATE = 16000
AUDIO_HIGH_PASS = 80
AUDIO_LOW_PASS = 8000

# Video Constraints
MAX_VIDEO_DURATION = 4 * 3600  # 4 hours in seconds
MAX_FILE_SIZE = 100 * 1024 * 1024  # 100MB in bytes


@dataclass
class TranscriptionConfig:
    url: str
    model_name: str = "base.en"
    temperature: float = 0.2
    beam_size: int = 2
    best_of: int = 1
    processing_level: str = "light"
    json_output: bool = False
    temp_dir: str = TEMP_DIR

    def __post_init__(self):
        """Validate configuration on creation"""
        self.validate()

    def validate(self):
        """Validate all configuration settings"""
        if not 0.0 <= self.temperature <= 1.0:
            raise ValueError("Temperature must be between 0.0 and 1.0")
        if not 0 <= self.beam_size <= 5:
            raise ValueError("Beam size must be between 0 and 5")
        if not 1 <= self.best_of <= 5:
            raise ValueError("Best of must be between 1 and 5")
        if self.processing_level not in [
            "none",
            "light",
            "medium",
        ]:  # Changed from "heavy"
            raise ValueError("Invalid processing level")

        # Validate video constraints
        self.validate_video_constraints()

    def validate_video_constraints(self):
        """Validate video length and size"""
        duration = check_video_length(self.url)
        if duration > MAX_VIDEO_DURATION:
            raise ValueError(f"Video is longer than {MAX_VIDEO_DURATION//3600} hours")

        size = check_file_size(self.url)
        if size > MAX_FILE_SIZE:
            raise ValueError(f"File size exceeds {MAX_FILE_SIZE//1024//1024}MB")

    @classmethod
    def from_args(cls, args: argparse.Namespace) -> "TranscriptionConfig":
        """Create config from command line arguments"""
        return cls(
            url=args.url,
            model_name=args.model,
            temperature=args.temperature,
            beam_size=args.beam_size,
            best_of=args.best_of,
            processing_level=args.processing_level,
            json_output=args.json,
        )


class AudioProcessor:
    """Handles audio processing and optimization for transcription"""

    def __init__(self, processing_level: str = "light"):
        self.processing_level = processing_level
        self.temp_files: list[str] = []

    def process(self, file_path: str) -> str:
        """Main processing pipeline"""
        try:
            # Create unique temp filename
            temp_path = self._create_temp_path(file_path)
            self.temp_files.append(temp_path)

            # Load and optimize audio
            audio = self._load_audio(file_path)
            audio = self._optimize_audio(audio)

            if self.processing_level == "medium":
                audio = self._apply_medium_processing(audio)

            # Export with optimal settings
            self._export_audio(audio, temp_path)
            return temp_path

        except Exception as e:
            print(
                json.dumps({"error": f"Audio processing failed: {e}"}), file=sys.stderr
            )
            return file_path

    def _create_temp_path(self, file_path: str) -> str:
        """Create unique temporary file path"""
        temp_dir = os.path.dirname(file_path)
        base_name = os.path.splitext(os.path.basename(file_path))[0]
        unique_id = str(uuid.uuid4())[:8]
        return os.path.join(temp_dir, f"{base_name}_{unique_id}_proc.ogg")

    def _load_audio(self, file_path: str) -> AudioSegment:
        """Load audio with optimal settings"""
        return AudioSegment.from_file(file_path, parameters=["-threads", "auto", "-vn"])

    def _optimize_audio(self, audio: AudioSegment) -> AudioSegment:
        """Apply basic optimizations"""
        return (
            audio.set_frame_rate(AUDIO_SAMPLE_RATE)
            .set_channels(1)
            .normalize()
            .high_pass_filter(AUDIO_HIGH_PASS)
            .low_pass_filter(AUDIO_LOW_PASS)
        )

    def _apply_medium_processing(self, audio: AudioSegment) -> AudioSegment:
        """Apply enhanced processing without librosa dependency"""
        # Convert to numpy array for processing
        samples = np.array(audio.get_array_of_samples(), dtype=np.float32)

        # Simple noise reduction using rolling median
        window_size = int(AUDIO_SAMPLE_RATE / 10)  # 100ms window
        samples = self._rolling_median_filter(samples, window_size)

        # Rebuild audio segment with processed samples
        processed_audio = AudioSegment(
            samples.astype(np.int16).tobytes(),
            frame_rate=AUDIO_SAMPLE_RATE,
            sample_width=2,
            channels=1,
        )

        # Apply dynamic range compression
        return processed_audio.compress_dynamic_range(threshold=-20.0, ratio=2.5)

    def _rolling_median_filter(
        self, samples: np.ndarray, window_size: int
    ) -> np.ndarray:
        """Apply a rolling median filter for noise reduction"""
        # Ensure window size is odd
        window_size = window_size + 1 if window_size % 2 == 0 else window_size

        # Pad the array to handle edges
        pad_width = window_size // 2
        padded = np.pad(samples, pad_width, mode="edge")

        # Apply rolling median
        result = np.zeros_like(samples)
        for i in range(len(samples)):
            result[i] = np.median(padded[i : i + window_size])

        return result

    def _export_audio(self, audio: AudioSegment, output_path: str) -> None:
        """Export audio with optimal settings"""
        audio.export(
            output_path,
            format="ogg",
            parameters=["-c:a", "libvorbis", "-q:a", "4", "-threads", "auto"],
        )

    def cleanup(self) -> None:
        """Clean up temporary files"""
        for file in self.temp_files:
            try:
                if file and os.path.exists(file):
                    os.remove(file)
            except OSError:
                pass


class Transcriber:
    """Handles the transcription process"""

    def __init__(self, config: TranscriptionConfig):
        self.config = config
        self.model = None

    def transcribe(self, audio_file: str) -> str:
        """Transcribe audio file to text"""
        try:
            self.model = self._setup_model()
            return self._perform_transcription(audio_file)
        finally:
            self._cleanup()

    def _setup_model(self) -> WhisperModel:
        """Initialize the Whisper model"""
        device = "cuda" if torch.cuda.is_available() else "cpu"
        compute_type = "float16" if device == "cuda" else "float32"

        return WhisperModel(
            self.config.model_name, device=device, compute_type=compute_type
        )

    def _perform_transcription(self, audio_file: str) -> str:
        """Perform the actual transcription"""
        segments, _ = self.model.transcribe(
            audio_file,
            temperature=self.config.temperature,
            beam_size=self.config.beam_size,
            best_of=self.config.best_of,
        )
        return " ".join([segment.text.strip() for segment in segments])

    def _cleanup(self) -> None:
        """Clean up model resources"""
        if self.model:
            del self.model
            if torch.cuda.is_available():
                torch.cuda.empty_cache()
            gc.collect()


def check_dependencies():
    try:
        subprocess.run(["ffmpeg", "-version"], capture_output=True, check=True)
    except (subprocess.SubprocessError, FileNotFoundError):
        raise RuntimeError("ffmpeg is not installed or not accessible")


def check_video_length(url) -> float:
    ydl_opts = {
        "quiet": True,
        "no_warnings": True,
    }
    with yt_dlp.YoutubeDL(ydl_opts) as ydl:
        info = ydl.extract_info(url, download=False)
        return info.get("duration", 0)


def check_file_size(url) -> int:
    ydl_opts = {
        "quiet": True,
        "no_warnings": True,
    }
    with yt_dlp.YoutubeDL(ydl_opts) as ydl:
        info = ydl.extract_info(url, download=False)
        filesize = info.get("filesize", 0)
        if filesize == 0:
            filesize = info.get("filesize_approx", 0)
        return filesize


def download_audio(url) -> str:
    temp_dir = None
    transcribe_dir = "/tmp/transcribe"
    if not os.path.exists(transcribe_dir):
        os.makedirs(transcribe_dir, mode=0o755)
    temp_dir = tempfile.mkdtemp(dir=transcribe_dir)
    os.chmod(temp_dir, 0o755)

    ydl_opts = {
        "format": "bestaudio",
        "outtmpl": os.path.join(temp_dir, "%(id)s.%(ext)s"),
        "quiet": True,
        "no_warnings": True,
        "cachedir": False,
    }
    try:
        with yt_dlp.YoutubeDL(ydl_opts) as ydl:
            info = ydl.extract_info(url, download=True)
            fname = ydl.prepare_filename(info)
            os.chmod(fname, 0o644)
            return fname
    except Exception as e:
        raise RuntimeError(f"Failed to download audio: {e}")
    finally:
        if temp_dir and os.path.exists(temp_dir):
            shutil.rmtree(temp_dir)


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


def cleanup_text(text: str) -> str:
    """Efficient text cleanup using regex compilation and string methods"""
    # Compile regex patterns once
    MULTIPLE_SPACES = re.compile(r"\s+")
    PERIOD_SPACE = re.compile(r"\.\s+([a-z])")
    REPEATED_PUNCT = re.compile(r"([.!?,]){2,}")

    # Basic cleanup
    text = text.strip()
    text = MULTIPLE_SPACES.sub(" ", text)

    # Fix punctuation spacing
    text = text.replace(" .", ".").replace(" ,", ",")
    text = text.replace(" ?", "?").replace(" !", "!")

    # Capitalize sentences
    text = PERIOD_SPACE.sub(lambda m: ". " + m.group(1).upper(), text)

    # Remove repeated punctuation
    text = REPEATED_PUNCT.sub(r"\1", text)

    # Ensure first character is capitalized
    if text and text[0].isalpha():
        text = text[0].upper() + text[1:]

    return text


def organize_text(text: str) -> str:
    """Organize text into logical paragraphs based on content"""
    sentences = re.split(r"(?<=[.!?])\s+", text)
    paragraphs: list[list[str]] = [[]]

    for sentence in sentences:
        if len(paragraphs[-1]) >= 5:
            paragraphs.append([])
        paragraphs[-1].append(sentence)

    return "\n\n".join(" ".join(para) for para in paragraphs)


def output_results(text: str, config: TranscriptionConfig, temp_dir: str) -> None:
    """Handle output formatting and file writing"""
    try:
        if config.json_output:
            response = {
                "transcription": text,
                "model_name": config.model_name,
                "settings": asdict(config),
            }
            print(json.dumps(response))
        else:
            temp_output = os.path.join(
                temp_dir, f"transcript_{str(uuid.uuid4())[:8]}.txt"
            )
            with open(temp_output, "w", encoding="utf-8") as f:
                f.write(text)
            print(temp_output)
    except Exception as e:
        raise RuntimeError(f"Failed to output results: {e}")


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
        default=2,
        help="Beam search size (0-5). Higher = more accurate but slower. 0 disables beam search",
    )
    parser.add_argument(
        "--best-of",
        type=int,
        default=1,
        help="Number of transcription attempts (1-5). Higher = more accurate but slower",
    )
    parser.add_argument(
        "--processing-level",
        choices=["none", "light", "medium"],  # Changed from "heavy"
        default="light",
        help="Audio processing level (default: light)",
    )
    return parser.parse_args()


def main():
    args = parse_args()
    audio_processor = AudioProcessor(args.processing_level)

    try:
        # Create and validate config
        config = TranscriptionConfig.from_args(args)
        transcriber = Transcriber(config)

        with tempfile.TemporaryDirectory(dir=config.temp_dir) as temp_dir:
            # Download audio
            downloaded_file = retry_with_backoff(lambda: download_audio(config.url))

            # Process audio
            processed_file = audio_processor.process(downloaded_file)

            # Transcribe
            text = retry_with_backoff(lambda: transcriber.transcribe(processed_file))

            # Post-process text
            text = cleanup_text(text)
            text = organize_text(text)

            # Output results
            output_results(text, config, temp_dir)

    except Exception as e:
        print(json.dumps({"error": str(e)}), file=sys.stderr)
        sys.exit(1)
    finally:
        audio_processor.cleanup()


if __name__ == "__main__":
    check_dependencies()
    main()
