import hashlib
import json
import logging
import math
import os
import sys
import tempfile
import time
from pathlib import Path
from typing import Optional

import psutil
import torch
import yt_dlp
from faster_whisper import WhisperModel

# Configure logging to use stderr instead of stdout
logging.basicConfig(level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s", stream=sys.stderr)
logger = logging.getLogger(__name__)


class TranscriptionError(Exception):
    """Base exception for transcription errors."""

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
        model_name: str = "large-v3-turbo",
        device: Optional[str] = None,
        compute_type: Optional[str] = None,
        max_video_duration: Optional[int] = None,
        max_file_size: Optional[int] = None,
        chunk_length_seconds: int = 120,
        initial_prompt: Optional[str] = None,
        cache_dir: Optional[str] = None,
        enable_cache: bool = True,
        max_cache_size_gb: float = 10.0,
    ):
        self.model_name = model_name
        self._temp_files = []  # Track temporary files for cleanup

        # Cache configuration
        self.enable_cache = enable_cache and bool(
            os.environ.get("ENABLE_CHUNK_CACHE", "true").lower() == "true"
        )
        self.cache_dir = cache_dir or os.environ.get("WHISPER_CACHE_DIR", "/tmp/audio_cache")
        self.max_cache_size_gb = float(os.environ.get("MAX_CACHE_SIZE_GB", str(max_cache_size_gb)))

        if self.enable_cache:
            os.makedirs(self.cache_dir, exist_ok=True)
            # Initialize cache on startup to clean old files
            self._manage_cache_size()

        # Optimize device selection based on available hardware
        self.device = self._optimize_device(device)

        # Optimize compute type based on device
        self.compute_type = compute_type or self._optimize_compute_type()

        self.max_video_duration = max_video_duration
        self.max_file_size = max_file_size
        self.chunk_length_seconds = chunk_length_seconds
        self.initial_prompt = initial_prompt

        # Optimize batch size based on available memory
        self.batch_size = self._optimize_batch_size()

        # Initialize model with optimized settings
        download_root = os.environ.get("WHISPER_DOWNLOAD_ROOT", "/tmp/models")
        self.model = WhisperModel(
            self.model_name,
            device=self.device,
            compute_type=self.compute_type,
            download_root=download_root,
            cpu_threads=self._optimize_cpu_threads(),
        )

    def _optimize_device(self, device: Optional[str]) -> str:
        """Optimize device selection based on available hardware."""
        if device:
            return device

        # Check for CUDA availability and memory
        if torch.cuda.is_available():
            # Check GPU memory to ensure we have enough
            free_memory_mb = torch.cuda.get_device_properties(0).total_memory / (1024 * 1024)
            if free_memory_mb > 4000:  # Need at least 4GB for large models
                return "cuda"

        return "cpu"

    def _optimize_compute_type(self) -> str:
        """Optimize compute type based on device and memory."""
        if self.device == "cuda":
            # Check for available FP16 support
            if torch.cuda.get_device_capability(0)[0] >= 7:
                return "float16"
        return "float32"

    def _optimize_cpu_threads(self) -> int:
        """Optimize CPU threads based on available cores."""
        if self.device == "cpu":
            # Use 80% of available logical CPUs for transcription
            return max(1, int(psutil.cpu_count(logical=True) * 0.8))
        return 4  # Default when using GPU

    def _generate_cache_key(self, video_id: str, chunk_index: int) -> str:
        """Generate a unique cache key for a video chunk."""
        # Create a deterministic key that includes model, video_id, and chunk_index
        # We include model name since results will be different by model
        key_string = f"{self.model_name}_{video_id}_{chunk_index}_{self.chunk_length_seconds}"
        return hashlib.md5(key_string.encode()).hexdigest()

    def _manage_cache_size(self) -> None:
        """Manage cache size by removing oldest files when cache exceeds max size."""
        if not self.enable_cache or not os.path.exists(self.cache_dir):
            return

        try:
            # Get all files in cache directory with their modification times
            cache_files = []
            total_size_bytes = 0

            for file_path in Path(self.cache_dir).glob("*"):
                if file_path.is_file():
                    size = file_path.stat().st_size
                    mtime = file_path.stat().st_mtime
                    cache_files.append((file_path, size, mtime))
                    total_size_bytes += size

            # Convert bytes to GB
            total_size_gb = total_size_bytes / (1024 * 1024 * 1024)

            # If cache size exceeds max, remove oldest files
            if total_size_gb > self.max_cache_size_gb:
                # Sort by modification time (oldest first)
                cache_files.sort(key=lambda x: x[2])

                # Remove files until we're under the limit (with 10% buffer)
                target_size = self.max_cache_size_gb * 0.9
                current_size = total_size_gb

                for file_path, size, _ in cache_files:
                    if current_size <= target_size:
                        break

                    try:
                        file_path.unlink()
                        size_gb = size / (1024 * 1024 * 1024)
                        current_size -= size_gb
                    except Exception:
                        pass

        except Exception:
            pass
            # Don't fail transcription due to cache management issues

    def _optimize_batch_size(self) -> int:
        """Optimize batch size based on available memory or environment settings."""
        # Check if batch size is set in environment
        env_batch_size = os.environ.get("WHISPER_BATCH_SIZE")
        if env_batch_size is not None:
            try:
                return int(env_batch_size)
            except (ValueError, TypeError):
                pass  # Fall back to auto-optimization if parsing fails

        if self.device == "cuda":
            # Estimate free GPU memory and adapt batch size
            free_memory_gb = torch.cuda.get_device_properties(0).total_memory / (1024 * 1024 * 1024)
            if free_memory_gb > 12:
                return 16
            if free_memory_gb > 8:
                return 8
            return 4
        # Estimate system memory and adapt CPU batch size
        system_memory_gb = psutil.virtual_memory().total / (1024 * 1024 * 1024)
        if system_memory_gb > 16:
            return 8
        if system_memory_gb > 8:
            return 4
        return 2

    def process_url(self, url: str) -> dict:
        """Process a single URL and return transcription result."""
        try:
            with tempfile.TemporaryDirectory() as temp_dir:
                # Download audio and retrieve media title
                audio_path, media_title, duration, video_id = self._download_audio(url, temp_dir)

                # Transcribe
                transcription = self._transcribe(audio_path, duration, video_id)

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
                "language": None,
                "language_probability": 0,
            }
        except FileNotFoundError as e:
            return {
                "error": f"Audio file not found: {e}",
                "text": None,
                "model_name": self.model_name,
                "duration": 0,
                "title": None,
                "url": url,
                "language": None,
                "language_probability": 0,
            }
        except PermissionError as e:
            return {
                "error": f"Permission denied accessing audio file: {e}",
                "text": None,
                "model_name": self.model_name,
                "duration": 0,
                "title": None,
                "url": url,
                "language": None,
                "language_probability": 0,
            }
        except OSError as e:
            return {
                "error": f"OS error during transcription: {e}",
                "text": None,
                "model_name": self.model_name,
                "duration": 0,
                "title": None,
                "url": url,
                "language": None,
                "language_probability": 0,
            }
        except Exception as e:
            import traceback

            error_info = traceback.format_exc()
            return {
                "error": f"Unexpected error: {e}\n{error_info}",
                "text": None,
                "model_name": self.model_name,
                "duration": 0,
                "title": None,
                "url": url,
                "language": None,
                "language_probability": 0,
            }

    def _download_audio(self, url: str, temp_dir: str) -> tuple[str, str, float, str]:
        """Download audio from URL and retrieve media title and duration.

        This uses a streaming approach to avoid loading the entire file into memory.

        Returns:
            Tuple containing:
            - Path to downloaded audio file
            - Media title
            - Duration in seconds
            - Video ID for caching
        """
        # First, extract info without downloading to get metadata
        info_opts = {
            "quiet": True,
            "no_warnings": True,
            "logger": NullLogger(),  # Suppress yt_dlp logs
            "extract_flat": True,  # Only extract basic metadata
        }

        try:
            # Step 1: Extract metadata without downloading
            with yt_dlp.YoutubeDL(info_opts) as ydl:
                info = ydl.extract_info(url, download=False)
                if not isinstance(info, dict):
                    raise TranscriptionError("Failed to extract video information.")

                # Get metadata
                media_title = info.get("title") or url
                duration = info.get("duration", 0)
                video_id = info.get("id", hashlib.md5(url.encode()).hexdigest())

                # Validate duration if constraint is set
                if self.max_video_duration and duration > self.max_video_duration:
                    raise TranscriptionError(
                        f"Media duration ({duration}s) exceeds maximum allowed ({self.max_video_duration}s)"
                    )

            # Step 2: Configure download with streaming settings
            stream_opts = {
                "format": "bestaudio/best",
                "outtmpl": os.path.join(temp_dir, "%(id)s.%(ext)s"),
                "quiet": True,
                "no_warnings": True,
                "extractaudio": True,
                "postprocessors": [
                    {
                        "key": "FFmpegExtractAudio",
                        "preferredcodec": "wav",
                        "preferredquality": "192",
                    }
                ],
                "audioformat": "wav",
                "audioquality": 0,  # Best quality
                "logger": NullLogger(),  # Suppress yt_dlp logs
                # Stream optimization settings
                "buffersize": 1024 * 1024,  # 1MB buffer
                "external_downloader_args": ["-bufsize", "1M"],  # Smaller FFmpeg buffer
                "concurrent_fragment_downloads": 1,  # Limit concurrent downloads
                "progress_hooks": [self._monitor_download_progress],
            }

            # Step 3: Download audio in streaming mode
            with yt_dlp.YoutubeDL(stream_opts) as ydl:
                # Download audio in streaming mode
                download_info = ydl.extract_info(url, download=True)
                if not isinstance(download_info, dict):
                    raise TranscriptionError("Failed to download audio.")

                downloaded_file = ydl.prepare_filename(download_info)

                # Handle the case where FFmpeg has converted to WAV
                base, _ = os.path.splitext(downloaded_file)
                wav_file = f"{base}.wav"
                if os.path.exists(wav_file):
                    downloaded_file = wav_file

                # Add to temp files for cleanup
                self._temp_files.append(downloaded_file)

                # Check file size (if constraint is set)
                # We just check the file size, not load it into memory
                if self.max_file_size:
                    file_size = os.path.getsize(downloaded_file)
                    if file_size > self.max_file_size:
                        os.remove(downloaded_file)  # Clean up immediately
                        self._temp_files.remove(downloaded_file)
                        raise TranscriptionError(
                            f"Downloaded file size ({file_size} bytes) exceeds maximum allowed ({self.max_file_size} bytes)"
                        )

                return downloaded_file, media_title, duration, video_id

        except TranscriptionError:
            raise
        except Exception as e:
            raise TranscriptionError(f"Failed to download audio: {e}")

    def _monitor_download_progress(self, progress):
        """Monitor download progress and report status."""
        if progress["status"] == "downloading":
            # We could implement progress reporting here
            # This would connect with the gRPC progress streaming functionality
            pass
        elif progress["status"] == "error":
            pass
        elif progress["status"] == "finished":
            # We could report download completion here
            pass

    def _should_chunk_audio(self, duration: float) -> bool:
        """Determine if audio should be chunked based on duration."""
        return duration > self.chunk_length_seconds

    def _transcribe_with_temperature_fallback(
        self, audio_path: str, segment_index: int = 0, segment_count: int = 1
    ) -> tuple[list, dict]:
        """Transcribe with temperature fallback for improved accuracy."""
        # Read environment variables if set, otherwise use defaults
        beam_size = int(os.environ.get("WHISPER_BEAM_SIZE", 5))
        best_of = int(os.environ.get("WHISPER_BEST_OF", 1))
        use_vad = os.environ.get("WHISPER_VAD_FILTER", "true").lower() == "true"
        vad_min_silence = int(os.environ.get("WHISPER_VAD_MIN_SILENCE_MS", 300))
        vad_speech_pad = int(os.environ.get("WHISPER_VAD_SPEECH_PAD_MS", 100))
        vad_threshold = float(os.environ.get("WHISPER_VAD_THRESHOLD", 0.35))
        env_temp = os.environ.get("WHISPER_TEMPERATURE")

        # If environment variable is set and not using fallback
        if env_temp is not None and env_temp.lower() != "fallback":
            temperatures = [float(env_temp)]
        else:
            temperatures = [0.0, 0.2, 0.4, 0.6, 0.8]

        last_exception = None

        for temp in temperatures:
            try:
                f"Transcribing segment {segment_index + 1}/{segment_count}"
                if segment_count > 1:
                    pass

                result = self.model.transcribe(
                    audio_path,
                    beam_size=beam_size,
                    temperature=temp,
                    best_of=best_of,
                    condition_on_previous_text=True,
                    vad_filter=use_vad,
                    vad_parameters={
                        "min_silence_duration_ms": vad_min_silence,  # Enhanced for YouTube content
                        "speech_pad_ms": vad_speech_pad,
                        "threshold": vad_threshold,
                    },
                    initial_prompt=self.initial_prompt,
                    language=None,  # Auto-detect language
                    task="transcribe",
                    batch_size=self.batch_size,
                )

                # Debug info
                segments_list = list(result[0])
                if len(segments_list) > 0:
                    pass
                else:
                    pass

                return result
            except Exception as e:
                last_exception = e
                continue

        raise TranscriptionError(f"All temperature attempts failed: {last_exception}")

    def _save_to_cache(self, video_id: str, chunk_index: int, result_data: dict) -> str:
        """Save transcription result to cache and return cache path."""
        if not self.enable_cache:
            return None

        try:
            # Create cache key
            cache_key = self._generate_cache_key(video_id, chunk_index)
            cache_path = os.path.join(self.cache_dir, f"{cache_key}.json")

            # Save to cache file
            with open(cache_path, "w") as f:
                json.dump(result_data, f)

            # Touch file to update access time (for cache eviction)
            os.utime(cache_path, None)

            return cache_path
        except Exception:
            return None

    def _get_from_cache(self, video_id: str, chunk_index: int) -> Optional[dict]:
        """Try to get transcription result from cache."""
        if not self.enable_cache:
            return None

        try:
            # Create cache key
            cache_key = self._generate_cache_key(video_id, chunk_index)
            cache_path = os.path.join(self.cache_dir, f"{cache_key}.json")

            # Check if cache file exists
            if not os.path.exists(cache_path):
                return None

            # Load from cache file
            with open(cache_path) as f:
                cached_data = json.load(f)

            # Touch file to update access time (for cache eviction)
            os.utime(cache_path, None)

            return cached_data
        except Exception:
            return None

    def _transcribe(self, audio_path: str, duration: float, video_id: str) -> dict:
        """Transcribe audio file with streaming chunking support and caching for longer files."""
        try:
            start_time = time.time()
            text_parts = []
            language_info = None
            cache_hits = 0
            cache_misses = 0

            if self._should_chunk_audio(duration):
                # Use streaming chunks for long audio files
                from pydub import AudioSegment
                from pydub.utils import make_chunks

                # Create a stream context manager to manage resources efficiently
                class AudioStreamContext:
                    def __init__(self, audio_path):
                        self.audio_path = audio_path
                        self.segment = None
                        self.temp_chunks = []

                    def __enter__(self):
                        # Load the audio file using pydub - more memory efficient than librosa for this use case
                        # Load in 5MB chunks to avoid memory issues
                        self.segment = AudioSegment.from_file(
                            self.audio_path, chunk_size=5 * 1024 * 1024
                        )
                        return self

                    def __exit__(self, exc_type, exc_val, exc_tb):
                        # Clean up all temp chunk files
                        for chunk_path in self.temp_chunks:
                            try:
                                if os.path.exists(chunk_path):
                                    os.remove(chunk_path)
                            except Exception:
                                pass

                        # Release memory
                        self.segment = None

                # Process the audio in streaming chunks
                with AudioStreamContext(audio_path) as stream:
                    # Calculate chunk size in milliseconds
                    chunk_length_ms = self.chunk_length_seconds * 1000
                    math.ceil(len(stream.segment) / chunk_length_ms)

                    audio_chunks = make_chunks(stream.segment, chunk_length_ms)

                    for i, chunk in enumerate(audio_chunks):
                        # First check cache for this chunk
                        cached_result = None
                        if self.enable_cache:
                            cached_result = self._get_from_cache(video_id, i)

                        if cached_result:
                            # Cache hit
                            cache_hits += 1

                            # Extract text from cached result
                            chunk_text = cached_result.get("text", "")
                            if chunk_text:
                                text_parts.append(chunk_text)

                            # Save language info from first chunk if needed
                            if i == 0 and not language_info:
                                language_info = {
                                    "language": cached_result.get("language"),
                                    "language_probability": cached_result.get(
                                        "language_probability", 0
                                    ),
                                }

                            # Report progress
                            (i + 1) / len(audio_chunks)
                            continue

                        # Cache miss - process the chunk
                        cache_misses += 1

                        # Create temporary chunk file
                        chunk_path = f"{audio_path}_stream_chunk_{i}.wav"

                        # Write chunk to disk in streaming mode
                        try:
                            chunk.export(chunk_path, format="wav")
                            stream.temp_chunks.append(chunk_path)
                            self._temp_files.append(chunk_path)  # Add to main cleanup list too

                            # Report progress
                            (i + 1) / len(audio_chunks)

                            # Process this chunk
                            segments, info = self._transcribe_with_temperature_fallback(
                                chunk_path, i, len(audio_chunks)
                            )

                            # Save language info from first chunk
                            if i == 0 and info:
                                language_info = info

                            # Process segments
                            chunk_segments = list(segments)
                            chunk_text = ""
                            if chunk_segments:
                                chunk_text = " ".join(
                                    seg.text.strip() for seg in chunk_segments if seg.text.strip()
                                )
                                text_parts.append(chunk_text)

                            # Save to cache
                            if self.enable_cache and chunk_text:
                                cache_data = {
                                    "text": chunk_text,
                                    "model_name": self.model_name,
                                    "language": (language_info.language if language_info else None),
                                    "language_probability": (
                                        language_info.language_probability if language_info else 0
                                    ),
                                    "timestamp": time.time(),
                                }
                                cache_path = self._save_to_cache(video_id, i, cache_data)
                                if cache_path:
                                    pass

                            # Remove chunk immediately to free disk space
                            try:
                                os.remove(chunk_path)
                                stream.temp_chunks.remove(chunk_path)
                                self._temp_files.remove(chunk_path)
                            except Exception:
                                pass

                            # Force memory cleanup after each chunk
                            if torch.cuda.is_available():
                                torch.cuda.empty_cache()
                            import gc

                            gc.collect()

                        except Exception:
                            # Continue with the next chunk despite errors
                            continue

                if not text_parts:
                    raise TranscriptionError("No speech detected in any chunk")
            else:
                # Process short audio without chunking
                # Check cache first
                cached_result = None
                if self.enable_cache:
                    cached_result = self._get_from_cache(video_id, 0)

                if cached_result:
                    # Cache hit for the entire short file
                    cache_hits += 1

                    # Extract text from cached result
                    text = cached_result.get("text", "")
                    if text:
                        text_parts = [text]

                    # Save language info
                    language_info = {
                        "language": cached_result.get("language"),
                        "language_probability": cached_result.get("language_probability", 0),
                    }
                else:
                    # Cache miss - process the audio
                    cache_misses += 1
                    segments, info = self._transcribe_with_temperature_fallback(audio_path)
                    language_info = info

                    segments = list(segments)
                    if not segments:
                        raise TranscriptionError("No speech detected")

                    text = " ".join(seg.text.strip() for seg in segments if seg.text.strip())
                    text_parts = [text]

                    # Save to cache
                    if self.enable_cache and text:
                        cache_data = {
                            "text": text,
                            "model_name": self.model_name,
                            "language": (language_info.language if language_info else None),
                            "language_probability": (
                                language_info.language_probability if language_info else 0
                            ),
                            "timestamp": time.time(),
                        }
                        cache_path = self._save_to_cache(video_id, 0, cache_data)
                        if cache_path:
                            pass

            # Combine all text parts
            final_text = " ".join(text_parts)

            # Cache statistics
            cache_stats = {
                "hits": cache_hits,
                "misses": cache_misses,
                "hit_ratio": (
                    cache_hits / (cache_hits + cache_misses)
                    if (cache_hits + cache_misses) > 0
                    else 0
                ),
            }

            result = {
                "text": final_text,
                "model_name": self.model_name,
                "duration": time.time() - start_time,
                "error": None,
                "cache_stats": cache_stats,
            }

            # Add language detection info
            if isinstance(language_info, dict):
                # Handle the case when language_info comes from cache
                result["language"] = language_info.get("language")
                result["language_probability"] = language_info.get("language_probability", 0)
            elif language_info:
                # Handle the case when language_info is a Whisper info object
                result["language"] = language_info.language
                result["language_probability"] = language_info.language_probability

            # Manage cache size after processing
            if self.enable_cache:
                self._manage_cache_size()

            # Final memory cleanup
            if torch.cuda.is_available():
                torch.cuda.empty_cache()
            import gc

            gc.collect()

            return result

        except Exception as e:
            raise TranscriptionError(f"Transcription failed: {e}")

    def close(self):
        """Clean up resources if necessary."""
        try:
            # Clean up model
            if hasattr(self, "model"):
                del self.model

            # Clean GPU memory if available
            if torch.cuda.is_available():
                torch.cuda.empty_cache()

            # Force garbage collection
            import gc

            gc.collect()

            # Clean any temporary files
            if hasattr(self, "_temp_files") and self._temp_files:
                for temp_file in self._temp_files:
                    try:
                        if os.path.exists(temp_file):
                            os.remove(temp_file)
                    except Exception:
                        pass  # Best effort cleanup
        except Exception:
            pass
