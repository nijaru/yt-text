"""CLI script for standalone transcription."""

import asyncio
import sys
from pathlib import Path

import click
from sqlalchemy.ext.asyncio import AsyncSession, create_async_engine
from sqlalchemy.orm import sessionmaker
from sqlmodel import SQLModel

from src.core.config import settings
from src.lib.backends.factory import BackendFactory
from src.services.cache import CacheService
from src.services.download import DownloadService
from src.services.transcription import TranscriptionService


async def setup_database():
    """Set up database for CLI usage."""
    engine = create_async_engine(
        settings.database_url,
        echo=False,
    )
    
    # Create tables
    async with engine.begin() as conn:
        await conn.run_sync(SQLModel.metadata.create_all)
    
    # Create session factory
    async_session = sessionmaker(
        engine, class_=AsyncSession, expire_on_commit=False
    )
    
    return engine, async_session


async def transcribe_url(url: str, model: str = "base", language: str = None):
    """Transcribe a single URL."""
    
    # Set up database
    engine, async_session = await setup_database()
    
    try:
        # Create services
        download_service = DownloadService()
        cache_service = CacheService()
        backend_factory = BackendFactory()
        await backend_factory.initialize()
        
        async with async_session() as session:
            transcription_service = TranscriptionService(
                db_session=session,
                download_service=download_service,
                cache_service=cache_service,
                backend_factory=backend_factory,
            )
            
            click.echo(f"Starting transcription of: {url}")
            click.echo(f"Model: {model}")
            if language:
                click.echo(f"Language: {language}")
            
            # Create job without starting background processing
            job = await transcription_service.create_job(
                url=url,
                model=model,
                language=language,
                start_processing=False,
            )
            
            click.echo(f"Job created: {job.id}")
            
            if job.status == "completed":
                # Cached result
                click.echo("✅ Found cached result")
                click.echo(f"Title: {job.title or 'Unknown'}")
                if job.duration:
                    click.echo(f"Duration: {job.duration}s")
                click.echo(f"Model used: {job.model_used}")
                click.echo(f"Language: {job.detected_language or 'unknown'}")
                click.echo(f"Word count: {job.word_count}")
                click.echo("\n--- Transcription ---")
                click.echo(job.text)
                return
            
            # Wait for processing
            click.echo("Processing...")
            
            # For CLI, process the job directly rather than background processing
            # This avoids event loop issues with background tasks
            await transcription_service._process_job(job.id)
            
            # Get final job status
            final_job = await transcription_service.get_job(job.id)
            
            if final_job.status == "completed":
                click.echo("✅ Transcription completed!")
                click.echo(f"Title: {final_job.title or 'Unknown'}")
                if final_job.duration:
                    click.echo(f"Duration: {final_job.duration}s")
                click.echo(f"Model used: {final_job.model_used}")
                click.echo(f"Language: {final_job.detected_language or 'unknown'}")
                click.echo(f"Word count: {final_job.word_count}")
                click.echo(f"Processing time: {final_job.processing_time_ms}ms")
                click.echo("\n--- Transcription ---")
                click.echo(final_job.text)
            elif final_job.status == "failed":
                click.echo(f"❌ Transcription failed: {final_job.error}")
                sys.exit(1)
                    
    finally:
        await engine.dispose()


@click.command()
@click.argument('url')
@click.option('--model', '-m', default='base', help='Whisper model to use')
@click.option('--language', '-l', help='Language code (e.g., en, es, fr)')
@click.option('--output', '-o', help='Output file (optional)')
def main(url: str, model: str, language: str, output: str):
    """Transcribe video from URL using yt-dlp and Whisper."""
    
    # Create temp and data directories
    settings.create_directories()
    
    try:
        result = asyncio.run(transcribe_url(url, model, language))
        
        if output and result:
            output_path = Path(output)
            output_path.write_text(result, encoding='utf-8')
            click.echo(f"\nTranscription saved to: {output_path}")
            
    except KeyboardInterrupt:
        click.echo("\nTranscription cancelled")
        sys.exit(1)
    except Exception as e:
        click.echo(f"Error: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()