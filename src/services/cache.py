"""Cache service for transcription results."""

import hashlib
import json
from typing import Any, Optional

import diskcache

from src.core.config import settings


class CacheService:
    """Service for caching transcription results."""

    def __init__(self):
        if settings.cache_enabled:
            self.cache = diskcache.Cache(
                directory=str(settings.cache_dir),
                size_limit=settings.cache_size_limit,
                eviction_policy='least-recently-used',
            )
        else:
            self.cache = None

    def _generate_cache_key(self, url: str, model: str) -> str:
        """Generate a cache key for URL and model combination."""
        content = f"{url}:{model}"
        return hashlib.sha256(content.encode()).hexdigest()

    async def get_transcription(
        self, url: str, model: str
    ) -> Optional[dict[str, Any]]:
        """Get cached transcription result."""
        if not self.cache:
            return None

        try:
            key = self._generate_cache_key(url, model)
            result = self.cache.get(key)
            
            if result:
                # Verify the result has required fields
                if isinstance(result, dict) and 'text' in result:
                    return result
                else:
                    # Invalid cache entry, remove it
                    self.cache.delete(key)
                    
        except Exception:
            # Cache error - return None and continue
            pass
            
        return None

    async def store_transcription(
        self, url: str, model: str, result: dict[str, Any]
    ) -> None:
        """Store transcription result in cache."""
        if not self.cache:
            return

        try:
            key = self._generate_cache_key(url, model)
            
            # Ensure result has required fields
            if 'text' not in result:
                return
                
            # Add metadata
            cache_entry = {
                **result,
                'cached_at': str(int(time.time())),
                'url': url,
                'model': model,
            }
            
            # Store with TTL
            self.cache.set(
                key, 
                cache_entry, 
                expire=settings.cache_ttl
            )
            
        except Exception:
            # Cache error - log but don't fail
            pass

    async def invalidate_url(self, url: str) -> None:
        """Invalidate all cached results for a URL."""
        if not self.cache:
            return

        try:
            # We need to iterate through keys to find matches
            # This is expensive but rarely used
            keys_to_delete = []
            for key in self.cache:
                try:
                    value = self.cache.get(key)
                    if isinstance(value, dict) and value.get('url') == url:
                        keys_to_delete.append(key)
                except Exception:
                    continue
                    
            for key in keys_to_delete:
                self.cache.delete(key)
                
        except Exception:
            pass

    async def get_cache_stats(self) -> dict[str, Any]:
        """Get cache statistics."""
        if not self.cache:
            return {
                'enabled': False,
                'size': 0,
                'volume': 0,
                'hits': 0,
                'misses': 0,
            }

        try:
            stats = self.cache.stats(enable=True)
            return {
                'enabled': True,
                'size': len(self.cache),
                'volume': self.cache.volume(),
                'hits': stats.get('cache_hits', 0),
                'misses': stats.get('cache_misses', 0),
            }
        except Exception:
            return {
                'enabled': True,
                'size': 0,
                'volume': 0,
                'hits': 0,
                'misses': 0,
            }

    async def clear_cache(self) -> None:
        """Clear all cached data."""
        if not self.cache:
            return

        try:
            self.cache.clear()
        except Exception:
            pass

    def __del__(self):
        """Clean up cache on deletion."""
        if hasattr(self, 'cache') and self.cache:
            try:
                self.cache.close()
            except Exception:
                pass


# Import time module for timestamps
import time