import argparse
import json
import sys
import logging
import requests

# Configure logging to use stderr instead of stdout
logging.basicConfig(level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s", stream=sys.stderr)
logger = logging.getLogger(__name__)

import yt_dlp


class ValidationError(Exception):
    """Base exception for validation errors."""

    pass


class NullLogger:
    """A logger class that does nothing. Used to suppress yt_dlp output."""

    def debug(self, msg):
        pass

    def warning(self, msg):
        pass

    def error(self, msg):
        pass


MAX_VIDEO_DURATION = 4 * 3600  # 4 hours in seconds


def validate_url(url: str) -> dict:
    """
    Validate the media URL.

    Args:
        url (str): The URL to validate.

    Returns:
        dict: Validation result containing 'valid', 'duration', 'format', and 'error'.
    """
    result = {
        "valid": False,
        "duration": 0,
        "format": "",
        "error": "",
        "url": url,
    }

    ydl_opts = {
        "quiet": True,
        "no_warnings": True,
        "logger": NullLogger(),  # Suppress yt_dlp logs
    }

    try:
        with yt_dlp.YoutubeDL(ydl_opts) as ydl:
            info = ydl.extract_info(url, download=False)

            if not isinstance(info, dict):
                raise ValidationError("Failed to extract video information.")

            duration = info.get("duration", 0)
            format_ext = info.get("ext", "")

            if duration > MAX_VIDEO_DURATION:
                raise ValidationError(
                    f"Video too long: {duration} seconds (max: {MAX_VIDEO_DURATION} seconds)"
                )

            # Additional validations can be added here (e.g., supported formats)

            # If all validations pass
            result.update(
                {
                    "valid": True,
                    "duration": duration,
                    "format": format_ext,
                    "error": "",
                }
            )

    except yt_dlp.utils.DownloadError as e:
        result["error"] = f"Download error: {str(e)}"
        result["valid"] = False
    except ValidationError as ve:
        result["error"] = str(ve)
        result["valid"] = False
    except requests.exceptions.RequestException as e:
        result["error"] = f"Network error: {str(e)}"
        result["valid"] = False
    except OSError as e:
        result["error"] = f"File system error: {str(e)}"
        result["valid"] = False
    except json.JSONDecodeError as e:
        result["error"] = f"Invalid JSON: {str(e)}"
        result["valid"] = False
    except Exception as e:
        import traceback

        result["error"] = f"Unexpected error: {str(e)}\n{traceback.format_exc()}"
        result["valid"] = False

    return result


def main():
    parser = argparse.ArgumentParser(description="Validate Media URL")
    parser.add_argument("--url", type=str, required=True, help="URL to validate")
    args = parser.parse_args()

    url = args.url.strip()

    if not url:
        sys.exit(1)

    try:
        result = validate_url(url)

        # Standardize the JSON response
        response = {
            "valid": result["valid"],
            "duration": result["duration"],
            "format": result["format"],
            "error": result["error"],
            "url": result["url"],
        }
        
        # Output the JSON response to stdout
        sys.stdout.write(json.dumps(response))
        sys.stdout.flush()

        if not result["valid"]:
            sys.exit(1)  # Exit with status 1 for invalid URL

    except Exception as e:
        # Catch any unexpected exceptions and format the error response
        response = {
            "valid": False,
            "duration": 0,
            "format": "",
            "error": f"Critical error: {str(e)}",
            "url": url,
        }
        
        # Output the JSON response to stdout
        sys.stdout.write(json.dumps(response))
        sys.stdout.flush()
        sys.exit(2)  # Exit with status 2 for critical errors


if __name__ == "__main__":
    main()
