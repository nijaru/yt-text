#!/usr/bin/env python3
"""
Start the gRPC server for transcription services.

This script is used to start the server with command-line arguments to configure
the port, worker count, and logging level.
"""

import argparse
import logging
import os
import sys
from pathlib import Path

# Add project root to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent.parent))

# Import the server module
from scripts.grpc.server import serve

# Configure argument parser
parser = argparse.ArgumentParser(description="Start the transcription gRPC server")
parser.add_argument("--port", type=int, default=50051, help="Port to listen on")
parser.add_argument("--workers", type=int, default=10, help="Number of worker threads")
parser.add_argument(
    "--log-level",
    choices=["DEBUG", "INFO", "WARNING", "ERROR"],
    default="INFO",
    help="Logging level",
)
parser.add_argument("--env-file", type=str, help="Path to .env file")


def main():
    # Parse command-line arguments
    args = parser.parse_args()

    # Configure logging
    logging.basicConfig(
        level=getattr(logging, args.log_level),
        format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    )

    # Load environment file if specified
    if args.env_file:
        from dotenv import load_dotenv

        if os.path.exists(args.env_file):
            logging.info(f"Loading environment from {args.env_file}")
            load_dotenv(args.env_file)
        else:
            logging.warning(f"Environment file not found: {args.env_file}")

    # Log configuration
    logging.info(f"Starting gRPC server on port {args.port} with {args.workers} workers")

    # Start the server
    serve(args.port, args.workers)


if __name__ == "__main__":
    main()
