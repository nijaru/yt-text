import unittest
import json
import sys
from unittest.mock import patch, MagicMock, call

import pytest
import requests

from scripts.validate import (
    validate_url,
    ValidationError,
    MAX_VIDEO_DURATION,
    main,
    NullLogger
)


class TestValidateAdvanced(unittest.TestCase):
    """Advanced test cases for the validate module."""

    def test_null_logger(self):
        """Test NullLogger functionality."""
        logger = NullLogger()
        
        # Verify logger methods don't raise exceptions when called
        logger.debug("Test debug message")
        logger.warning("Test warning message")
        logger.error("Test error message")
        
        # Nothing to assert since these methods should just do nothing

    @patch('yt_dlp.YoutubeDL')
    def test_network_error(self, mock_ytdl):
        """Test validation handling for network errors."""
        # Mock YoutubeDL to raise network error
        mock_instance = MagicMock()
        mock_ytdl.return_value.__enter__.return_value = mock_instance
        
        # Simulate network error
        network_error = requests.exceptions.RequestException("Network error")
        mock_instance.extract_info.side_effect = network_error
        
        # Call the function
        result = validate_url('https://www.youtube.com/watch?v=video123')
        
        # Verify handling
        self.assertFalse(result['valid'])
        self.assertEqual(result['duration'], 0)
        self.assertEqual(result['format'], "")
        self.assertIn('Network error', result['error'])
        self.assertEqual(result['url'], 'https://www.youtube.com/watch?v=video123')

    @patch('yt_dlp.YoutubeDL')
    def test_os_error(self, mock_ytdl):
        """Test validation handling for OS errors."""
        # Mock YoutubeDL to raise OS error
        mock_instance = MagicMock()
        mock_ytdl.return_value.__enter__.return_value = mock_instance
        
        # Simulate file system error
        os_error = OSError("File system error")
        mock_instance.extract_info.side_effect = os_error
        
        # Call the function
        result = validate_url('https://www.youtube.com/watch?v=video123')
        
        # Verify handling
        self.assertFalse(result['valid'])
        self.assertEqual(result['duration'], 0)
        self.assertEqual(result['format'], "")
        self.assertIn('File system error', result['error'])
        self.assertEqual(result['url'], 'https://www.youtube.com/watch?v=video123')

    @patch('yt_dlp.YoutubeDL')
    def test_json_decode_error(self, mock_ytdl):
        """Test validation handling for JSON decode errors."""
        # Mock YoutubeDL to raise JSON decode error
        mock_instance = MagicMock()
        mock_ytdl.return_value.__enter__.return_value = mock_instance
        
        # Simulate JSON decode error
        json_error = json.JSONDecodeError("Invalid JSON", "", 0)
        mock_instance.extract_info.side_effect = json_error
        
        # Call the function
        result = validate_url('https://www.youtube.com/watch?v=video123')
        
        # Verify handling
        self.assertFalse(result['valid'])
        self.assertEqual(result['duration'], 0)
        self.assertEqual(result['format'], "")
        self.assertIn('Invalid JSON', result['error'])
        self.assertEqual(result['url'], 'https://www.youtube.com/watch?v=video123')

    @patch('yt_dlp.YoutubeDL')
    def test_generic_exception(self, mock_ytdl):
        """Test validation handling for unexpected exceptions."""
        # Mock YoutubeDL to raise a generic exception
        mock_instance = MagicMock()
        mock_ytdl.return_value.__enter__.return_value = mock_instance
        
        # Simulate unexpected error
        generic_error = Exception("Something went wrong")
        mock_instance.extract_info.side_effect = generic_error
        
        # Call the function
        result = validate_url('https://www.youtube.com/watch?v=video123')
        
        # Verify handling
        self.assertFalse(result['valid'])
        self.assertEqual(result['duration'], 0)
        self.assertEqual(result['format'], "")
        self.assertIn('Unexpected error', result['error'])
        self.assertIn('Something went wrong', result['error'])
        self.assertEqual(result['url'], 'https://www.youtube.com/watch?v=video123')


class TestValidateMainFunction(unittest.TestCase):
    """Test cases for the main function in validate.py."""

    def setUp(self):
        """Set up test environment."""
        # Patch sys.exit to prevent tests from actually exiting
        self.exit_patcher = patch('sys.exit')
        self.mock_exit = self.exit_patcher.start()
        
        # Patch print to capture output
        self.print_patcher = patch('builtins.print')
        self.mock_print = self.print_patcher.start()
        
        # Patch argparse.ArgumentParser.parse_args
        self.parse_args_patcher = patch('argparse.ArgumentParser.parse_args')
        self.mock_parse_args = self.parse_args_patcher.start()
        
        # Patch validate_url
        self.validate_url_patcher = patch('scripts.validate.validate_url')
        self.mock_validate_url = self.validate_url_patcher.start()

    def tearDown(self):
        """Clean up after tests."""
        self.exit_patcher.stop()
        self.print_patcher.stop()
        self.parse_args_patcher.stop()
        self.validate_url_patcher.stop()

    def test_main_with_valid_url(self):
        """Test main function with a valid URL."""
        # Setup arguments
        mock_args = MagicMock()
        mock_args.url = "https://www.youtube.com/watch?v=valid"
        self.mock_parse_args.return_value = mock_args
        
        # Setup validation result
        self.mock_validate_url.return_value = {
            "valid": True,
            "duration": 300,
            "format": "mp4",
            "error": "",
            "url": "https://www.youtube.com/watch?v=valid"
        }
        
        # Call the function
        main()
        
        # Verify validate_url was called with correct URL
        self.mock_validate_url.assert_called_once_with("https://www.youtube.com/watch?v=valid")
        
        # Verify result was printed as JSON
        self.mock_print.assert_called_once()
        printed_data = self.mock_print.call_args[0][0]
        self.assertIsInstance(printed_data, str)
        
        # Should not exit with error
        self.mock_exit.assert_not_called()

    def test_main_with_invalid_url(self):
        """Test main function with an invalid URL."""
        # Setup arguments
        mock_args = MagicMock()
        mock_args.url = "https://www.youtube.com/watch?v=invalid"
        self.mock_parse_args.return_value = mock_args
        
        # Setup validation result
        self.mock_validate_url.return_value = {
            "valid": False,
            "duration": 0,
            "format": "",
            "error": "Video unavailable",
            "url": "https://www.youtube.com/watch?v=invalid"
        }
        
        # Call the function
        main()
        
        # Verify validate_url was called with correct URL
        self.mock_validate_url.assert_called_once_with("https://www.youtube.com/watch?v=invalid")
        
        # Verify result was printed as JSON
        self.mock_print.assert_called_once()
        
        # Should exit with status 1 for invalid URL
        self.mock_exit.assert_called_once_with(1)

    def test_main_with_empty_url(self):
        """Test main function with an empty URL."""
        # Setup arguments
        mock_args = MagicMock()
        mock_args.url = ""  # Empty URL
        self.mock_parse_args.return_value = mock_args
        
        # Call the function
        main()
        
        # Verify validate_url was not called
        self.mock_validate_url.assert_not_called()
        
        # Verify error was printed as JSON
        self.mock_print.assert_called_once()
        printed_data = self.mock_print.call_args[0][0]
        error_json = json.loads(printed_data)
        self.assertFalse(error_json["valid"])
        self.assertIn("No URL provided", error_json["error"])
        
        # Should exit with status 1 for missing URL
        self.mock_exit.assert_called_once_with(1)

    def test_main_with_critical_error(self):
        """Test main function handling unexpected exceptions."""
        # Setup arguments
        mock_args = MagicMock()
        mock_args.url = "https://www.youtube.com/watch?v=valid"
        self.mock_parse_args.return_value = mock_args
        
        # Make validate_url raise an exception
        self.mock_validate_url.side_effect = Exception("Critical test error")
        
        # Call the function
        main()
        
        # Verify error was printed as JSON
        self.mock_print.assert_called_once()
        printed_data = self.mock_print.call_args[0][0]
        error_json = json.loads(printed_data)
        self.assertFalse(error_json["valid"])
        self.assertIn("Critical error", error_json["error"])
        self.assertIn("Critical test error", error_json["error"])
        
        # Should exit with status 2 for critical errors
        self.mock_exit.assert_called_once_with(2)


if __name__ == '__main__':
    unittest.main()