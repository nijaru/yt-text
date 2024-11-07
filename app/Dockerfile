# Use a multi-stage build to keep the final image size small
# Stage 1: Build stage
FROM golang:1.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o main .

# Stage 2: Final stage
FROM python:3.12-slim-bookworm

# Install system dependencies
RUN apt-get update && apt-get install -y \
    ffmpeg \
    git \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Install uv
COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /bin/

# Set the working directory
WORKDIR /app

# Set up the virtual environment and install Python dependencies
COPY pyproject.toml ./
RUN uv sync

# Copy the built Go binary from the builder stage
COPY --from=builder /app/main /usr/local/bin/main

# Copy static files
COPY static /app/static

# Copy scripts
COPY scripts /app/scripts
RUN chmod +x /app/scripts/*.py

# Expose the port your application runs on
EXPOSE 8080

# Set the entrypoint
ENTRYPOINT ["/usr/local/bin/main"]
