# YouTube Transcription Service - TODO

*Note: For the current status of in-progress build fixes, see [progress.md](progress.md)*

## High Priority

1. **Testing Completion** 📝 (In Progress)
   - Complete Python component test coverage:
     - Improve test coverage for transcription.py (currently 43%)
     - Add missing tests for validate.py (currently 0%)
     - Add tests for ytext.py CLI module
     - Add tests for grpc/server.py and grpc/start_server.py
   - Create end-to-end tests for the Docker environment
   - Add integration tests for Python gRPC service
   - Create end-to-end tests for the WebSocket workflow

2. **Performance Improvements** ⚡
   - Implement disk-based caching for processed audio chunks
   - Add memory usage metrics collection
   - Implement adaptive chunk sizing based on system resources
   - ✅ Optimize Docker build for faster startup
   - Improve error handling between Go server and Python gRPC service
   
3. **Fix Build Issues** ✅
   - ✅ Create temporary Python-only container build for gRPC server
   - ✅ Update to Go 1.24 for both local and production builds
   - ✅ Add replace directive for Go module imports
   - ✅ Fix Go protobuf import path issues:
     - ✅ Resolve path issues with `yt-text/protos/transcribe` imports
     - ✅ Fix Docker build with proper module structure
   - ✅ Fix integration between Go server and Python gRPC components
   - ✅ Merge Python gRPC container and Go server container
   - ✅ Fix missing git dependency in Docker Alpine image
   - ✅ Fix Go module import issues with Google's protobuf packages
   - ✅ Improve logging with structured zerolog implementation
   - ✅ Update error handling to use Fiber's built-in error system
   - Ensure Docker builds work correctly in CI/CD pipeline
   - Create documentation for proper local development setup

## Medium Priority

1. **User Experience Enhancements** 💻
   - Add dark mode support to the web interface
   - Implement responsive design for mobile devices
   - Add transcript export options (TXT, SRT, VTT)
   - Create shareable links for transcriptions

2. **Documentation** 📚
   - Document API endpoints with OpenAPI/Swagger
   - Add developer onboarding documentation
   - Update test documentation with recent changes
   - Create architecture diagram for the system

## Future Enhancements

1. **Advanced Features** ✨
   - Support for custom Whisper model parameters
   - Add speaker diarization capability
   - Implement translation features using Whisper multilingual models
   - Add support for transcript editing and corrections

2. **MCP Integration** 🔌
   - Add MCP (Media Content Processing) integration for subtitle/transcription
   - Create MCP plugin architecture for easy extension
   - Implement standardized output format for subtitle files
   - Add API endpoint for MCP compatibility
   - Support both direct transcription and subtitle generation modes

3. **Infrastructure Improvements** 🏗️
   - Set up monitoring and alerting with Prometheus/Grafana
   - Add distributed tracing with OpenTelemetry
   - Implement database migrations for schema changes
   - Create backup and restore procedures for database

## Completed Tasks

- ✅ **CI/CD Integration**
  - Set up CI/CD pipeline for automatic deployments to Fly.io
  - Add automatic testing before deployment
  - Create deployment documentation

- ✅ **WebSocket Improvements**
  - Add detailed progress updates with percentage completion
  - Implement structured error handling and error codes
  - Add automatic reconnection with exponential backoff
  - Implement job status querying endpoint

- ✅ **Memory Optimization**
  - Optimize memory usage for Whisper transcription
  - Implement streaming audio processing with pydub (5MB chunks)
  - Add progress metrics and performance monitoring
  - Implement resource cleanup and CUDA cache clearing
  - Reduce Docker image size for faster deployments

- ✅ **Security Enhancements**
  - Implement rate limiting for all endpoints
  - Add CORS configuration for production
  - Implement request validation middleware
  - Add security headers to HTTP responses

- ✅ **Initial Testing**
  - Add unit tests for Go components
  - Fixed failing tests in test_transcription.py
  - Create test documentation for both Go and Python components

- ✅ **Python Dependencies**
  - Migrate to uv package manager from pip
  - Update pyproject.toml with proper dependencies
  - Generate requirements.txt with uv pip compile
  - Add dev dependencies for testing and linting

- ✅ **gRPC Service**
  - Fix Python import paths for grpc modules
  - Update start_server.py to work with uv
  - Generate proper protobuf files with grpc_tools.protoc
  - Successfully integrate with Go backend