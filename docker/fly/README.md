# Fly.io Deployment Configuration

## Requirements

1. **Configure project for Fly.io deployment**
   - Create necessary deployment files
   - Set up persistent volume for SQLite
   - Configure memory and CPU constraints
   - Ensure proper environment variable handling

2. **Optimize Whisper model usage**
   - Use base.en model as default
   - Configure for memory efficiency
   - Set up CPU-only processing
   - Implement disk offloading where possible

## Configuration Files to Create

### 1. fly.toml
- Application name and region settings
- Memory allocation (256MB per instance)
- HTTP service configuration
- Volume mounting configuration
- Health check endpoint

### 2. Dockerfile Updates
- Ensure proper multi-stage build
- Optimize image size
- Set environment variables
- Configure volume mounting points

### 3. Volume Configuration
- Set up persistent storage for SQLite database
- Configure backup policy
- Handle volume initialization

## Environment Configuration

- PORT: 8080
- GO_ENV: production
- DB_PATH: /data/urls.db
- LOG_DIR: /data/logs
- VIDEO_MAX_DURATION: 4h
- WHISPER_MODEL: base.en
- SCRIPTS_PATH: /app/scripts
- TEMP_DIR: /tmp/transcribe

## Memory Optimization

1. **Whisper Configuration**
   - Set compute_type to INT8 or FLOAT16
   - Enable disk offloading
   - Disable GPU usage

2. **Process Management**
   - Limit concurrent Python processes
   - Implement graceful degradation under memory pressure
   - Add job queue throttling

## Deployment Instructions

1. Update model configuration in code to use base.en
2. Add Fly.io configuration files
3. Set up proper volume mounting
4. Configure memory constraints
5. Add health check endpoints
