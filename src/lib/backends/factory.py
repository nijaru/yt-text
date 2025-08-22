"""Factory for creating and managing transcription backends."""

from typing import Optional

from src.core.config import settings
from src.lib.backends.base import TranscriptionBackend


class BackendFactory:
    """Factory for creating and managing transcription backends."""

    def __init__(self):
        self._backends: list[TranscriptionBackend] = []
        self._initialized = False

    async def initialize(self) -> None:
        """Initialize all available backends."""
        if self._initialized:
            return

        # Import backends dynamically to avoid import errors
        backend_classes = []

        # Try to import and register backends based on config
        for backend_name in settings.transcription_backends:
            try:
                if backend_name == "whisper_cpp":
                    from src.lib.backends.whisper_cpp import WhisperCPPBackend
                    backend_classes.append(WhisperCPPBackend)
                elif backend_name == "mlx" and self._is_mlx_available():
                    from src.lib.backends.mlx_whisper import MLXWhisperBackend
                    backend_classes.append(MLXWhisperBackend)
                elif backend_name == "openai" and settings.openai_api_key:
                    from src.lib.backends.openai_api import OpenAIBackend
                    backend_classes.append(OpenAIBackend)
                elif backend_name == "fallback":
                    from src.lib.backends.fallback_whisper import FallbackWhisperBackend
                    backend_classes.append(FallbackWhisperBackend)
            except ImportError:
                # Backend not available, skip
                continue

        # Initialize backends
        for backend_class in backend_classes:
            try:
                backend = backend_class()
                if await backend.is_available():
                    self._backends.append(backend)
            except Exception:
                # Backend failed to initialize, skip
                continue

        # Sort by priority
        self._backends.sort(key=lambda b: b.get_priority())
        self._initialized = True

    async def get_best_backend(self) -> Optional[TranscriptionBackend]:
        """Get the best available backend."""
        if not self._initialized:
            await self.initialize()

        for backend in self._backends:
            if await backend.is_available():
                return backend
        
        return None

    async def get_backend_by_name(self, name: str) -> Optional[TranscriptionBackend]:
        """Get a specific backend by name."""
        if not self._initialized:
            await self.initialize()

        for backend in self._backends:
            if backend.get_name() == name and await backend.is_available():
                return backend
        
        return None

    async def get_available_backends(self) -> list[TranscriptionBackend]:
        """Get all available backends."""
        if not self._initialized:
            await self.initialize()

        available = []
        for backend in self._backends:
            if await backend.is_available():
                available.append(backend)
        
        return available

    async def get_backend_for_model(self, model: str) -> Optional[TranscriptionBackend]:
        """Get the best backend that supports the specified model."""
        if not self._initialized:
            await self.initialize()

        for backend in self._backends:
            if (await backend.is_available() and 
                backend.supports_model(model)):
                return backend
        
        return None

    def _is_mlx_available(self) -> bool:
        """Check if MLX is available (Apple Silicon only)."""
        try:
            import platform
            return platform.machine() == "arm64" and platform.system() == "Darwin"
        except Exception:
            return False

    async def cleanup_all(self) -> None:
        """Clean up all backends."""
        for backend in self._backends:
            try:
                await backend.cleanup()
            except Exception:
                pass

    def get_backend_info(self) -> dict[str, dict]:
        """Get information about all backends."""
        info = {}
        for backend in self._backends:
            info[backend.get_name()] = {
                "priority": backend.get_priority(),
                "supported_models": backend.get_supported_models(),
                "supported_languages": backend.get_supported_languages(),
            }
        return info