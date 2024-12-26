# yt-text

A tool that combines [yt-dlp](https://github.com/yt-dlp/yt-dlp) and [Faster Whisper](https://github.com/guillaumekln/faster-whisper) to convert videos to text. It operates as a web server that accepts URLs from YouTube or any other platform supported by yt-dlp (such as Vimeo, Twitter, TikTok, etc.) and returns the video's transcript.

A standalone version for direct command-line usage is available at `python/scripts/ytext.py`.

## Features

- Download audio from YouTube and other platforms supported by yt-dlp
- Convert audio to text using Faster Whisper (optimized version of OpenAI Whisper)
- Web server interface for easy URL submission
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

### Command Line

The script `ytext.py` provides a simple interface for downloading audio and transcribing it to text. You can provide the URL either as a positional argument or with the `--url` flag:

```sh
uv run scripts/ytext.py <youtube-url>
uv run scripts/ytext.py --url <youtube-url>    # same as above
```

Multiple URLs can be provided either as comma-separated values or as separate arguments:

```sh
uv run scripts/ytext.py url1,url2,url3
uv run scripts/ytext.py url1 url2 url3         # same as above
```

You can also run it directly with Python after installing dependencies and ensuring you are using a supported version of Python:

```sh
python3 scripts/ytext.py <youtube-url>
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

## Web Server

### Docker (Recommended)

In order to run the web server using Docker, you need to have Docker installed on your system. There is a Dockerfile and docker-compose.yml file included in the repository. The server can be run manually, but this is not actively tested.

1. Build and run the Docker container:

   ```sh
   docker-compose up --build
   ```

## License

This project is licensed under the GNU Affero General Public License (AGPL) version 3. See the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Acknowledgements

- [yt-dlp](https://github.com/yt-dlp/yt-dlp) for downloading audio
- [Faster Whisper](https://github.com/guillaumekln/faster-whisper) for transcription
- [OpenAI Whisper](https://github.com/openai/whisper) for the original model
