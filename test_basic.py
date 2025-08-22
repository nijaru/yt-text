"""Basic functionality test."""

import asyncio
import sys
import tempfile
from pathlib import Path

from src.core.config import settings
from src.lib.backends.factory import BackendFactory
from src.services.download import DownloadService


async def test_backends():
    """Test available transcription backends."""
    print("Testing transcription backends...")
    
    factory = BackendFactory()
    await factory.initialize()
    
    backends = await factory.get_available_backends()
    
    if not backends:
        print("❌ No transcription backends available")
        print("Available backends to install:")
        print("- pip install openai-whisper  # Fallback backend")
        print("- Install whisper.cpp  # High performance")
        print("- pip install mlx-whisper  # Apple Silicon only")
        return False
    
    print(f"✅ Found {len(backends)} available backend(s):")
    for backend in backends:
        print(f"  - {backend.get_name()} (priority: {backend.get_priority()})")
    
    return True


async def test_download():
    """Test download functionality."""
    print("\nTesting download functionality...")
    
    download_service = DownloadService()
    
    # Test URL validation
    test_urls = [
        "https://www.youtube.com/watch?v=dQw4w9WgXcQ",  # Valid
        "https://example.com/video.mp4",  # Might work
        "invalid_url",  # Invalid
        "ftp://example.com/file.mp4",  # Invalid scheme
    ]
    
    valid_count = 0
    for url in test_urls:
        is_valid = download_service.is_supported_url(url)
        status = "✅ Valid" if is_valid else "❌ Invalid"
        print(f"  {status}: {url}")
        if is_valid:
            valid_count += 1
    
    print(f"URL validation working: {valid_count}/{len(test_urls)} tests passed")
    return valid_count > 0


async def test_config():
    """Test configuration."""
    print("\nTesting configuration...")
    
    print(f"Environment: {settings.env}")
    print(f"Database URL: {settings.database_url}")
    print(f"Temp directory: {settings.temp_dir}")
    print(f"Cache enabled: {settings.cache_enabled}")
    print(f"Transcription backends: {settings.transcription_backends}")
    
    # Test directory creation
    try:
        settings.create_directories()
        print("✅ Directory creation successful")
        return True
    except Exception as e:
        print(f"❌ Directory creation failed: {e}")
        return False


async def main():
    """Run basic tests."""
    print("yt-text Basic Functionality Test")
    print("=" * 40)
    
    # Test configuration
    config_ok = await test_config()
    
    # Test backends
    backends_ok = await test_backends()
    
    # Test download service
    download_ok = await test_download()
    
    print("\n" + "=" * 40)
    print("Test Summary:")
    print(f"Config: {'✅' if config_ok else '❌'}")
    print(f"Backends: {'✅' if backends_ok else '❌'}")
    print(f"Download: {'✅' if download_ok else '❌'}")
    
    if all([config_ok, backends_ok, download_ok]):
        print("\n🎉 Basic functionality test PASSED")
        print("\nYou can now:")
        print("1. Run the web server: uv run dev")
        print("2. Use CLI: uv run transcribe <youtube-url>")
        return 0
    else:
        print("\n❌ Basic functionality test FAILED")
        print("\nPlease install missing dependencies:")
        if not backends_ok:
            print("- pip install openai-whisper")
        return 1


if __name__ == "__main__":
    sys.exit(asyncio.run(main()))