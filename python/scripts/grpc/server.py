#!/usr/bin/env python3
"""
gRPC Server for the Transcription Service.

This server provides transcription services via gRPC, implementing the interface
defined in transcribe.proto. It handles video validation, transcription using Whisper,
and fetching YouTube captions.
"""

import argparse
import concurrent.futures
import logging
import signal
import sys
import time
from collections.abc import Iterator
from concurrent import futures
from pathlib import Path

import grpc
from dotenv import load_dotenv

# Add project root to path for imports
sys.path.append(str(Path(__file__).parent.parent.parent))

# Import generated gRPC code
from scripts import transcription

# Import our existing functionality
from scripts import validate
from scripts import youtube_captions
from scripts.grpc import transcribe_pb2, transcribe_pb2_grpc

# Configure logging
logging.basicConfig(
    level=logging.INFO, format="%(asctime)s - %(name)s - %(levelname)s - %(message)s"
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
            return transcribe_pb2.VideoInfo(
                valid=result.get("valid", False),
                duration=result.get("duration", 0.0),
                format=result.get("format", ""),
                error=result.get("error", ""),
                url=request.url,
            )

        except Exception as e:
            logger.exception(f"Error validating URL: {str(e)}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f"Internal error: {str(e)}")
            return transcribe_pb2.VideoInfo(
                valid=False, error=f"Internal error: {str(e)}", url=request.url
            )

    def Transcribe(self, request, context):
        """Transcribe a video and stream progress updates using optimized streaming approach."""
        logger.info(f"Received transcription request for URL: {request.url}")

        # Convert options from gRPC map to Python dict
        options = {}
        if request.options:
            for key, value in request.options.items():
                options[key] = value

        try:
            # Initial progress update
            yield transcribe_pb2.TranscribeResponse(
                progress=0.0, status_message="Starting transcription process"
            )

            # Create a thread-local cache to store the final transcription result
            # We'll use this to avoid a separate transcription call at the end
            from threading import local

            thread_data = local()
            thread_data.final_result = None

            # Cache the result in our callback
            def cache_result(result):
                thread_data.final_result = result

            # Add result callback to options
            options["result_callback"] = cache_result

            # Track if we've seen an error
            saw_error = False
            error_message = ""

            # Stream progress updates
            last_progress = 0.0
            for progress, status in self._transcribe_with_progress(request.url, options):
                # Detect error status
                if status.startswith("error:"):
                    saw_error = True
                    error_message = status[7:]  # Remove "error: " prefix

                # Only send updates when progress changes significantly or status changes
                if (
                    progress - last_progress >= 0.05
                    or status != "processing"
                    or status.startswith("error:")
                ):
                    status_message = f"Transcription {status}"
                    if not status.startswith("error:"):
                        status_message += f": {int(progress * 100)}%"

                    yield transcribe_pb2.TranscribeResponse(
                        progress=progress, status_message=status_message
                    )
                    last_progress = progress

            # Get final result
            # If we got it from our callback, use that (avoids double processing)
            if hasattr(thread_data, "final_result") and thread_data.final_result:
                result = thread_data.final_result
                logger.info("Using cached transcription result (streaming optimization)")
            elif saw_error:
                # We saw an error, don't try to transcribe again
                result = {
                    "text": "",
                    "model_name": options.get("model", "large-v3-turbo"),
                    "duration": 0.0,
                    "title": "",
                    "language": "",
                    "language_probability": 0.0,
                    "error": error_message or "Transcription failed",
                }
                logger.error(f"Transcription error: {error_message}")
            else:
                # We need to do a separate call because our streaming didn't cache the result
                # This is a fallback and should rarely happen with proper implementation
                logger.warning(
                    "Falling back to separate transcription call - suboptimal performance"
                )
                result = transcription.Transcriber(
                    model_name=options.get("model", "large-v3-turbo"),
                    chunk_length_seconds=int(options.get("chunk_length", 120)),
                ).process_url(request.url)

            # Check for errors
            if saw_error or result.get("error"):
                yield transcribe_pb2.TranscribeResponse(
                    error=result.get("error", error_message) or "Unknown transcription error",
                    progress=1.0,
                    status_message="Transcription failed",
                )
            else:
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
                    status_message="Transcription completed",
                )

        except Exception as e:
            logger.exception(f"Error transcribing: {str(e)}")
            import traceback

            logger.exception(traceback.format_exc())
            yield transcribe_pb2.TranscribeResponse(
                error=f"Transcription error: {str(e)}",
                progress=1.0,
                status_message="Transcription failed",
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
                    error=result["error"], source="youtube_api"
                )

            # Success response
            return transcribe_pb2.TranscribeResponse(
                text=result.get("transcription", ""),
                title=result.get("title", ""),
                language=result.get("language", ""),
                source="youtube_api",
                progress=1.0,
                status_message="YouTube captions retrieved successfully",
            )

        except Exception as e:
            logger.exception(f"Error fetching YouTube captions: {str(e)}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f"Internal error: {str(e)}")
            return transcribe_pb2.TranscribeResponse(
                error=f"Error fetching YouTube captions: {str(e)}", source="youtube_api"
            )

    def _transcribe_with_progress(
        self, url: str, options: dict[str, str]
    ) -> Iterator[tuple[float, str]]:
        """Generate progress updates during transcription using real-time events.

        Args:
            url: The URL to transcribe
            options: Transcription options

        Yields:
            Tuples of (progress_percentage, status)
        """
        import queue
        import threading

        # Create a thread-safe queue for progress updates
        progress_queue = queue.Queue()

        # Track the different states of transcription
        class TranscriptionProgress:
            def __init__(self):
                self.download_started = False
                self.download_finished = False
                self.transcription_started = False
                self.download_progress = 0.0
                self.transcription_progress = 0.0
                self.chunk_count = 0
                self.chunks_processed = 0
                self.error = None
                self.status = "initializing"
                self.completed = False
                self.lock = threading.Lock()

            def update_download(self, progress: float, status: str = "downloading"):
                with self.lock:
                    self.download_started = True
                    self.download_progress = progress
                    if progress >= 0.99:
                        self.download_finished = True
                    self.status = status
                    # Download is 20% of the total progress
                    total_progress = min(0.2 * progress, 0.2)
                    progress_queue.put((total_progress, status))

            def update_transcription(self, progress: float):
                with self.lock:
                    if not self.transcription_started:
                        self.transcription_started = True
                    self.transcription_progress = progress
                    self.status = "processing"
                    # Transcription is 80% of the total progress (after 20% download)
                    total_progress = 0.2 + (0.8 * progress)
                    progress_queue.put((total_progress, self.status))

            def set_chunk_info(self, chunk_count: int):
                with self.lock:
                    self.chunk_count = chunk_count

            def chunk_processed(self):
                with self.lock:
                    self.chunks_processed += 1
                    if self.chunk_count > 0:
                        progress = self.chunks_processed / self.chunk_count
                        self.update_transcription(progress)

            def complete(self):
                with self.lock:
                    self.completed = True
                    progress_queue.put((1.0, "completed"))

            def set_error(self, error_message: str):
                with self.lock:
                    self.error = error_message
                    progress_queue.put((1.0, f"error: {error_message}"))

        # Create progress tracker
        progress_tracker = TranscriptionProgress()

        # Define monitoring callbacks
        def download_progress_callback(progress):
            if progress["status"] == "downloading":
                # Extract download progress if available
                if "downloaded_bytes" in progress and "total_bytes" in progress:
                    if progress["total_bytes"] > 0:
                        percent = progress["downloaded_bytes"] / progress["total_bytes"]
                        progress_tracker.update_download(percent)
                elif "downloaded_bytes" in progress:
                    # We don't know the total, but we can show that something is happening
                    progress_tracker.update_download(0.5)
            elif progress["status"] == "finished":
                progress_tracker.update_download(1.0, "processing")
            elif progress["status"] == "error":
                progress_tracker.set_error(
                    f"Download error: {progress.get('error', 'unknown error')}"
                )

        def chunk_progress_callback(i, total):
            if total > 0:
                progress_tracker.set_chunk_info(total)
                progress_tracker.chunk_processed()

        # Start transcription in a separate thread
        transcription_thread = threading.Thread(
            target=self._run_transcription_thread,
            args=(
                url,
                options,
                progress_tracker,
                download_progress_callback,
                chunk_progress_callback,
            ),
        )
        transcription_thread.daemon = True  # Allow thread to exit when main thread exits
        transcription_thread.start()

        # Initial progress
        yield 0.0, "initializing"

        # Continue yielding progress updates until transcription completes or fails
        timeout_seconds = int(options.get("timeout", 600))  # Default 10 min timeout
        start_time = time.time()

        try:
            while not progress_tracker.completed and not progress_tracker.error:
                try:
                    # Get progress from queue with timeout
                    progress, status = progress_queue.get(timeout=3)
                    yield progress, status

                    # Check for timeout
                    if time.time() - start_time > timeout_seconds:
                        progress_tracker.set_error("Transcription timed out")
                        yield 1.0, "error: Transcription timed out"
                        break

                except queue.Empty:
                    # No progress update in the timeout period
                    if not transcription_thread.is_alive():
                        # Thread died without completing or setting error
                        if not progress_tracker.completed:
                            yield 1.0, "error: Transcription thread terminated unexpectedly"
                            break

                    # Thread is still running but no updates, just wait
                    continue

            # If we exited due to error
            if progress_tracker.error:
                yield 1.0, f"error: {progress_tracker.error}"

        except Exception as e:
            # Handle any unexpected exceptions in the progress reporting
            yield 1.0, f"error: Error in progress reporting: {str(e)}"

        finally:
            # Ensure we report completion
            if not progress_tracker.completed and not progress_tracker.error:
                yield 1.0, "completed"

    def _run_transcription_thread(
        self,
        url: str,
        options: dict[str, str],
        progress_tracker,
        download_callback,
        chunk_callback,
    ):
        """Run transcription in a separate thread with progress tracking and result caching.

        Args:
            url: URL to transcribe
            options: Transcription options dictionary that may contain:
                - model: Whisper model name (default: large-v3-turbo)
                - chunk_length: Length of audio chunks in seconds (default: 120)
                - prompt: Initial prompt for transcription
                - result_callback: Optional callback to receive the final result
            progress_tracker: Progress tracking object
            download_callback: Callback for download progress
            chunk_callback: Callback for chunk processing
        """
        result = None
        try:
            # Set up transcriber with progress monitoring
            model = options.get("model", "large-v3-turbo")
            chunk_length = int(options.get("chunk_length", 120))

            # Extract result callback if present
            result_callback = options.get("result_callback")

            # Monitor download progress
            transcriber = transcription.Transcriber(
                model_name=model,
                chunk_length_seconds=chunk_length,
                initial_prompt=options.get("prompt"),
            )

            # Patch the _monitor_download_progress method to call our callback
            original_monitor = transcriber._monitor_download_progress

            def wrapped_monitor(progress):
                download_callback(progress)
                return original_monitor(progress)

            transcriber._monitor_download_progress = wrapped_monitor

            # Patch the transcribe method to monitor chunk progress
            original_transcribe = transcriber._transcribe

            def wrapped_transcribe(audio_path, duration):
                # Set up a progress callback for chunks
                def chunk_processor_injector(cls):
                    original_enter = cls.__enter__

                    def wrapped_enter(self):
                        result = original_enter(self)
                        # Inject our monitoring code
                        if hasattr(self, "segment") and self.segment:
                            chunk_length_ms = transcriber.chunk_length_seconds * 1000
                            chunk_count = math.ceil(len(self.segment) / chunk_length_ms)
                            progress_tracker.set_chunk_info(chunk_count)
                        return result

                    cls.__enter__ = wrapped_enter
                    return cls

                # Apply our wrapper
                if "_should_chunk_audio" in dir(transcriber) and transcriber._should_chunk_audio(
                    duration
                ):
                    # We know we'll be in chunking mode, so we can inject our monitoring
                    # This requires modifying the code at runtime, which is advanced
                    from types import MethodType

                    # Create a new version of the original method that calls our callback
                    def process_chunk_with_progress(self, *args, **kwargs):
                        # Call the original method
                        result = original_transcribe(*args, **kwargs)

                        # Update progress after each chunk
                        progress_tracker.chunk_processed()

                        return result

                    # Replace the original method with our version
                    transcriber._transcribe = MethodType(process_chunk_with_progress, transcriber)

                # Call original implementation
                return original_transcribe(audio_path, duration)

            transcriber._transcribe = wrapped_transcribe

            # Run the transcription
            logger.info(f"Starting streaming transcription for URL: {url}")
            result = transcriber.process_url(url)
            logger.info(f"Completed transcription for URL: {url}")

            # Store the result in the callback if provided
            if result_callback and callable(result_callback):
                logger.info("Storing transcription result in callback")
                result_callback(result)

            # Cleanup
            transcriber.close()

            # Check for errors in the result
            if result.get("error"):
                logger.error(f"Transcription error in result: {result.get('error')}")
                progress_tracker.set_error(result["error"])
            else:
                # Success
                logger.info(f"Transcription successful, text length: {len(result.get('text', ''))}")
                progress_tracker.complete()

        except Exception as e:
            import traceback

            error_details = traceback.format_exc()
            logger.exception(f"Exception in transcription thread: {e}\n{error_details}")
            progress_tracker.set_error(f"Transcription failed: {str(e)}\n{error_details}")

            # Still try to call the callback with error
            if "result_callback" in options and callable(options["result_callback"]):
                error_result = {
                    "text": "",
                    "model_name": options.get("model", "large-v3-turbo"),
                    "duration": 0.0,
                    "error": str(e),
                    "title": "",
                    "url": url,
                    "language": "",
                    "language_probability": 0.0,
                }
                options["result_callback"](error_result)

        # Final cleanup (in both success and error cases)
        if "transcriber" in locals():
            try:
                transcriber.close()
            except Exception as e:
                logger.warning(f"Error during final cleanup: {e}")
                pass  # Best effort cleanup

        # Return result for future use
        return result


def serve(port: int, max_workers: int = 10):
    """Start the gRPC server.

    Args:
        port: The port to listen on
        max_workers: Maximum number of worker threads
    """
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=max_workers))
    transcribe_pb2_grpc.add_TranscriptionServiceServicer_to_server(TranscriptionServicer(), server)

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
