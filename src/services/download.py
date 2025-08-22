"""Download service using yt-dlp."""

import asyncio
import json
from pathlib import Path
from typing import Any, Callable, Optional
from urllib.parse import urlparse

import yt_dlp

from src.core.config import settings


class DownloadService:
    """Service for downloading audio from videos using yt-dlp."""

    def __init__(self):
        self.temp_dir = settings.temp_dir
        self.temp_dir.mkdir(parents=True, exist_ok=True)

    def is_supported_url(self, url: str) -> bool:
        """Check if URL is supported by yt-dlp."""
        try:
            parsed = urlparse(url)
            if not parsed.scheme or not parsed.netloc:
                return False
            
            # Quick validation - yt-dlp supports many sites
            # We could add a blocklist here if needed
            if any(domain in parsed.netloc.lower() for domain in settings.url_blocklist):
                return False
                
            if settings.url_allowlist and not any(
                domain in parsed.netloc.lower() for domain in settings.url_allowlist
            ):
                return False
                
            return True
        except Exception:
            return False

    async def download_audio(
        self, 
        url: str, 
        progress_callback: Optional[Callable[[int], None]] = None
    ) -> dict[str, Any]:
        """Download audio from URL and return metadata."""
        
        def progress_hook(d: dict) -> None:
            if progress_callback and d['status'] == 'downloading':
                if 'total_bytes' in d and 'downloaded_bytes' in d:
                    progress = int((d['downloaded_bytes'] / d['total_bytes']) * 100)
                    progress_callback(progress)

        # Configure yt-dlp options
        ydl_opts = {
            'format': 'bestaudio/best',
            'outtmpl': str(self.temp_dir / '%(title)s.%(ext)s'),
            'postprocessors': [{
                'key': 'FFmpegExtractAudio',
                'preferredcodec': 'wav',
                'preferredquality': '16000',  # 16kHz for Whisper
            }],
            'postprocessor_args': [
                '-ac', '1',  # Mono
                '-ar', '16000',  # 16kHz sample rate
            ],
            'quiet': True,
            'no_warnings': True,
            'extractaudio': True,
            'audioformat': 'wav',
            'progress_hooks': [progress_hook] if progress_callback else [],
        }

        # Add duration and file size limits
        if settings.max_video_duration:
            ydl_opts['match_filter'] = self._duration_filter

        try:
            # Run yt-dlp in thread pool to avoid blocking
            loop = asyncio.get_event_loop()
            result = await loop.run_in_executor(
                None, self._download_with_ydl, url, ydl_opts
            )
            return result
            
        except Exception as e:
            raise RuntimeError(f"Download failed: {str(e)}")

    def _download_with_ydl(self, url: str, ydl_opts: dict) -> dict[str, Any]:
        """Run yt-dlp download in sync context."""
        with yt_dlp.YoutubeDL(ydl_opts) as ydl:
            # First extract info without downloading
            info = ydl.extract_info(url, download=False)
            
            # Check file size if available
            if settings.max_file_size and 'filesize' in info:
                if info['filesize'] > settings.max_file_size:
                    raise ValueError(f"File too large: {info['filesize']} bytes")
            
            # Check duration
            duration = info.get('duration')
            if duration and settings.max_video_duration:
                if duration > settings.max_video_duration:
                    raise ValueError(f"Video too long: {duration} seconds")
            
            # Now download
            ydl.download([url])
            
            # Find the downloaded file
            audio_path = None
            for file_path in self.temp_dir.glob(f"{info['title'][:50]}*.wav"):
                audio_path = file_path
                break
                
            if not audio_path or not audio_path.exists():
                raise RuntimeError("Downloaded audio file not found")
                
            return {
                'audio_path': audio_path,
                'title': info.get('title', 'Unknown'),
                'duration': duration,
                'url': url,
                'extractor': info.get('extractor'),
                'file_size': audio_path.stat().st_size if audio_path.exists() else None,
            }

    def _duration_filter(self, info_dict: dict) -> Optional[str]:
        """Filter videos by duration."""
        duration = info_dict.get('duration')
        if duration and duration > settings.max_video_duration:
            return f"Video duration {duration}s exceeds limit of {settings.max_video_duration}s"
        return None

    async def cleanup_file(self, file_path: Path) -> None:
        """Clean up downloaded file."""
        try:
            if file_path and file_path.exists():
                file_path.unlink()
        except Exception:
            # Log but don't fail
            pass

    async def get_video_info(self, url: str) -> dict[str, Any]:
        """Get video metadata without downloading."""
        ydl_opts = {
            'quiet': True,
            'no_warnings': True,
        }
        
        try:
            loop = asyncio.get_event_loop()
            return await loop.run_in_executor(
                None, self._extract_info, url, ydl_opts
            )
        except Exception as e:
            raise RuntimeError(f"Failed to extract video info: {str(e)}")

    def _extract_info(self, url: str, ydl_opts: dict) -> dict[str, Any]:
        """Extract video info in sync context."""
        with yt_dlp.YoutubeDL(ydl_opts) as ydl:
            info = ydl.extract_info(url, download=False)
            return {
                'title': info.get('title', 'Unknown'),
                'duration': info.get('duration'),
                'description': info.get('description', ''),
                'uploader': info.get('uploader', ''),
                'upload_date': info.get('upload_date'),
                'view_count': info.get('view_count'),
                'extractor': info.get('extractor'),
                'webpage_url': info.get('webpage_url', url),
            }