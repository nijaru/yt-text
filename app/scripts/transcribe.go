package scripts

import (
	"context"

	"github.com/rs/zerolog"
)

func (r *ScriptRunner) Transcribe(
	ctx context.Context,
	url string,
	opts map[string]string,
	enableConstraints bool,
) (TranscriptionResult, error) {
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
		return result, newScriptError(op, err, "transcription failed")
	}

	if err := unmarshalResult(output, &result); err != nil {
		return result, newScriptError(op, err, "failed to parse transcription result")
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
