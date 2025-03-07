#!/bin/bash
# Simple wrapper script to start the gRPC server

# Determine script directory
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
cd "$SCRIPT_DIR"

# Default port
PORT=${1:-50051}

echo "Starting gRPC server on port $PORT"
uv run scripts/grpc/start_server.py --port $PORT