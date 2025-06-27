import os
import unittest
import tempfile
import json
from unittest.mock import patch, MagicMock, mock_open

import pytest
import torch
import numpy as np

# Add mock for faster_whisper before import
import sys
from unittest.mock import MagicMock

# Mock CUDA functions
class MockCudaDevice:
    def __init__(self, total_memory=8 * 1024 * 1024 * 1024):
        self.total_memory = total_memory

# Mock the WhisperModel class
class MockWhisperModel:
    def __init__(self, *args, **kwargs):
        pass
    
    def transcribe(self, *args, **kwargs):
        # Return empty segments and info
        class MockInfo:
            language = "en"
            language_probability = 0.95
        
        class MockSegment:
            def __init__(self, text="Test transcription"):
                self.text = text
        
        return [MockSegment()], MockInfo()

# Add the mock to sys.modules
sys.modules['faster_whisper'] = MagicMock()
sys.modules['faster_whisper'].WhisperModel = MockWhisperModel

# Mock torch.cuda functions
torch.cuda.is_available = MagicMock(return_value=True)
torch.cuda.get_device_properties = MagicMock(return_value=MockCudaDevice())
torch.cuda.get_device_capability = MagicMock(return_value=[7, 5])
torch.cuda.empty_cache = MagicMock()

# Now import the module we want to test
from scripts.transcription import Transcriber, TranscriptionError


class TestTranscriber(unittest.TestCase):
    """Test cases for the Transcriber class."""

    @patch('torch.cuda.is_available')
    @patch('psutil.cpu_count')
    def test_init_with_defaults(self, mock_cpu_count, mock_cuda_available):
        """Test initialization with default values."""
        # Mock hardware detection
        mock_cuda_available.return_value = False
        mock_cpu_count.return_value = 4

        # Need to patch WhisperModel to avoid actual model loading
        with patch('scripts.transcription.WhisperModel') as mock_whisper:
            transcriber = Transcriber()

            # Verify initialization with default settings
            self.assertEqual(transcriber.model_name, "large-v3-turbo")
            self.assertEqual(transcriber.device, "cpu")
            self.assertEqual(transcriber.compute_type, "float32")
            self.assertTrue(transcriber.enable_cache)
            self.assertEqual(transcriber.cache_dir, "/tmp/audio_cache")
            self.assertEqual(transcriber.max_cache_size_gb, 10.0)
            self.assertEqual(transcriber.chunk_length_seconds, 120)

            # Verify WhisperModel was initialized
            mock_whisper.assert_called_once()

    @patch('torch.cuda.is_available')
    @patch('torch.cuda.get_device_properties')
    def test_init_with_cuda(self, mock_gpu_props, mock_cuda_available):
        """Test initialization with CUDA available."""
        # Mock hardware detection for GPU
        mock_cuda_available.return_value = True
        mock_gpu = MagicMock()
        mock_gpu.total_memory = 8 * 1024 * 1024 * 1024  # 8GB RAM
        mock_gpu_props.return_value = mock_gpu

        # Mock CUDA capabilities for compute type
        with patch('torch.cuda.get_device_capability', return_value=[7, 5]):
            # Need to patch WhisperModel to avoid actual model loading
            with patch('scripts.transcription.WhisperModel') as mock_whisper:
                transcriber = Transcriber()

                # Verify GPU settings
                self.assertEqual(transcriber.device, "cuda")
                self.assertEqual(transcriber.compute_type, "float16")
                
                # Verify WhisperModel was initialized with GPU settings
                mock_whisper.assert_called_once_with(
                    "large-v3-turbo",
                    device="cuda",
                    compute_type="float16",
                    download_root="/tmp/models",
                    cpu_threads=4  # Default for GPU
                )

    @patch('os.path.exists')
    @patch('os.utime')
    @patch('builtins.open', new_callable=mock_open)
    @patch('scripts.transcription.WhisperModel')
    def test_get_from_cache_hit(self, mock_whisper, mock_file, mock_utime, mock_exists):
        """Test retrieving a result from cache when cache hit occurs."""
        # Setup cache hit scenario
        mock_exists.return_value = True
        mock_file.return_value.__enter__.return_value.read.return_value = json.dumps({
            "text": "Cached transcription result",
            "model_name": "large-v3-turbo",
            "language": "en",
            "language_probability": 0.98,
            "timestamp": 1612345678.0
        })

        transcriber = Transcriber(enable_cache=True)
        result = transcriber._get_from_cache("video123", 0)

        # Verify cache was checked and result returned
        self.assertIsNotNone(result)
        self.assertEqual(result["text"], "Cached transcription result")
        self.assertEqual(result["language"], "en")
        self.assertEqual(result["language_probability"], 0.98)
        
        # Verify cache access time was updated
        mock_utime.assert_called_once()

    @patch('os.path.exists')
    @patch('scripts.transcription.WhisperModel')
    def test_get_from_cache_miss(self, mock_whisper, mock_exists):
        """Test cache miss scenario."""
        # Setup cache miss
        mock_exists.return_value = False

        transcriber = Transcriber(enable_cache=True)
        result = transcriber._get_from_cache("video123", 0)

        # Verify cache miss returns None
        self.assertIsNone(result)

    @patch('scripts.transcription.Path')
    @patch('scripts.transcription.WhisperModel')
    def test_manage_cache_size(self, mock_whisper, mock_path):
        """Test cache size management when cache exceeds limit."""
        # Mock directory structure with files
        mock_files = []
        total_size = 0
        
        # Create mock files (11GB total to exceed 10GB default)
        for i in range(5):
            file_size = 2.2 * 1024 * 1024 * 1024  # 2.2GB each
            mock_file = MagicMock()
            mock_file.is_file.return_value = True
            mock_file.stat.return_value.st_size = file_size
            mock_file.stat.return_value.st_mtime = 1612345678.0 + i  # Increasingly newer
            mock_file.name = f"cache_file_{i}.json"
            mock_files.append(mock_file)
            total_size += file_size
        
        # Setup Path.glob to return our mock files
        mock_path.return_value.glob.return_value = mock_files
        
        # Create transcriber but avoid running _manage_cache_size on initialization
        with patch('os.path.exists', return_value=True):
            with patch.object(Transcriber, '_manage_cache_size', side_effect=lambda: None):
                transcriber = Transcriber(max_cache_size_gb=10.0)
            
            # Mock the unlink method to track calls
            for mock_file in mock_files:
                mock_file.unlink = MagicMock()
            
            # Call the method directly after setup
            transcriber._manage_cache_size()
            
            # Verify oldest file was removed
            mock_files[0].unlink.assert_called_once()
            
            # Verify newer files weren't removed
            mock_files[3].unlink.assert_not_called()
            mock_files[4].unlink.assert_not_called()

    @patch('yt_dlp.YoutubeDL')
    @patch('scripts.transcription.WhisperModel')
    def test_download_audio_success(self, mock_whisper, mock_ytdl):
        """Test successful audio download."""
        # Mock YoutubeDL context for info extraction
        mock_info_instance = MagicMock()
        mock_info = {
            'title': 'Test Video',
            'duration': 300,  # 5 minutes
            'id': 'video123'
        }
        mock_info_instance.extract_info.return_value = mock_info
        
        # Mock YoutubeDL context for download
        mock_download_instance = MagicMock()
        mock_download_instance.extract_info.return_value = mock_info
        mock_download_instance.prepare_filename.return_value = '/tmp/video123.mp4'
        
        # Set up the two context managers to be called in sequence
        mock_ytdl.side_effect = [
            MagicMock(__enter__=MagicMock(return_value=mock_info_instance)),
            MagicMock(__enter__=MagicMock(return_value=mock_download_instance))
        ]
        
        # Patch os.path functions
        with patch('os.path.exists') as mock_exists:
            with patch('os.path.splitext', return_value=('/tmp/video123', '.mp4')):
                with patch('os.path.getsize', return_value=1024):
                    # Set up exists to return True for wav file (simulating FFmpeg conversion)
                    mock_exists.side_effect = lambda path: path == '/tmp/video123.wav'
                    
                    transcriber = Transcriber()
                    audio_path, title, duration, video_id = transcriber._download_audio(
                        'https://www.youtube.com/watch?v=video123', 
                        '/tmp'
                    )
                    
                    # Verify correct information was returned
                    self.assertEqual(audio_path, '/tmp/video123.wav')
                    self.assertEqual(title, 'Test Video')
                    self.assertEqual(duration, 300)
                    self.assertEqual(video_id, 'video123')

    @patch('yt_dlp.YoutubeDL')
    @patch('scripts.transcription.WhisperModel')
    def test_download_audio_duration_exceeded(self, mock_whisper, mock_ytdl):
        """Test exception when video duration exceeds limit."""
        # Mock YoutubeDL context for info extraction
        mock_instance = MagicMock()
        mock_info = {
            'title': 'Long Test Video',
            'duration': 7200,  # 2 hours
            'id': 'video123'
        }
        mock_instance.extract_info.return_value = mock_info
        mock_ytdl.return_value.__enter__.return_value = mock_instance
        
        transcriber = Transcriber(max_video_duration=3600)  # 1 hour limit
        
        # Verify exception is raised
        with self.assertRaises(TranscriptionError) as context:
            transcriber._download_audio(
                'https://www.youtube.com/watch?v=video123', 
                '/tmp'
            )
        
        self.assertIn('exceeds maximum allowed', str(context.exception))

    @patch('tempfile.TemporaryDirectory')
    @patch('scripts.transcription.WhisperModel')
    def test_process_url_integration(self, mock_whisper, mock_temp_dir):
        """Test integration of process_url method."""
        # Create a mock for the temporary directory
        mock_temp_dir_instance = MagicMock()
        mock_temp_dir.return_value.__enter__.return_value = '/tmp/test_dir'
        
        transcriber = Transcriber()
        
        # Mock the internal methods that would be called
        with patch.object(transcriber, '_download_audio') as mock_download:
            with patch.object(transcriber, '_transcribe') as mock_transcribe:
                # Set up return values
                mock_download.return_value = ('/tmp/test_dir/audio.wav', 'Test Video', 300, 'video123')
                mock_transcribe.return_value = {
                    'text': 'This is a test transcription',
                    'model_name': 'large-v3-turbo',
                    'duration': 5.2,
                    'error': None,
                    'language': 'en',
                    'language_probability': 0.98
                }
                
                # Call the method
                result = transcriber.process_url('https://www.youtube.com/watch?v=video123')
                
                # Verify result
                self.assertEqual(result['text'], 'This is a test transcription')
                self.assertEqual(result['title'], 'Test Video')
                self.assertEqual(result['url'], 'https://www.youtube.com/watch?v=video123')
                self.assertEqual(result['language'], 'en')
                
                # Verify methods were called
                mock_download.assert_called_once_with('https://www.youtube.com/watch?v=video123', '/tmp/test_dir')
                mock_transcribe.assert_called_once_with('/tmp/test_dir/audio.wav', 300, 'video123')

    @patch('scripts.transcription.WhisperModel')
    def test_error_handling(self, mock_whisper):
        """Test error handling in process_url."""
        transcriber = Transcriber()
        
        # Mock _download_audio to raise an error
        with patch.object(transcriber, '_download_audio', side_effect=TranscriptionError('Test error')):
            result = transcriber.process_url('https://www.youtube.com/watch?v=video123')
            
            # Verify error was captured and formatted correctly
            self.assertEqual(result['error'], 'Test error')
            self.assertIsNone(result['text'])
            self.assertEqual(result['url'], 'https://www.youtube.com/watch?v=video123')
            self.assertIsNone(result['language'])
            
    @patch('torch.cuda.empty_cache')
    @patch('gc.collect')
    @patch('scripts.transcription.WhisperModel')
    @patch('torch.cuda.is_available')
    def test_close_method(self, mock_cuda_available, mock_whisper, mock_gc, mock_cuda_empty):
        """Test resource cleanup in close method."""
        # Setup - ensure cuda is seen as available
        mock_cuda_available.return_value = True
        
        transcriber = Transcriber()
        transcriber._temp_files = ['/tmp/test1.wav', '/tmp/test2.wav']
        
        # Mock file existence check and removal
        with patch('os.path.exists', return_value=True) as mock_exists:
            with patch('os.remove') as mock_remove:
                # Call close method
                transcriber.close()
                
                # Verify cleanup occurred
                self.assertEqual(mock_remove.call_count, 2)
                mock_cuda_empty.assert_called_once()
                mock_gc.assert_called_once()


if __name__ == '__main__':
    unittest.main()