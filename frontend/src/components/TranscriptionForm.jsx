import { Link, Wand2, Loader2 } from './Icons';

const models = [
  { value: 'tiny', label: 'Tiny (Fastest)' },
  { value: 'base', label: 'Base (Balanced)' },
  { value: 'small', label: 'Small (Better)' },
  { value: 'medium', label: 'Medium (Accurate)' },
  { value: 'large', label: 'Large (Most Accurate)' },
];

const languages = [
  { value: '', label: 'Auto-detect' },
  { value: 'en', label: 'English' },
  { value: 'es', label: 'Spanish' },
  { value: 'fr', label: 'French' },
  { value: 'de', label: 'German' },
  { value: 'it', label: 'Italian' },
  { value: 'pt', label: 'Portuguese' },
  { value: 'ru', label: 'Russian' },
  { value: 'ja', label: 'Japanese' },
  { value: 'ko', label: 'Korean' },
  { value: 'zh', label: 'Chinese' },
];

function TranscriptionForm(props) {
  return (
    <form onSubmit={props.onSubmit} class="space-y-6">
      {/* URL Input */}
      <div>
        <label class="block text-sm font-medium text-gray-300 mb-2">
          Video URL
        </label>
        <div class="relative">
          <input
            type="url"
            value={props.url}
            onInput={(e) => props.setUrl(e.target.value)}
            placeholder="https://www.youtube.com/watch?v=..."
            required
            disabled={props.loading}
            class="input-field pl-10"
          />
          <Link class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500" />
        </div>
        <p class="mt-2 text-xs text-gray-500">
          Supports YouTube, Vimeo, Twitter, TikTok, and 1000+ other sites
        </p>
      </div>
      
      {/* Model and Language Selection */}
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div>
          <label class="block text-sm font-medium text-gray-300 mb-2">
            AI Model
          </label>
          <select
            value={props.model}
            onInput={(e) => props.setModel(e.target.value)}
            disabled={props.loading}
            class="select-field"
          >
            {models.map(model => (
              <option value={model.value}>{model.label}</option>
            ))}
          </select>
        </div>
        
        <div>
          <label class="block text-sm font-medium text-gray-300 mb-2">
            Language
          </label>
          <select
            value={props.language}
            onInput={(e) => props.setLanguage(e.target.value)}
            disabled={props.loading}
            class="select-field"
          >
            {languages.map(lang => (
              <option value={lang.value}>{lang.label}</option>
            ))}
          </select>
        </div>
      </div>
      
      {/* Submit Button */}
      <button
        type="submit"
        disabled={props.loading}
        class="w-full btn-primary"
      >
        {props.loading ? (
          <span class="flex items-center justify-center space-x-2">
            <Loader2 class="w-5 h-5 animate-spin" />
            <span>Processing...</span>
          </span>
        ) : (
          <span class="flex items-center justify-center space-x-2">
            <Wand2 class="w-5 h-5" />
            <span>Start Transcription</span>
          </span>
        )}
      </button>
    </form>
  );
}

export default TranscriptionForm;