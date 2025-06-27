import os
import unittest
import tempfile
import time
import json
from pathlib import Path
from unittest.mock import patch, MagicMock, mock_open, call

import pytest
import torch
import numpy as np

# Mock faster_whisper
import sys
from unittest.mock import MagicMock

# Mock WhisperModel for testing
class MockWhisperModel:
    def __init__(self, *args, **kwargs):
        self.args = args
        self.kwargs = kwargs
        self.transcribe_calls = []
    
    def transcribe(self, *args, **kwargs):
        # Record call for later verification
        self.transcribe_calls.append((args, kwargs))
        
        # Return mock transcription results
        class MockInfo:
            language = "en"
            language_probability = 0.95
        
        class MockSegment:
            def __init__(self, text="Test transcription", start=0.0, end=5.0):
                self.text = text
                self.start = start
                self.end = end
        
        # Generate different segments based on input to simulate chunks
        if len(args) > 0 and isinstance(args[0], str) and "chunk" in args[0]:
            chunk_num = int(args[0].split("chunk")[1].split(".")[0])
            segments = [
                MockSegment(f"Chunk {chunk_num} transcription part 1", start=0.0, end=2.5),
                MockSegment(f"Chunk {chunk_num} transcription part 2", start=2.5, end=5.0)
            ]
        else:
            segments = [MockSegment()]
        
        return segments, MockInfo()

# Add the mock to sys.modules
sys.modules['faster_whisper'] = MagicMock()
sys.modules['faster_whisper'].WhisperModel = MockWhisperModel

# Mock torch.cuda functions
torch.cuda.is_available = MagicMock(return_value=True)
torch.cuda.empty_cache = MagicMock()
torch.cuda.get_device_capability = MagicMock(return_value=[7, 5])
torch.cuda.get_device_properties = MagicMock(return_value=MagicMock(total_memory=8 * 1024 * 1024 * 1024))

# Import our module after mocking
from scripts.transcription import Transcriber, TranscriptionError


class TestTranscriptionAdvanced(unittest.TestCase):
    """Advanced test cases for the Transcriber class focusing on uncovered functionality."""

    def setUp(self):
        """Set up mock objects and common test data."""
        # Create a temporary directory for testing
        self.temp_dir = tempfile.TemporaryDirectory()
        self.temp_path = Path(self.temp_dir.name)
        
        # Create a mock audio file
        self.audio_path = self.temp_path / "test_audio.wav"
        with open(self.audio_path, "wb") as f:
            f.write(b"mock audio data")
        
        # Patch hardware detection to ensure predictable behavior
        self.cuda_patch = patch('torch.cuda.is_available', return_value=True)
        self.cuda_patch.start()
        
        # Create a transcriber instance for testing
        self.transcriber = Transcriber(
            enable_cache=True,
            cache_dir=str(self.temp_path / "cache"),
            chunk_length_seconds=30
        )
        
        # Replace the model with our mock for testing
        self.transcriber.model = MockWhisperModel()

    def tearDown(self):
        """Clean up after tests."""
        self.cuda_patch.stop()
        self.temp_dir.cleanup()
        self.transcriber.close()

    def test_should_chunk_audio(self):
        """Test the chunking decision logic."""
        # Test with short audio (no chunking)
        self.assertFalse(self.transcriber._should_chunk_audio(29))
        
        # Test with audio exactly at threshold (should chunk)
        self.assertTrue(self.transcriber._should_chunk_audio(30))
        
        # Test with longer audio (should chunk)
        self.assertTrue(self.transcriber._should_chunk_audio(120))

    @patch('os.path.getsize')
    @patch('librosa.get_duration')
    @patch('pydub.AudioSegment.from_file')
    def test_transcribe_with_chunking(self, mock_audio_segment, mock_get_duration, mock_getsize):
        """Test audio chunking and processing in _transcribe method."""
        # Configure mocks
        mock_getsize.return_value = 1000000  # 1MB
        mock_get_duration.return_value = 120  # 2 minutes
        
        # Mock AudioSegment
        mock_segment = MagicMock()
        mock_segment.duration_seconds = 120
        mock_segment.export.return_value.name = str(self.temp_path / "chunk0.wav")
        mock_audio_segment.return_value = mock_segment
        
        # Create mock chunk files that will be "found" by glob
        for i in range(4):
            chunk_path = self.temp_path / f"chunk{i}.wav"
            with open(chunk_path, "wb") as f:
                f.write(b"mock chunk data")
        
        # Patch glob to return our mocked chunks
        with patch('glob.glob', return_value=[
            str(self.temp_path / f"chunk{i}.wav") for i in range(4)
        ]):
            # Call the method
            result = self.transcriber._transcribe(str(self.audio_path), 120, "test_video")
            
            # Verify correct transcription was performed
            self.assertIn("Chunk 0 transcription part 1", result["text"])
            self.assertIn("Chunk 0 transcription part 2", result["text"])
            self.assertIn("Chunk 1 transcription part 1", result["text"])
            self.assertIn("Chunk 1 transcription part 2", result["text"])
            
            # Verify the model was called for each chunk
            self.assertEqual(len(self.transcriber.model.transcribe_calls), 4)

    @patch('os.path.getsize')
    @patch('librosa.get_duration')
    @patch('pydub.AudioSegment.from_file')
    def test_transcribe_without_chunking(self, mock_audio_segment, mock_get_duration, mock_getsize):
        """Test transcription without chunking for short audio."""
        # Configure mocks for short audio (under chunk threshold)
        mock_getsize.return_value = 100000  # 100KB
        mock_get_duration.return_value = 20  # 20 seconds
        
        # Call the method
        result = self.transcriber._transcribe(str(self.audio_path), 20, "test_video")
        
        # Verify transcription was done directly without chunking
        self.assertIn("Test transcription", result["text"])
        self.assertEqual(len(self.transcriber.model.transcribe_calls), 1)
        self.assertEqual(result["language"], "en")
        self.assertAlmostEqual(result["language_probability"], 0.95)

    @patch('time.sleep', return_value=None)
    def test_transcribe_with_temperature_fallback(self, mock_sleep):
        """Test temperature fallback logic when transcription fails."""
        # Make original WhisperModel transcribe method to simulate failure
        original_transcribe = self.transcriber.model.transcribe
        
        # Create a counter to track calls and simulate failure on specific temps
        call_count = 0
        
        def mock_transcribe_with_failures(*args, **kwargs):
            nonlocal call_count
            call_count += 1
            
            # Get temperature from kwargs
            temp = kwargs.get('temperature', 0.0)
            
            # Fail for lower temperatures
            if temp < 0.6:
                raise RuntimeError("Simulated transcription failure")
            
            # Otherwise succeed
            return original_transcribe(*args, **kwargs)
        
        # Apply our mocked transcribe method
        self.transcriber.model.transcribe = mock_transcribe_with_failures
        
        # Call the method with fallback
        result = self.transcriber._transcribe_with_temperature_fallback(
            str(self.audio_path), initial_prompt=None
        )
        
        # Verify fallback worked - should have tried multiple temperatures
        self.assertGreater(call_count, 1)
        self.assertIn("Test transcription", result[0])
        self.assertEqual(result[1].language, "en")

    @patch('builtins.open', new_callable=mock_open)
    @patch('json.dump')
    @patch('os.makedirs')
    def test_save_to_cache(self, mock_makedirs, mock_json_dump, mock_file):
        """Test saving transcription results to cache."""
        # Test data
        video_id = "test_video_123"
        offset = 0
        result = {
            "text": "Test transcription",
            "language": "en",
            "language_probability": 0.95,
            "model_name": "large-v3-turbo"
        }
        
        # Call the method
        self.transcriber._save_to_cache(video_id, offset, result)
        
        # Verify cache directory was created
        mock_makedirs.assert_called_once()
        
        # Verify file was opened for writing
        mock_file.assert_called_once()
        
        # Verify JSON was written with correct data
        mock_json_dump.assert_called_once()
        # Get the first positional argument (the data dict)
        saved_data = mock_json_dump.call_args[0][0]
        
        # Check the saved data includes our result plus timestamp
        self.assertEqual(saved_data["text"], "Test transcription")
        self.assertEqual(saved_data["language"], "en")
        self.assertIn("timestamp", saved_data)

    @patch('os.path.getsize')
    def test_monitor_download_progress(self, mock_getsize):
        """Test download progress monitoring."""
        # Setup progress callback mock
        progress_callback = MagicMock()
        
        # Simulate file size increasing over time
        file_sizes = [0, 1024 * 1024, 2 * 1024 * 1024, 4 * 1024 * 1024]
        mock_getsize.side_effect = file_sizes
        
        # Patch time.sleep to avoid actual delays
        with patch('time.sleep', return_value=None):
            # Call the method
            self.transcriber._monitor_download_progress(
                str(self.audio_path), 
                total_size=10 * 1024 * 1024,  # 10MB total
                progress_callback=progress_callback
            )
            
            # Verify progress callback was called with increasing percentages
            expected_progress = [0, 10, 20, 40]  # Percent based on file_sizes
            for i, progress in enumerate(expected_progress):
                self.assertAlmostEqual(
                    progress_callback.call_args_list[i][0][0],
                    progress,
                    delta=1  # Allow 1% difference due to rounding
                )

    @patch('torch.cuda.is_available')
    @patch('torch.cuda.get_device_properties')
    @patch('torch.cuda.get_device_capability')
    def test_optimize_device_and_compute_type(self, mock_capability, mock_properties, mock_is_available):
        """Test device and compute type optimization logic."""
        # Test cases with different hardware configurations
        test_cases = [
            # CUDA available, high-end GPU
            {
                'is_available': True,
                'total_memory': 16 * 1024 * 1024 * 1024,  # 16GB
                'capability': [8, 0],  # Ampere or newer
                'expected_device': 'cuda',
                'expected_compute_type': 'float16'
            },
            # CUDA available, older GPU
            {
                'is_available': True,
                'total_memory': 4 * 1024 * 1024 * 1024,  # 4GB
                'capability': [5, 0],  # Pascal or older
                'expected_device': 'cuda',
                'expected_compute_type': 'float32'  # Lower precision for older GPUs
            },
            # CUDA not available, fallback to CPU
            {
                'is_available': False,
                'total_memory': 0,
                'capability': [0, 0],
                'expected_device': 'cpu',
                'expected_compute_type': 'float32'
            }
        ]
        
        for case in test_cases:
            # Configure mocks for this test case
            mock_is_available.return_value = case['is_available']
            if case['is_available']:
                mock_properties.return_value = MagicMock(total_memory=case['total_memory'])
                mock_capability.return_value = case['capability']
            
            # Create a new transcriber instance
            transcriber = Transcriber()
            
            # Verify device and compute type were optimized correctly
            self.assertEqual(transcriber.device, case['expected_device'])
            self.assertEqual(transcriber.compute_type, case['expected_compute_type'])
            
    @patch('psutil.cpu_count')
    def test_optimize_cpu_threads(self, mock_cpu_count):
        """Test CPU thread optimization logic."""
        # Test cases with different CPU configurations
        test_cases = [
            # Many CPU cores (high-end)
            {
                'logical_cpus': 32,
                'physical_cpus': 16,
                'expected_threads': 8  # Half of physical CPUs
            },
            # Moderate CPU (mid-range)
            {
                'logical_cpus': 8,
                'physical_cpus': 4,
                'expected_threads': 4  # All physical CPUs
            },
            # Low CPU (minimum)
            {
                'logical_cpus': 2,
                'physical_cpus': 1,
                'expected_threads': 1
            }
        ]
        
        # Test with CPU device
        with patch('torch.cuda.is_available', return_value=False):
            for case in test_cases:
                # Configure CPU count mocks
                mock_cpu_count.side_effect = [
                    case['logical_cpus'],   # First call gets logical count
                    case['physical_cpus']   # Second call gets physical count
                ]
                
                # Create a new transcriber instance
                transcriber = Transcriber()
                
                # Verify CPU threads were optimized correctly
                self.assertEqual(transcriber.cpu_threads, case['expected_threads'])

    def test_optimize_batch_size(self):
        """Test batch size optimization for different configurations."""
        # Test cases with different device/memory configurations
        test_cases = [
            # High-end GPU
            {
                'device': 'cuda',
                'total_memory': 24 * 1024 * 1024 * 1024,  # 24GB
                'model_size': 'large-v3',
                'expected_batch_size': 32  # Maximum for high memory
            },
            # Mid-range GPU
            {
                'device': 'cuda',
                'total_memory': 8 * 1024 * 1024 * 1024,  # 8GB
                'model_size': 'large-v3',
                'expected_batch_size': 16  # Medium for average memory
            },
            # Low-end GPU
            {
                'device': 'cuda',
                'total_memory': 4 * 1024 * 1024 * 1024,  # 4GB
                'model_size': 'large-v3',
                'expected_batch_size': 8  # Lower for low memory
            },
            # CPU mode - should default to lower batch size
            {
                'device': 'cpu',
                'total_memory': 0,  # Not applicable
                'model_size': 'large-v3',
                'expected_batch_size': 8  # Default for CPU
            },
            # Small model - can use higher batch size
            {
                'device': 'cuda',
                'total_memory': 4 * 1024 * 1024 * 1024,  # 4GB
                'model_size': 'small',
                'expected_batch_size': 16  # Higher because model is smaller
            }
        ]
        
        for case in test_cases:
            # Mock device-specific functions
            with patch('torch.cuda.is_available', return_value=(case['device'] == 'cuda')):
                if case['device'] == 'cuda':
                    with patch('torch.cuda.get_device_properties') as mock_props:
                        # Set up the mock device properties
                        mock_device = MagicMock()
                        mock_device.total_memory = case['total_memory']
                        mock_props.return_value = mock_device
                        
                        # Create transcriber with specified model
                        transcriber = Transcriber(model_name=case['model_size'])
                        
                        # Verify batch size optimization
                        self.assertEqual(
                            transcriber._optimize_batch_size(case['model_size'], case['device']),
                            case['expected_batch_size']
                        )
                else:
                    # Test CPU mode
                    transcriber = Transcriber(model_name=case['model_size'], device='cpu')
                    self.assertEqual(
                        transcriber._optimize_batch_size(case['model_size'], 'cpu'),
                        case['expected_batch_size']
                    )


if __name__ == '__main__':
    unittest.main()