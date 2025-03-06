import os
import tempfile
import time
import math
import psutil
from typing import Dict, Optional, List, Union, Tuple

import torch
import yt_dlp
import numpy as np
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
        model_name: str = "large-v3-turbo",
        device: Optional[str] = None,
        compute_type: Optional[str] = None,
        max_video_duration: Optional[int] = None,
        max_file_size: Optional[int] = None,
        chunk_length_seconds: int = 120,
        initial_prompt: Optional[str] = None,
    ):
        self.model_name = model_name
        self._temp_files = []  # Track temporary files for cleanup
        
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
        self.model = WhisperModel(
            self.model_name,
            device=self.device,
            compute_type=self.compute_type,
            download_root="/tmp/models",
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
    
    def _optimize_batch_size(self) -> int:
        """Optimize batch size based on available memory."""
        if self.device == "cuda":
            # Estimate free GPU memory and adapt batch size
            free_memory_gb = torch.cuda.get_device_properties(0).total_memory / (1024 * 1024 * 1024)
            if free_memory_gb > 12:
                return 16
            elif free_memory_gb > 8:
                return 8
            else:
                return 4
        else:
            # Estimate system memory and adapt CPU batch size
            system_memory_gb = psutil.virtual_memory().total / (1024 * 1024 * 1024)
            if system_memory_gb > 16:
                return 8
            elif system_memory_gb > 8:
                return 4
            else:
                return 2

    def process_url(self, url: str) -> Dict:
        """Process a single URL and return transcription result."""
        try:
            with tempfile.TemporaryDirectory() as temp_dir:
                # Download audio and retrieve media title
                audio_path, media_title, duration = self._download_audio(url, temp_dir)

                # Transcribe
                transcription = self._transcribe(audio_path, duration)

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

    def _download_audio(self, url: str, temp_dir: str) -> Tuple[str, str, float]:
        """Download audio from URL and retrieve media title and duration."""
        ydl_opts = {
            "format": "bestaudio/best",
            "outtmpl": os.path.join(temp_dir, "%(id)s.%(ext)s"),
            "quiet": True,
            "no_warnings": True,
            "extractaudio": True,
            "postprocessors": [{
                "key": "FFmpegExtractAudio",
                "preferredcodec": "wav",
                "preferredquality": "192",
            }],
            "audioformat": "wav",
            "audioquality": 0,  # Best quality
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
                
                # Handle the case where FFmpeg has converted to WAV
                base, _ = os.path.splitext(downloaded_file)
                wav_file = f"{base}.wav"
                if os.path.exists(wav_file):
                    downloaded_file = wav_file

                # Add to temp files for cleanup
                self._temp_files.append(downloaded_file)

                # Handle file size constraints
                file_size = os.path.getsize(downloaded_file)
                if self.max_file_size and file_size > self.max_file_size:
                    os.remove(downloaded_file)  # Clean up file immediately
                    self._temp_files.remove(downloaded_file)
                    raise TranscriptionError(
                        f"Downloaded file size ({file_size} bytes) exceeds maximum allowed ({self.max_file_size} bytes)"
                    )

                return downloaded_file, media_title, duration

        except TranscriptionError:
            raise
        except Exception as e:
            raise TranscriptionError(f"Failed to download audio: {e}")

    def _should_chunk_audio(self, duration: float) -> bool:
        """Determine if audio should be chunked based on duration."""
        return duration > self.chunk_length_seconds

    def _transcribe_with_temperature_fallback(
        self, audio_path: str, segment_index: int = 0, segment_count: int = 1
    ) -> Tuple[List, Dict]:
        """Transcribe with temperature fallback for improved accuracy."""
        temperatures = [0.0, 0.2, 0.4, 0.6, 0.8]
        last_exception = None
        
        for temp in temperatures:
            try:
                progress_text = f"Transcribing segment {segment_index + 1}/{segment_count}"
                if segment_count > 1:
                    print(f"{progress_text} (temperature={temp})")
                
                return self.model.transcribe(
                    audio_path,
                    beam_size=5,  # Optimal as requested
                    temperature=temp,
                    best_of=1,
                    condition_on_previous_text=True,
                    vad_filter=True,
                    vad_parameters=dict(
                        min_silence_duration_ms=300,  # Enhanced for YouTube content
                        speech_pad_ms=100,
                        threshold=0.35,
                    ),
                    initial_prompt=self.initial_prompt,
                    language=None,  # Auto-detect language
                    task="transcribe",
                    batch_size=self.batch_size,
                )
            except Exception as e:
                last_exception = e
                print(f"Transcription failed with temperature {temp}, trying higher temperature.")
                continue
                
        raise TranscriptionError(f"All temperature attempts failed: {last_exception}")

    def _transcribe(self, audio_path: str, duration: float) -> Dict:
        """Transcribe audio file with chunking support for longer files."""
        try:
            start_time = time.time()
            text_parts = []
            language_info = None
            
            if self._should_chunk_audio(duration):
                # Chunking for long audio
                import librosa
                import soundfile as sf
                
                # Load audio for chunking
                audio, sr = librosa.load(audio_path, sr=None)
                chunk_size = self.chunk_length_seconds * sr
                chunks = math.ceil(len(audio) / chunk_size)
                
                for i in range(chunks):
                    chunk_start = i * chunk_size
                    chunk_end = min(chunk_start + chunk_size, len(audio))
                    chunk_audio = audio[chunk_start:chunk_end]
                    
                    # Save chunk to temporary file
                    chunk_path = f"{audio_path}_chunk_{i}.wav"
                    sf.write(chunk_path, chunk_audio, sr)
                    self._temp_files.append(chunk_path)
                    
                    # Transcribe chunk
                    segments, info = self._transcribe_with_temperature_fallback(
                        chunk_path, i, chunks
                    )
                    
                    # Save language info from first chunk
                    if i == 0 and info:
                        language_info = info
                    
                    # Process segments
                    chunk_segments = list(segments)
                    if chunk_segments:
                        chunk_text = " ".join(seg.text.strip() for seg in chunk_segments if seg.text.strip())
                        text_parts.append(chunk_text)
                
                if not text_parts:
                    raise TranscriptionError("No speech detected in any chunk")
            else:
                # Process short audio without chunking
                segments, info = self._transcribe_with_temperature_fallback(audio_path)
                language_info = info
                
                segments = list(segments)
                if not segments:
                    raise TranscriptionError("No speech detected")
                
                text = " ".join(seg.text.strip() for seg in segments if seg.text.strip())
                text_parts = [text]
            
            # Combine all text parts
            final_text = " ".join(text_parts)
            
            result = {
                "text": final_text,
                "model_name": self.model_name,
                "duration": time.time() - start_time,
                "error": None,
            }
            
            # Add language detection info
            if language_info:
                result["language"] = language_info.language
                result["language_probability"] = language_info.language_probability
            
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
        except Exception as e:
            print(f"Error during cleanup: {e}")
