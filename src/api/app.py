"""Main Litestar application."""

import logging
from contextlib import asynccontextmanager
from typing import AsyncIterator

from litestar import Litestar, Request, Response
from litestar.config.cors import CORSConfig
from litestar.config.response_cache import ResponseCacheConfig
from litestar.datastructures import State
from litestar.exceptions import HTTPException
from litestar.logging import LoggingConfig
from litestar.openapi import OpenAPIConfig
from litestar.static_files import StaticFilesConfig
from sqlalchemy.ext.asyncio import AsyncEngine, AsyncSession, create_async_engine
from sqlalchemy.orm import sessionmaker
from sqlmodel import SQLModel

from src.api.routes import transcription_router
from src.core.config import settings
from src.core.deps import dependencies
from src.core.models import ErrorResponse


# Configure logging
logging_config = LoggingConfig(
    root={"level": settings.log_level, "handlers": ["console"]},
    formatters={
        "standard": {
            "format": "%(asctime)s - %(name)s - %(levelname)s - %(message)s"
        }
    },
    log_exceptions="always",
)


async def create_db_engine() -> AsyncEngine:
    """Create async database engine."""
    connect_args = {}
    if "sqlite" in settings.database_url:
        # SQLite specific settings for better concurrency
        connect_args = {
            "check_same_thread": False,
            "timeout": 20,  # Increase timeout for busy database
        }
    
    return create_async_engine(
        settings.database_url,
        echo=settings.debug,
        pool_size=settings.database_pool_size,
        pool_timeout=settings.database_pool_timeout,
        connect_args=connect_args,
        isolation_level="READ UNCOMMITTED" if "sqlite" in settings.database_url else "READ COMMITTED",
    )


@asynccontextmanager
async def lifespan(app: Litestar) -> AsyncIterator[None]:
    """Application lifespan manager."""
    # Startup
    settings.create_directories()
    
    # Create database engine
    engine = await create_db_engine()
    
    # Create tables
    async with engine.begin() as conn:
        await conn.run_sync(SQLModel.metadata.create_all)
    
    # Create session factory
    async_session = sessionmaker(
        engine, class_=AsyncSession, expire_on_commit=False
    )
    
    # Store in app state
    app.state.db_engine = engine
    app.state.db_session = async_session
    
    logging.info("Application started")
    
    yield
    
    # Shutdown
    await engine.dispose()
    logging.info("Application shutdown")


def handle_app_exception(request: Request, exc: HTTPException) -> Response:
    """Handle application exceptions."""
    error_response = ErrorResponse.create(
        code=exc.detail or "ERROR",
        message=str(exc),
        details={"status_code": exc.status_code},
    )
    return Response(
        content=error_response.model_dump(),
        status_code=exc.status_code,
        media_type="application/json",
    )


def create_app() -> Litestar:
    """Create and configure the Litestar application."""
    # CORS configuration
    cors_config = CORSConfig(
        allow_origins=settings.cors_origins,
        allow_credentials=settings.cors_credentials,
        allow_methods=["GET", "POST", "PUT", "DELETE", "OPTIONS"],
        allow_headers=["*"],
        max_age=3600,
    )
    
    # OpenAPI configuration
    openapi_config = OpenAPIConfig(
        title="yt-text API",
        version="2.0.0",
        description="Video to text transcription service",
        path="/docs" if settings.docs_enabled else None,
    )
    
    # Static files configuration
    static_files_config = [
        # Serve static assets like CSS, JS, favicon
        StaticFilesConfig(
            directories=[settings.static_dir / "dist" / "assets"],
            path="/assets",
            html_mode=False,
        ),
        # Serve favicon and other root static files
        StaticFilesConfig(
            directories=[settings.static_dir],
            path="/static",
            html_mode=False,
        ),
    ]
    
    # Response cache configuration (optional)
    cache_config = None  # Disabled for now
    
    # Create application
    app = Litestar(
        route_handlers=transcription_router,
        dependencies=dependencies,
        lifespan=[lifespan],
        exception_handlers={
            HTTPException: handle_app_exception,
        },
        cors_config=cors_config,
        openapi_config=openapi_config,
        static_files_config=static_files_config,
        logging_config=logging_config,
        debug=settings.debug,
    )
    
    return app


# Create app instance
app = create_app()