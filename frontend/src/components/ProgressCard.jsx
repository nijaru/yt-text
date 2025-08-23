import { Show, createSignal, onMount, onCleanup, createEffect } from 'solid-js';
import { Loader2 } from './Icons';

function ProgressCard(props) {
  const [elapsedTime, setElapsedTime] = createSignal(0);
  let startTime = Date.now();
  let timer;

  onMount(() => {
    // Update elapsed time every second
    timer = setInterval(() => {
      setElapsedTime(Math.floor((Date.now() - startTime) / 1000));
    }, 1000);
  });

  onCleanup(() => {
    if (timer) {
      clearInterval(timer);
      timer = null;
    }
  });
  
  // Stop timer if job fails
  createEffect(() => {
    if (props.status === 'failed' && timer) {
      clearInterval(timer);
      timer = null;
    }
  });

  const formatTime = (seconds) => {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins}:${secs.toString().padStart(2, '0')}`;
  };

  const getPhaseLabel = () => {
    switch (props.phase) {
      case 'downloading':
        return 'Downloading Audio';
      case 'transcribing':
        return 'Transcribing with AI';
      case 'finalizing':
        return 'Finalizing';
      default:
        return 'Processing';
    }
  };

  const getPhaseDescription = () => {
    switch (props.phase) {
      case 'downloading':
        return 'Extracting audio from video source...';
      case 'transcribing':
        return 'Processing audio with Whisper AI model...';
      case 'finalizing':
        return 'Saving transcription results...';
      default:
        return 'Preparing to process...';
    }
  };

  return (
    <div class="mt-8 glass-dark rounded-xl p-6">
      <div class="flex items-center justify-between mb-4">
        <div class="flex items-center space-x-3">
          <Loader2 class="w-5 h-5 text-white animate-spin" />
          <h3 class="text-lg font-semibold text-white">{getPhaseLabel()}</h3>
        </div>
        <span class="text-sm text-gray-400">
          {formatTime(elapsedTime())}
        </span>
      </div>
      
      {/* Phase Pipeline */}
      <div class="flex items-center space-x-2 mb-6">
        <div class="flex items-center space-x-2 flex-1">
          <div class={`w-8 h-8 rounded-full flex items-center justify-center text-xs font-medium transition-all ${
            props.phase === 'downloading' ? 'bg-blue-500 text-white scale-110' :
            props.phase === 'transcribing' || props.phase === 'finalizing' || props.phase === 'complete' ? 'bg-green-600 text-white' :
            'bg-gray-700 text-gray-400'
          }`}>
            1
          </div>
          <div class={`flex-1 h-1 rounded transition-all ${
            props.phase === 'transcribing' || props.phase === 'finalizing' || props.phase === 'complete' ? 'bg-green-600' : 'bg-gray-700'
          }`}></div>
        </div>
        
        <div class="flex items-center space-x-2 flex-1">
          <div class={`w-8 h-8 rounded-full flex items-center justify-center text-xs font-medium transition-all ${
            props.phase === 'transcribing' ? 'bg-purple-500 text-white scale-110' :
            props.phase === 'finalizing' || props.phase === 'complete' ? 'bg-green-600 text-white' :
            'bg-gray-700 text-gray-400'
          }`}>
            2
          </div>
          <div class={`flex-1 h-1 rounded transition-all ${
            props.phase === 'finalizing' || props.phase === 'complete' ? 'bg-green-600' : 'bg-gray-700'
          }`}></div>
        </div>
        
        <div class={`w-8 h-8 rounded-full flex items-center justify-center text-xs font-medium transition-all ${
          props.phase === 'finalizing' ? 'bg-green-500 text-white scale-110' :
          props.phase === 'complete' ? 'bg-green-600 text-white' :
          'bg-gray-700 text-gray-400'
        }`}>
          3
        </div>
      </div>
      
      {/* Animated Progress Bar */}
      <div class="w-full bg-gray-800 rounded-full h-2 overflow-hidden">
        <Show when={props.phase === 'downloading'}>
          <div class="h-full w-full relative bg-blue-900/30 overflow-hidden">
            <div class="absolute inset-0 bg-gradient-to-r from-transparent via-blue-400/60 to-transparent progress-shimmer" 
                 style="background-size: 200% 100%;"></div>
          </div>
        </Show>
        <Show when={props.phase === 'transcribing'}>
          <div class="h-full w-full relative bg-purple-900/30 overflow-hidden">
            <div class="absolute inset-0 bg-gradient-to-r from-transparent via-purple-400/60 to-transparent progress-shimmer" 
                 style="background-size: 200% 100%;"></div>
          </div>
        </Show>
        <Show when={props.phase === 'finalizing'}>
          <div class="h-full w-full relative bg-green-900/30 overflow-hidden">
            <div class="absolute inset-0 bg-gradient-to-r from-transparent via-green-400/60 to-transparent progress-shimmer" 
                 style="background-size: 200% 100%;"></div>
          </div>
        </Show>
        <Show when={props.phase === 'complete'}>
          <div class="h-full w-full bg-green-500"></div>
        </Show>
        <Show when={props.status === 'failed'}>
          <div class="h-full w-full bg-red-500"></div>
        </Show>
      </div>
      
      <div class="mt-4 text-center">
        <p class="text-gray-300 text-base">{getPhaseDescription()}</p>
        <Show when={props.phase === 'downloading'}>
          <p class="text-sm text-gray-500 mt-1">This may take a moment depending on video size</p>
        </Show>
        <Show when={props.phase === 'transcribing'}>
          <p class="text-sm text-gray-500 mt-1">Processing time varies by audio length and model</p>
        </Show>
      </div>
    </div>
  );
}

export default ProgressCard;