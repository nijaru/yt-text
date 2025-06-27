import unittest
from unittest.mock import patch, MagicMock

import pytest

from scripts.validate import validate_url, ValidationError, MAX_VIDEO_DURATION


class TestValidate(unittest.TestCase):
    """Test cases for the validate module."""

    @patch('yt_dlp.YoutubeDL')
    def test_valid_url(self, mock_ytdl):
        """Test validation with a valid URL."""
        # Mock the YoutubeDL instance
        mock_instance = MagicMock()
        mock_ytdl.return_value.__enter__.return_value = mock_instance
        
        # Set up mock info dict with valid duration
        mock_instance.extract_info.return_value = {
            'duration': 300,  # 5 minutes
            'ext': 'mp4'
        }
        
        result = validate_url('https://www.youtube.com/watch?v=dQw4w9WgXcQ')
        
        # Assertions
        self.assertTrue(result['valid'])
        self.assertEqual(result['duration'], 300)
        self.assertEqual(result['format'], 'mp4')
        self.assertEqual(result['error'], '')
        self.assertEqual(result['url'], 'https://www.youtube.com/watch?v=dQw4w9WgXcQ')

    @patch('yt_dlp.YoutubeDL')
    def test_video_too_long(self, mock_ytdl):
        """Test validation with a video that exceeds maximum duration."""
        # Mock the YoutubeDL instance
        mock_instance = MagicMock()
        mock_ytdl.return_value.__enter__.return_value = mock_instance
        
        # Set up mock info dict with excessive duration
        mock_instance.extract_info.return_value = {
            'duration': MAX_VIDEO_DURATION + 100,  # Exceeds max duration
            'ext': 'mp4'
        }
        
        result = validate_url('https://www.youtube.com/watch?v=dQw4w9WgXcQ')
        
        # Assertions
        self.assertFalse(result['valid'])
        self.assertIn('Video too long', result['error'])

    @patch('yt_dlp.YoutubeDL')
    def test_download_error(self, mock_ytdl):
        """Test validation when yt-dlp encounters a download error."""
        # Mock the YoutubeDL instance to raise a DownloadError
        mock_instance = MagicMock()
        mock_ytdl.return_value.__enter__.return_value = mock_instance
        
        import yt_dlp
        mock_instance.extract_info.side_effect = yt_dlp.utils.DownloadError('Video unavailable')
        
        result = validate_url('https://www.youtube.com/watch?v=invalid')
        
        # Assertions
        self.assertFalse(result['valid'])
        self.assertIn('Download error', result['error'])

    @patch('yt_dlp.YoutubeDL')
    def test_non_dict_info(self, mock_ytdl):
        """Test validation when yt-dlp returns non-dict info."""
        # Mock the YoutubeDL instance
        mock_instance = MagicMock()
        mock_ytdl.return_value.__enter__.return_value = mock_instance
        
        # Set up mock to return a non-dict value
        mock_instance.extract_info.return_value = None
        
        result = validate_url('https://www.youtube.com/watch?v=dQw4w9WgXcQ')
        
        # Assertions
        self.assertFalse(result['valid'])
        self.assertIn('Failed to extract video information', result['error'])


if __name__ == '__main__':
    unittest.main()