# yt-text

A tool that combines [yt-dlp](https://github.com/yt-dlp/yt-dlp) and [Faster Whisper](https://github.com/guillaumekln/faster-whisper) to convert videos to text. It operates as a web server that accepts URLs from YouTube or any other platform supported by yt-dlp (such as Vimeo, Twitter, TikTok, etc.) and returns the video's transcript.

A standalone version for direct command-line usage is available at `python/scripts/ytext.py`.

## Features

- Download audio from YouTube and other platforms supported by yt-dlp
- Convert audio to text using Faster Whisper (optimized version of OpenAI Whisper)
- Fetch existing YouTube captions when available (with fallback to Whisper)
- Web server interface with WebSocket support for real-time updates
- gRPC communication between Go and Python services for improved performance
- Hybrid storage approach for efficient transcription storage
- Job queue with prioritization and cancellation support
- Multiple concurrent transcription jobs with real-time progress updates
- Support for various Whisper models
- JSON output

## Installation

1. Clone the repository:

   ```sh
   git clone https://github.com/nijaru/yt-text.git
   cd yt-text
   ```

2. Install dependencies using uv (recommended):

   ```sh
   cd python && uv sync
   ```

## Usage

### Using the gRPC Server

The application uses gRPC for efficient communication between Go and Python:

1. Start the gRPC server:

   ```sh
   # Using the convenience script
   ./python/start_grpc.sh 50051
   
   # Or using make
   make grpc-server
   ```

2. Enable gRPC in the Go service:

   ```sh
   # Set environment variables
   export USE_GRPC=true
   export GRPC_SERVER_ADDRESS=localhost:50051
   
   # Or use Docker Compose which sets these by default
   docker-compose up
   ```

### Web Server

The web server provides a user-friendly interface for transcribing videos:

1. Start the server using Docker (recommended):

   ```sh
   docker-compose up --build
   ```

2. Access the web interface at http://localhost:8080

3. Enter a YouTube URL and submit the form to begin transcription

4. Monitor real-time progress via WebSocket updates

5. Multiple transcription jobs can be submitted simultaneously

### Command Line

The script `ytext.py` provides a simple interface for downloading audio and transcribing it to text:

```sh
cd python && uv run scripts/ytext.py <youtube-url>
```

Multiple URLs can be provided either as comma-separated values or as separate arguments:

```sh
uv run scripts/ytext.py url1,url2,url3
uv run scripts/ytext.py url1 url2 url3         # same as above
```

### Options

- `--model`: Specify the Whisper model to use. (Default is `base.en`)

### Available Models

The script supports various Whisper models:

- `tiny`
- `base`
- `small`
- `medium`
- `large`

Language-specific models are also available (e.g., `base.en` for English-optimized model).

## Development Roadmap

See [todo.md](todo.md) for current development priorities and future enhancements.

## License

This project is licensed under the GNU Affero General Public License (AGPL) version 3. See the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Acknowledgements

- [yt-dlp](https://github.com/yt-dlp/yt-dlp) for downloading audio
- [Faster Whisper](https://github.com/guillaumekln/faster-whisper) for transcription
- [OpenAI Whisper](https://github.com/openai/whisper) for the original model