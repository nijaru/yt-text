package scripts

import (
	"context"

	"github.com/rs/zerolog"
)

// Transcribe uses Whisper model to transcribe a video
func (r *ScriptRunner) Transcribe(
	ctx context.Context,
	url string,
	opts map[string]string,
	enableConstraints bool,
) (*TranscriptionResult, error) {
	const op = "ScriptRunner.Transcribe"
	var result TranscriptionResult

	logger := zerolog.Ctx(ctx)
	logger.Debug().
		Str("url", url).
		Interface("opts", opts).
		Msg("Starting transcription")

	args := buildTranscribeArgs(url, opts)
	flags := buildTranscribeFlags(enableConstraints)

	output, err := r.runScript(ctx, "api.py", args, flags)
	if err != nil {
		return nil, newScriptError(op, err, "transcription failed")
	}

	if err := unmarshalResult(output, &result); err != nil {
		return nil, newScriptError(op, err, "failed to parse transcription result")
	}
	
	// Set source to Whisper if not specified
	if result.Source == "" {
		result.Source = "whisper"
	}

	return &result, nil
}

// FetchYouTubeCaptions attempts to get captions from YouTube API
func (r *ScriptRunner) FetchYouTubeCaptions(
	ctx context.Context,
	videoID string,
	apiKey string,
) (*TranscriptionResult, error) {
	const op = "ScriptRunner.FetchYouTubeCaptions"
	
	logger := zerolog.Ctx(ctx)
	logger.Debug().
		Str("videoID", videoID).
		Msg("Fetching YouTube captions")

	args := map[string]string{
		"video_id": videoID,
		"api_key":  apiKey,
	}

	// Use the YouTube captions script
	output, err := r.runScript(ctx, "youtube_captions.py", args, nil)
	if err != nil {
		return nil, newScriptError(op, err, "failed to fetch YouTube captions")
	}

	// Parse the result into a temporary YouTubeCaptionResult first
	var captionResult YouTubeCaptionResult
	if err := unmarshalResult(output, &captionResult); err != nil {
		return nil, newScriptError(op, err, "failed to parse YouTube captions result")
	}
	
	// Check if there was an error
	if captionResult.Error != "" {
		return nil, newScriptError(op, nil, captionResult.Error)
	}
	
	// Convert to TranscriptionResult format
	var title, language *string
	if captionResult.Title != "" {
		title = &captionResult.Title
	}
	if captionResult.Language != "" {
		language = &captionResult.Language
	}
	
	// Create and return a TranscriptionResult
	result := &TranscriptionResult{
		Text:     captionResult.Transcription,
		Title:    title,
		Language: language,
		Source:   "youtube_api",
	}

	return result, nil
}

func buildTranscribeArgs(url string, opts map[string]string) map[string]string {
	args := map[string]string{"url": url}
	for k, v := range opts {
		args[k] = v
	}
	return args
}

func buildTranscribeFlags(enableConstraints bool) []string {
	var flags []string
	if enableConstraints {
		flags = append(flags, "enable_constraints")
	}
	return flags
}
