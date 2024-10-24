import argparse
import yt_dlp


def validate_url(url) -> bool:
    if not url.startswith(("http://", "https://")):
        raise ValueError("URL must start with http:// or https://")
    try:
        ydl_opts = {
            'quiet': True,  # Suppress yt-dlp output
            'no_warnings': True,
        }
        with yt_dlp.YoutubeDL(ydl_opts) as ydl:
            info = ydl.extract_info(url, download=False)
        
        if info is None:
            raise ValueError("Failed to extract information from the URL")
        
        # Log the info dictionary for debugging
        print("Extracted info:", info)
        
        # Check if the URL points to a single video
        if info.get('_type') and info['_type'] != 'video':
            raise ValueError("URL must be for a single video")
        
        # If _type is not present, assume it's a single video
        if '_type' not in info:
            return True
        
    except yt_dlp.utils.DownloadError as e:
        raise ValueError(f"DownloadError: {e}")
    except Exception as e:
        raise ValueError(f"An error occurred: {e}")
    
    return True


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="URL to validate for download")
    parser.add_argument("url", type=str, help="URL to validate for download")
    args = parser.parse_args()
    url = args.url
    try:
        if validate_url(url):
            print(True)
    except ValueError as e:
        print(e)