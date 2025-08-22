"""Configuration management using Pydantic settings."""

from pathlib import Path
from typing import Literal

from pydantic import Field, computed_field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Application settings with environment variable support."""

    model_config = SettingsConfigDict(
        env_file=".env",
        env_prefix="APP_",
        case_sensitive=False,
    )

    # API Server
    host: str = "0.0.0.0"
    port: int = 8000
    workers: int = 1
    env: Literal["production", "development", "testing"] = "development"
    debug: bool = False
    docs_enabled: bool = True

    # Database
    database_url: str = "sqlite+aiosqlite:///data/db.sqlite"
    database_pool_size: int = 5
    database_pool_timeout: int = 30

    # Paths
    data_dir: Path = Path("/app/data")
    temp_dir: Path = Path("/tmp/yt-text")
    model_dir: Path = Path("/app/models")
    static_dir: Path = Path("static")

    # Transcription
    whisper_model: str = "base"
    whisper_language: str = "auto"
    whisper_device: Literal["auto", "cpu", "cuda", "mps"] = "auto"
    transcription_backends: list[str] = Field(
        default_factory=lambda: ["whisper_cpp", "mlx", "openai"]
    )

    # OpenAI Fallback
    openai_api_key: str = ""
    openai_daily_limit: int = 100
    openai_model: str = "whisper-1"

    # Performance
    rate_limit_enabled: bool = True
    rate_limit_rpm: int = 10
    rate_limit_daily: int = 1000
    request_timeout: int = 30
    download_timeout: int = 300
    transcription_timeout: int = 1800
    cleanup_interval: int = 3600
    max_video_duration: int = 14400  # 4 hours
    max_file_size: int = 2_147_483_648  # 2GB
    max_concurrent_jobs: int = 3

    # Cache
    cache_enabled: bool = True
    cache_dir: Path = Path("/app/cache")
    cache_size_limit: int = 1_073_741_824  # 1GB
    cache_ttl: int = 604_800  # 1 week

    # Security
    cors_origins: list[str] = Field(default_factory=lambda: ["*"])
    cors_credentials: bool = True
    security_headers: bool = True
    trusted_hosts: list[str] = Field(default_factory=lambda: ["*"])
    url_allowlist: list[str] = Field(default_factory=list)
    url_blocklist: list[str] = Field(default_factory=list)

    # Logging
    log_level: Literal["DEBUG", "INFO", "WARNING", "ERROR"] = "INFO"
    log_format: Literal["json", "console"] = "console"
    log_file: Path = Path("/app/logs/app.log")
    log_max_size: int = 10_485_760  # 10MB
    log_backup_count: int = 5

    @computed_field
    @property
    def is_production(self) -> bool:
        """Check if running in production mode."""
        return self.env == "production"

    @computed_field
    @property
    def is_development(self) -> bool:
        """Check if running in development mode."""
        return self.env == "development"

    def create_directories(self) -> None:
        """Create necessary directories if they don't exist."""
        for directory in [self.data_dir, self.temp_dir, self.model_dir, self.cache_dir]:
            directory.mkdir(parents=True, exist_ok=True)
        
        if self.log_file:
            self.log_file.parent.mkdir(parents=True, exist_ok=True)


# Global settings instance
settings = Settings()