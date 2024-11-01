# yt-text
Download audio from YouTube and convert it to text using [yt-dlp](https://github.com/yt-dlp/yt-dlp) and [OpenAI Whisper](https://github.com/openai/whisper).

This tool operates as a web server that accepts a YouTube URL (or any other URL supported by yt-dlp) and returns the video's transcript. Alternatively, you can run `transcribe.py` directly to transcribe a video.

## Features
- Download audio from YouTube and other supported platforms
- Convert audio to text using OpenAI Whisper
- Operates as a web server for easy URL submission
- Direct transcription via `transcribe.py`

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
3. Install dependencies:
    ```sh
    pip install -r requirements.txt
    ```
4. Run the web server:
    ```sh
    go run main.go
    ```

## Usage
- To transcribe a video directly, run:
    ```sh
    python transcribe.py <youtube-url>
    ```
  You can also specify the model name to use for transcription:
    ```sh
    python transcribe.py <youtube-url> --model <model_name>
    ```
  The default model is `base.en`. You can choose from other models like `tiny`, `small`, `medium`, `large`, etc.

## Examples
- Transcribe a video using the default model:
    ```sh
    python transcribe.py <youtube-url>
    ```
- Transcribe a video using the `base` model:
    ```sh
    python transcribe.py <youtube-url> --model base
    ```

## License
This project is licensed under the GNU Affero General Public License (AGPL) version 3. See the [LICENSE](LICENSE) file for details.

## Contributing
Contributions are welcome! Please open an issue or submit a pull request for any changes.

## Acknowledgements
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) for downloading audio
- [OpenAI Whisper](https://github.com/openai/whisper) for transcription