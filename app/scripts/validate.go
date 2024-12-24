package scripts

import (
	"context"
)

func (r *ScriptRunner) Validate(ctx context.Context, url string) (VideoInfo, error) {
	const op = "ScriptRunner.Validate"
	var result VideoInfo

	output, err := r.runScript(ctx, "validate.py", map[string]string{
		"url": url,
	}, nil)
	if err != nil {
		return result, newScriptError(op, err, "validation failed")
	}

	if err := unmarshalResult(output, &result); err != nil {
		return result, newScriptError(op, err, "failed to parse validation result")
	}

	return result, nil
}
