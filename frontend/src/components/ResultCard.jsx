import { Copy, Download, RefreshCw } from './Icons';
import { createSignal } from 'solid-js';

function ResultCard(props) {
  const [copied, setCopied] = createSignal(false);
  
  const formatDuration = (seconds) => {
    if (!seconds) return '0:00';
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const secs = Math.floor(seconds % 60);
    
    if (hours > 0) {
      return `${hours}:${minutes.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
    }
    return `${minutes}:${secs.toString().padStart(2, '0')}`;
  };
  
  const copyToClipboard = async () => {
    if (props.result?.text) {
      await navigator.clipboard.writeText(props.result.text);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };
  
  const downloadText = () => {
    if (props.result?.text) {
      const blob = new Blob([props.result.text], { type: 'text/plain' });
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `transcription-${props.jobId?.slice(0, 8)}.txt`;
      a.click();
      window.URL.revokeObjectURL(url);
    }
  };
  
  return (
    <div class="space-y-6">
      <div class="glass-dark rounded-xl p-6">
        <div class="flex items-center justify-between mb-4">
          <h3 class="text-lg font-semibold text-white">Transcription Complete</h3>
          <button
            onClick={props.onReset}
            class="px-4 py-2 bg-purple-600 hover:bg-purple-700 text-white rounded-lg transition-colors flex items-center space-x-2"
          >
            <RefreshCw class="w-4 h-4" />
            <span>New Transcription</span>
          </button>
        </div>
        
        {/* Metadata */}
        <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
          <div class="text-center">
            <p class="text-gray-400 text-sm">Duration</p>
            <p class="text-white font-semibold">
              {formatDuration(props.result?.duration)}
            </p>
          </div>
          <div class="text-center">
            <p class="text-gray-400 text-sm">Words</p>
            <p class="text-white font-semibold">{props.result?.word_count}</p>
          </div>
          <div class="text-center">
            <p class="text-gray-400 text-sm">Language</p>
            <p class="text-white font-semibold">
              {props.result?.language?.toUpperCase() || 'Unknown'}
            </p>
          </div>
          <div class="text-center">
            <p class="text-gray-400 text-sm">Model</p>
            <p class="text-white font-semibold">{props.result?.model_used}</p>
          </div>
        </div>
        
        {/* Transcription Text */}
        <div class="bg-gray-800/30 rounded-lg p-6 max-h-[32rem] overflow-y-auto border border-gray-700/30">
          <p class="text-gray-100 whitespace-pre-wrap leading-relaxed text-base font-sans selection:bg-purple-600/30">
            {props.result?.text}
          </p>
        </div>
      </div>
      
      {/* Secondary Actions */}
      <div class="flex space-x-3">
        <button
          onClick={copyToClipboard}
          class="flex-1 px-4 py-3 bg-gray-700 hover:bg-gray-600 text-white rounded-lg transition-colors flex items-center justify-center space-x-2"
        >
          <Copy class="w-4 h-4" />
          <span>{copied() ? 'Copied!' : 'Copy Text'}</span>
        </button>
        <button
          onClick={downloadText}
          class="flex-1 px-4 py-3 bg-gray-700 hover:bg-gray-600 text-white rounded-lg transition-colors flex items-center justify-center space-x-2"
        >
          <Download class="w-4 h-4" />
          <span>Download TXT</span>
        </button>
      </div>
    </div>
  );
}

export default ResultCard;