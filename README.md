# yt-text

A tool that combines [yt-dlp](https://github.com/yt-dlp/yt-dlp) and [Faster Whisper](https://github.com/guillaumekln/faster-whisper) to convert videos to text. It operates as a web server that accepts URLs from YouTube or any other platform supported by yt-dlp (such as Vimeo, Twitter, TikTok, etc.) and returns the video's transcript.

A standalone version for direct command-line usage is available at `yt-text/python/scripts/ytext.py`.

## Features

- Download audio from YouTube and other platforms supported by yt-dlp
- Convert audio to text using Faster Whisper (optimized version of OpenAI Whisper)
- Web server interface for easy URL submission
- Support for various Whisper models
- JSON output
- GPU acceleration when available

## Installation

1. Clone the repository:

   ```sh
   git clone https://github.com/yourusername/yt-text.git
   cd yt-text
   ```

## Usage

### Command Line

The script `ytext.py` provides a simple interface for downloading audio and transcribing it to text. You can provide any number of urls as separate arguments or as comma-separated values. They can either be passed in directly or by using `--url`.

The script should support any URLs supported by yt-dlp, including YouTube, Vimeo, Twitter, and TikTok. The script will download the audio from the video and transcribe it to text using the specified Whisper model. The default model is `base.en` for English with a good balance of speed and accuracy.

Dependencies are listed in the `requirements.txt` file. You can install them using pip or simply run the script with [uv](https://github.com/astral-sh/uv).

```sh
python3 ytext.py <youtube-url>
```

```sh
python3 ytext.py --url <youtube-url>
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

### Web Server

#### Docker (Recommended)

In order to run the web server using Docker, you need to have Docker installed on your system. There is a Dockerfile and docker-compose.yml file included in the repository. The server can be run manually, but this is not actively tested.

1. Build and run the Docker container:

   ```sh
   docker-compose up --build
   ```

## Notes

The python code was recently refactored to add a standalone script with the same functionality as the web server. The refactor is not yet complete and needs to be tested. The manual server instructions have not yet been retested and may not work. The Makefile has not been updated to reflect the changes in the project structure. Docker is the recommend way of running the web server. The script should work standalone as long as the dependencies are installed.

## Limitations

- GPU acceleration requires CUDA-compatible hardware
  - Can use CPU if not available

## License

This project is licensed under the GNU Affero General Public License (AGPL) version 3. See the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Acknowledgements

- [yt-dlp](https://github.com/yt-dlp/yt-dlp) for downloading audio
- [Faster Whisper](https://github.com/guillaumekln/faster-whisper) for transcription
- [OpenAI Whisper](https://github.com/openai/whisper) for the original model
