#!/usr/bin/env python3
"""
gRPC Server for the Transcription Service

This server provides transcription services via gRPC, implementing the interface
defined in transcribe.proto. It handles video validation, transcription using Whisper,
and fetching YouTube captions.
"""

import os
import sys
import time
import json
import argparse
import logging
import signal
import concurrent.futures
from concurrent import futures
from typing import Dict, Any, Optional, Iterator
from pathlib import Path

import grpc
from dotenv import load_dotenv

# Add parent directory to path for imports
sys.path.append(str(Path(__file__).parent.parent))

# Import generated gRPC code
from grpc import transcribe_pb2
from grpc import transcribe_pb2_grpc

# Import our existing functionality
import validate
import transcription
import youtube_captions

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger("transcription_server")

class TranscriptionServicer(transcribe_pb2_grpc.TranscriptionServiceServicer):
    """Implementation of the TranscriptionService gRPC service."""
    
    def __init__(self):
        """Initialize the servicer."""
        self.executor = concurrent.futures.ThreadPoolExecutor(max_workers=4)
        logger.info("TranscriptionServicer initialized")
    
    def Validate(self, request, context):
        """Validate a video URL."""
        logger.info(f"Received validation request for URL: {request.url}")
        
        try:
            # Use existing validation logic
            result = validate.validate_video(request.url)
            
            # Convert to gRPC response
            response = transcribe_pb2.VideoInfo(
                valid=result.get("valid", False),
                duration=result.get("duration", 0.0),
                format=result.get("format", ""),
                error=result.get("error", ""),
                url=request.url
            )
            return response
            
        except Exception as e:
            logger.error(f"Error validating URL: {str(e)}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f"Internal error: {str(e)}")
            return transcribe_pb2.VideoInfo(
                valid=False,
                error=f"Internal error: {str(e)}",
                url=request.url
            )
    
    def Transcribe(self, request, context):
        """Transcribe a video and stream progress updates."""
        logger.info(f"Received transcription request for URL: {request.url}")
        
        # Convert options from gRPC map to Python dict
        options = {}
        if request.options:
            for key, value in request.options.items():
                options[key] = value
        
        try:
            # Initial progress update
            yield transcribe_pb2.TranscribeResponse(
                progress=0.0,
                status_message="Starting transcription process"
            )
            
            # Run transcription with progress updates
            last_progress = 0.0
            for progress, status in self._transcribe_with_progress(request.url, options):
                # Only send updates when progress changes significantly
                if progress - last_progress >= 0.05 or status != "processing":
                    yield transcribe_pb2.TranscribeResponse(
                        progress=progress,
                        status_message=f"Transcription {status}: {int(progress * 100)}%"
                    )
                    last_progress = progress
            
            # Get final result
            result = transcription.transcribe_url(
                request.url, 
                model=options.get("model", "large-v3-turbo"),
                chunk_length=int(options.get("chunk_length", 120)),
            )
            
            # Return final response with transcription result
            yield transcribe_pb2.TranscribeResponse(
                text=result.get("text", ""),
                model_name=result.get("model_name", ""),
                duration=result.get("duration", 0.0),
                title=result.get("title", ""),
                language=result.get("language", ""),
                language_probability=result.get("language_probability", 0.0),
                source="whisper",
                progress=1.0,
                status_message="Transcription completed"
            )
            
        except Exception as e:
            logger.error(f"Error transcribing: {str(e)}")
            yield transcribe_pb2.TranscribeResponse(
                error=f"Transcription error: {str(e)}",
                progress=1.0,
                status_message="Transcription failed"
            )
    
    def FetchYouTubeCaptions(self, request, context):
        """Fetch captions from YouTube API."""
        logger.info(f"Received YouTube captions request for video ID: {request.video_id}")
        
        try:
            # Use our existing YouTube captions client
            client = youtube_captions.YouTubeCaptionClient(request.api_key)
            result = client.get_caption_content(request.video_id)
            
            # Check for error
            if "error" in result and result["error"]:
                return transcribe_pb2.TranscribeResponse(
                    error=result["error"],
                    source="youtube_api"
                )
            
            # Success response
            return transcribe_pb2.TranscribeResponse(
                text=result.get("transcription", ""),
                title=result.get("title", ""),
                language=result.get("language", ""),
                source="youtube_api",
                progress=1.0,
                status_message="YouTube captions retrieved successfully"
            )
            
        except Exception as e:
            logger.error(f"Error fetching YouTube captions: {str(e)}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f"Internal error: {str(e)}")
            return transcribe_pb2.TranscribeResponse(
                error=f"Error fetching YouTube captions: {str(e)}",
                source="youtube_api"
            )
    
    def _transcribe_with_progress(self, url: str, options: Dict[str, str]) -> Iterator[tuple[float, str]]:
        """Generate progress updates during transcription.
        
        Args:
            url: The URL to transcribe
            options: Transcription options
            
        Yields:
            Tuples of (progress_percentage, status)
        """
        # Simulate progress for now - in a real implementation,
        # this would integrate with the actual transcription process
        start_time = time.time()
        estimated_duration = 180  # Assume ~3 minutes for transcription
        
        # Initial progress
        yield 0.0, "processing"
        
        # Download phase (0% - 20%)
        yield 0.1, "downloading"
        time.sleep(1)  # Simulated work
        yield 0.2, "processing"
        
        # Processing phase (20% - 100%)
        while True:
            elapsed = time.time() - start_time
            progress = min(elapsed / estimated_duration, 0.95)  # Cap at 95%
            
            # Move the process forward
            yield progress, "processing"
            
            if progress >= 0.95:
                break
                
            # Wait before next update
            time.sleep(3)
        
        # Final progress
        yield 1.0, "completed"


def serve(port: int, max_workers: int = 10):
    """Start the gRPC server.
    
    Args:
        port: The port to listen on
        max_workers: Maximum number of worker threads
    """
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=max_workers))
    transcribe_pb2_grpc.add_TranscriptionServiceServicer_to_server(
        TranscriptionServicer(), server
    )
    
    listen_addr = f"[::]:{port}"
    server.add_insecure_port(listen_addr)
    server.start()
    
    logger.info(f"Server started, listening on {listen_addr}")
    
    # Setup signal handlers for graceful shutdown
    def handle_shutdown(signum, frame):
        logger.info("Received shutdown signal, stopping server...")
        server.stop(grace=5)
        sys.exit(0)
    
    signal.signal(signal.SIGINT, handle_shutdown)
    signal.signal(signal.SIGTERM, handle_shutdown)
    
    # Keep thread alive
    try:
        while True:
            time.sleep(60 * 60 * 24)  # Sleep for 1 day
    except KeyboardInterrupt:
        server.stop(grace=5)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="gRPC Transcription Server")
    parser.add_argument("--port", type=int, default=50051, help="Server port")
    parser.add_argument("--workers", type=int, default=10, help="Worker threads")
    args = parser.parse_args()
    
    # Load environment variables
    load_dotenv()
    
    serve(args.port, args.workers)