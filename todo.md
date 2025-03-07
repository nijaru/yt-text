# YouTube Transcription Service - TODO

## Next Steps

1. **Deploy on Fly.io**
   - Create and configure fly.toml file
   - Update Dockerfile for Fly.io deployment
   - Set up persistent volume for SQLite database
   - Configure memory and CPU constraints for efficient operation
   - Implement health check endpoints
   - Configure environment variables for production
   - Set up CI/CD pipeline for automatic deployments

2. **Enhance WebSocket Communication Layer**
   - Add more detailed progress updates (downloading, processing, etc.)
   - Improve error handling and reconnection logic
   - Add support for job status querying

3. **Additional Improvements**
   - Add comprehensive testing for new components
   - Complete documentation for the new architecture
   - Implement rate limiting for job submissions

## Possible Future Enhancements

1. **MCP Integration**
   - Add MCP (Media Content Processing) integration for subtitle/transcription
   - Create MCP plugin architecture for easy extension
   - Implement standardized output format for subtitle files
   - Add API endpoint for MCP compatibility
   - Support both direct transcription and subtitle generation modes
   - Add documentation for MCP integration