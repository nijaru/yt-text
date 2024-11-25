# yt-text

A tool that combines [yt-dlp](https://github.com/yt-dlp/yt-dlp) and [Faster Whisper](https://github.com/guillaumekln/faster-whisper) to convert videos to text. It operates as a web server that accepts URLs from YouTube or any other platform supported by yt-dlp (such as Vimeo, Twitter, TikTok, etc.) and returns the video's transcript.

The current `transcribe.py` script is designed to work with the web server. A standalone version for direct command-line usage will be added in a future update.

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

2. (Optional) Create a virtual environment and activate it:

   ```sh
   python -m venv venv
   source venv/bin/activate  # On Windows use `venv\Scripts\activate`
   ```

3. Install Python dependencies:

   ```sh
   pip install torch yt-dlp faster-whisper pydub
   ```

4. Run the web server:
   ```sh
   go run main.go
   ```

## Current Usage

The `transcribe.py` script is currently integrated with the web server and expects specific arguments:

```sh
python transcribe.py --url <youtube-url> [--model <model_name>] [--json]
```

Options:

- `--url`: Video URL to transcribe (required) - supports YouTube and other platforms compatible with yt-dlp
- `--model`: Whisper model to use (default: "base.en")
- `--json`: Output in JSON format

Note: A standalone version of the script for direct command-line usage will be provided in a future update.

## Available Models

The script supports various Whisper models:

- `tiny`
- `base`
- `small`
- `medium`
- `large`

Language-specific models are also available (e.g., `base.en` for English-optimized model).

## Limitations

- Maximum video duration: 4 hours
  - Can be changed in transcribe.py
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
