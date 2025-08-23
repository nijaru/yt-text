// API service for transcription endpoints

const API_BASE = '/api';

class TranscriptionService {
  async submitJob(data) {
    const response = await fetch(`${API_BASE}/transcribe`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(data),
    });
    
    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.detail || 'Failed to start transcription');
    }
    
    return response.json();
  }
  
  async getJobStatus(jobId) {
    const response = await fetch(`${API_BASE}/jobs/${jobId}`);
    
    if (!response.ok) {
      throw new Error('Failed to get job status');
    }
    
    return response.json();
  }
  
  async getJobResult(jobId) {
    const response = await fetch(`${API_BASE}/jobs/${jobId}/result`);
    
    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.detail || 'Failed to get result');
    }
    
    return response.json();
  }
  
  async retryJob(jobId) {
    const response = await fetch(`${API_BASE}/jobs/${jobId}/retry`, {
      method: 'POST',
    });
    
    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.detail || 'Failed to retry job');
    }
    
    return response.json();
  }
}

export const transcriptionService = new TranscriptionService();