import { createSignal, createEffect, Show, For } from 'solid-js';
import { 
  FileAudio, 
  Link, 
  Zap, 
  ShieldCheck, 
  Sparkles,
  Copy,
  Download,
  RefreshCw,
  AlertCircle,
  Loader2,
  Github,
  BookOpen,
  Moon,
  ChevronDown
} from './components/Icons';
import TranscriptionForm from './components/TranscriptionForm';
import ProgressCard from './components/ProgressCard';
import ResultCard from './components/ResultCard';
import { transcriptionService } from './services/api';

function App() {
  const [url, setUrl] = createSignal('');
  const [model, setModel] = createSignal('base');
  const [language, setLanguage] = createSignal('');
  const [loading, setLoading] = createSignal(false);
  const [jobId, setJobId] = createSignal(null);
  const [status, setStatus] = createSignal('');
  const [phase, setPhase] = createSignal('queued');
  const [progress, setProgress] = createSignal(0);
  const [result, setResult] = createSignal(null);
  const [error, setError] = createSignal(null);
  const [ws, setWs] = createSignal(null);

  // Handle form submission
  const handleSubmit = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError(null);
    setResult(null);
    setProgress(0);
    setStatus('Initializing...');

    try {
      const response = await transcriptionService.submitJob({
        url: url(),
        model: model(),
        language: language() || null,
      });

      setJobId(response.job_id);
      setStatus(response.status);
      
      // Connect WebSocket for real-time updates
      connectWebSocket(response.job_id);
    } catch (err) {
      setError(err.message || 'Failed to start transcription');
      setLoading(false);
    }
  };

  // WebSocket connection for real-time updates
  const connectWebSocket = (jobId) => {
    const websocket = new WebSocket(`ws://localhost:8000/ws/jobs/${jobId}`);
    
    websocket.onmessage = (event) => {
      const data = JSON.parse(event.data);
      
      if (data.type === 'status_update') {
        setStatus(data.status);
        setPhase(data.phase);
        setProgress(data.progress || 0);
        
        // Check if job failed
        if (data.status === 'failed') {
          setError('Transcription failed. Please try again.');
          setLoading(false);
          setPhase('queued');
          websocket.close();
        }
      } else if (data.type === 'result') {
        fetchResult(jobId);
      } else if (data.type === 'error') {
        setError(data.error || 'Transcription failed');
        setLoading(false);
        setPhase('queued');
        websocket.close();
      }
    };
    
    websocket.onerror = () => {
      // Fallback to polling if WebSocket fails
      pollStatus(jobId);
    };
    
    websocket.onclose = () => {
      setWs(null);
    };
    
    setWs(websocket);
  };

  // Polling fallback
  const pollStatus = async (jobId) => {
    const poll = async () => {
      try {
        const data = await transcriptionService.getJobStatus(jobId);
        
        setStatus(data.status);
        setProgress(data.progress || 0);
        
        if (data.status === 'completed') {
          await fetchResult(jobId);
        } else if (data.status === 'failed') {
          setError(data.error || 'Transcription failed');
          setLoading(false);
        } else {
          // Continue polling
          setTimeout(poll, 2000);
        }
      } catch (err) {
        setError('Failed to check status');
        setLoading(false);
      }
    };
    
    poll();
  };

  // Fetch transcription result
  const fetchResult = async (jobId) => {
    try {
      const data = await transcriptionService.getJobResult(jobId);
      setResult(data);
      setLoading(false);
      setProgress(100);
      setStatus('Complete');
      
      if (ws()) {
        ws().close();
      }
    } catch (err) {
      setError(err.message || 'Failed to fetch result');
      setLoading(false);
    }
  };

  // Reset form
  const reset = () => {
    setUrl('');
    setJobId(null);
    setStatus('');
    setPhase('queued');
    setProgress(0);
    setResult(null);
    setError(null);
    setLoading(false);
    
    if (ws()) {
      ws().close();
    }
  };

  // Cleanup WebSocket on unmount
  createEffect(() => {
    return () => {
      if (ws()) {
        ws().close();
      }
    };
  });

  const features = [
    {
      icon: Zap,
      title: 'Lightning Fast',
      description: 'Optimized for speed with multiple backend options',
      color: 'purple'
    },
    {
      icon: ShieldCheck,
      title: 'Privacy First',
      description: 'Your data is processed locally and never stored',
      color: 'blue'
    },
    {
      icon: Sparkles,
      title: 'AI Powered',
      description: 'State-of-the-art models for accurate transcriptions',
      color: 'green'
    }
  ];

  return (
    <div class="min-h-screen bg-gray-950 relative">
      {/* Background gradient */}
      <div class="fixed inset-0 gradient-animated opacity-20 pointer-events-none"></div>
      
      <div class="relative flex flex-col min-h-screen">
        {/* Navigation */}
        <nav class="glass-dark border-b border-white/5">
          <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
            <div class="flex items-center justify-between h-20">
              <div class="flex items-center space-x-3">
                <div class="w-10 h-10 rounded-lg gradient-animated flex items-center justify-center">
                  <FileAudio class="w-5 h-5 text-white" />
                </div>
                <div>
                  <h1 class="text-2xl font-bold text-white">yt-text</h1>
                  <p class="text-sm text-gray-400">AI Transcription</p>
                </div>
              </div>
              
              <nav class="flex items-center space-x-2" role="navigation" aria-label="Main navigation">
                <a href="/docs" class="p-3 text-gray-300 hover:text-white hover:bg-white/10 rounded-lg transition-all focus:outline-none focus:ring-2 focus:ring-purple-500/50" aria-label="Documentation">
                  <BookOpen class="w-5 h-5" />
                </a>
                <a href="https://github.com/nijaru/yt-text" target="_blank" rel="noopener noreferrer" class="p-3 text-gray-300 hover:text-white hover:bg-white/10 rounded-lg transition-all focus:outline-none focus:ring-2 focus:ring-purple-500/50" aria-label="GitHub repository">
                  <Github class="w-5 h-5" />
                </a>
                <button class="p-3 text-gray-300 hover:text-white hover:bg-white/10 rounded-lg transition-all focus:outline-none focus:ring-2 focus:ring-purple-500/50" aria-label="Toggle theme">
                  <Moon class="w-5 h-5" />
                </button>
              </nav>
            </div>
          </div>
        </nav>
        
        {/* Main content */}
        <main class="flex-1 flex items-center justify-center p-4 pt-16">
          <div class="w-full max-w-4xl">
            {/* Hero section */}
            <div class="text-center mb-12">
              <h2 class="text-4xl md:text-5xl font-bold text-white mb-4">
                Transform Videos into
                <span class="block text-transparent bg-clip-text gradient-animated">
                  Accurate Text
                </span>
              </h2>
              <p class="text-xl text-gray-400 max-w-2xl mx-auto">
                Powered by state-of-the-art AI models including Whisper, MLX, and more.
                Just paste a URL and get instant transcriptions.
              </p>
            </div>
            
            {/* Main card */}
            <div class="glass rounded-2xl p-6 md:p-8 shadow-2xl">
              <Show when={!result()} fallback={
                <ResultCard 
                  result={result()} 
                  jobId={jobId()}
                  onReset={reset}
                />
              }>
                <TranscriptionForm
                  url={url()}
                  setUrl={setUrl}
                  model={model()}
                  setModel={setModel}
                  language={language()}
                  setLanguage={setLanguage}
                  loading={loading()}
                  onSubmit={handleSubmit}
                />
                
                <Show when={jobId() && !result()}>
                  <ProgressCard
                    status={status()}
                    phase={phase()}
                    progress={progress()}
                    jobId={jobId()}
                  />
                </Show>
                
                <Show when={error()}>
                  <div class="mt-8 bg-red-900/20 border border-red-500/50 rounded-xl p-6">
                    <div class="flex items-start space-x-3">
                      <AlertCircle class="w-5 h-5 text-red-500 mt-1 flex-shrink-0" />
                      <div class="flex-1">
                        <h3 class="text-red-400 font-semibold mb-1">Error</h3>
                        <p class="text-gray-300">{error()}</p>
                      </div>
                    </div>
                  </div>
                </Show>
              </Show>
            </div>
            
            {/* Features Grid */}
            <div class="grid grid-cols-1 md:grid-cols-3 gap-6 mt-12">
              <For each={features}>
                {(feature) => (
                  <div class="glass rounded-xl p-6 text-center">
                    <div class={`w-12 h-12 mx-auto mb-4 rounded-lg bg-${feature.color}-600/20 flex items-center justify-center`}>
                      <feature.icon class={`w-6 h-6 text-${feature.color}-400`} />
                    </div>
                    <h3 class="text-white font-semibold mb-2">{feature.title}</h3>
                    <p class="text-gray-400 text-base">{feature.description}</p>
                  </div>
                )}
              </For>
            </div>
          </div>
        </main>
        
        {/* Footer */}
        <footer class="glass-dark border-t border-white/5 py-6 mt-12">
          <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
            <div class="flex flex-col md:flex-row items-center justify-between">
              <p class="text-gray-400 text-base">
                © 2024 yt-text. Licensed under AGPL-3.0
              </p>
              <div class="flex items-center space-x-6 mt-4 md:mt-0">
                <a href="/docs" class="text-gray-400 hover:text-white text-base transition-colors">
                  API Docs
                </a>
                <a href="https://github.com/nijaru/yt-text" class="text-gray-400 hover:text-white text-base transition-colors">
                  GitHub
                </a>
                <span class="text-gray-400 text-base">v2.0.0</span>
              </div>
            </div>
          </div>
        </footer>
      </div>
    </div>
  );
}

export default App;