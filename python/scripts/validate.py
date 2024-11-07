import argparse
import yt_dlp


def validate_url(url) -> bool:
    if not url.startswith(("http://", "https://")):
        print("URL must start with http:// or https://")
        return False
    try:
        ydl_opts = {
            'quiet': True,  # Suppress yt-dlp output
            'no_warnings': True,
        }
        with yt_dlp.YoutubeDL(ydl_opts) as ydl:
            info = ydl.extract_info(url, download=False)
        
        if info is None:
            print("Failed to extract information from the URL")
            return False
        
        # Log the info dictionary for debugging
        print("Extracted info:", info)
        
        # Check if the URL points to a single video
        if info.get('_type') and info['_type'] != 'video':
            print("URL must be for a single video")
            return False
        
        # If _type is not present, assume it's a single video
        if '_type' not in info:
            return True
        
    except yt_dlp.utils.DownloadError as e:
        print(f"DownloadError: {e}")
        return False
    except Exception as e:
        print(f"An error occurred: {e}")
        return False
    
    return True


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="URL to validate for download")
    parser.add_argument("url", type=str, help="URL to validate for download")
    args = parser.parse_args()
    url = args.url
    if validate_url(url):
        print(True)
    else:
        print(False)