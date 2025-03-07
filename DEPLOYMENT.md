# Deployment Guide

This document provides information on deploying the yt-text application to production environments.

## Manual Deployment

The application can be deployed to Fly.io:

1. Install the Fly CLI tools:

   ```sh
   curl -L https://fly.io/install.sh | sh
   # or
   brew install flyctl
   ```

2. Login to Fly.io:

   ```sh
   fly auth login
   ```

3. Deploy using the provided configuration:

   ```sh
   make deploy
   # or
   fly deploy --config docker/fly/fly.toml
   ```

4. For more details on deployment configuration, see the files in `docker/fly/`

## CI/CD Pipeline

This project uses GitHub Actions for automated testing and deployment:

1. **Continuous Integration** - Runs tests, linting, and security scans on all commits and PRs
   - Go and Python code linting and testing
   - Security scanning for both languages
   - Docker image building and vulnerability scanning

2. **Continuous Deployment** - Automatically deploys to Fly.io when CI succeeds on the main branch
   - Triggered after successful CI runs on the main branch
   - Deploys the application using the Fly.io CLI

The CI/CD workflow includes:
- Running Go tests with race detection
- Linting Python code with ruff
- Building Docker images
- Scanning dependencies for vulnerabilities
- Deploying to Fly.io when merged to main

## Deployment Commands

### Deployment
- Deploy to Fly.io: `make deploy` or `fly deploy --config docker/fly/fly.toml`
- Launch a local production server (for testing): `make docker-prod` or `docker-compose -f docker/fly/docker-compose.yml up`

### Monitoring
- View Fly.io logs: `fly logs`
- View local production logs: `make docker-prod-logs`
- Check deployment status: `fly status`
- Access production console: `fly ssh console`

### Management
- Scale app manually: `fly scale count <number>`
- Stop local production server: `make docker-prod-stop`
- Restart app: `fly app restart`

## Production Configuration

The production environment (`docker/fly/`) uses the following configuration:
- Uses `base.en` Whisper model optimized for resource efficiency
- Tuned for performance with memory constraints
- Configured specifically for deployment to Fly.io

### Resource Allocation

- **Memory**: 1.8GB maximum with 512MB reservation
- **CPU**: 1.0 CPU maximum with 0.25 CPU reservation 
- **Storage**: Persistent volume for database storage

### Scaling Configuration

- **Min Running**: 1 machine minimum to ensure availability
- **Max Running**: 3 machines maximum for cost control
- **Concurrency**: 400-500 connections per machine
- **Auto-start/stop**: Machines start and stop automatically based on demand

See the [Docker Fly README](docker/fly/README.md) for detailed configuration information.