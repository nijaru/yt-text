package scripts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rs/zerolog"
)

type ScriptRunner struct {
	config Config
}

func NewScriptRunner(cfg Config) (*ScriptRunner, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	return &ScriptRunner{config: cfg}, nil
}

func validateConfig(cfg Config) error {
	// Verify scripts directory exists
	if _, err := os.Stat(cfg.ScriptsPath); os.IsNotExist(err) {
		return fmt.Errorf("scripts directory does not exist: %s", cfg.ScriptsPath)
	}

	// Verify required scripts exist
	requiredScripts := []string{"validate.py", "api.py"}
	for _, script := range requiredScripts {
		scriptPath := filepath.Join(cfg.ScriptsPath, script)
		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			return fmt.Errorf("required script not found: %s", scriptPath)
		}
	}
	return nil
}

func (r *ScriptRunner) RunScript(
	ctx context.Context,
	scriptName string,
	args map[string]string,
	flags []string,
) ([]byte, error) {
	const op = "ScriptRunner.RunScript"
	scriptPath := filepath.Join(r.config.ScriptsPath, scriptName)
	logger := zerolog.Ctx(ctx)

	logger.Debug().
		Str("script", scriptName).
		Interface("args", args).
		Interface("flags", flags).
		Msg("Executing script")

	cmdArgs := buildCommandArgs(scriptPath, args, flags)
	cmd := exec.CommandContext(ctx, r.config.PythonPath, cmdArgs...)
	cmd.Dir = r.config.ScriptsPath
	cmd.Env = buildEnvironment(r.config.Environment)

	output, err := r.executeCommand(cmd, logger)
	if err != nil {
		return nil, newScriptError(op, err, "script execution failed")
	}

	return output, nil
}

func buildCommandArgs(scriptPath string, args map[string]string, flags []string) []string {
	cmdArgs := []string{scriptPath}
	for k, v := range args {
		if v != "" {
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", k), v)
		}
	}
	for _, flag := range flags {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", flag))
	}
	return cmdArgs
}

func buildEnvironment(additionalEnv []string) []string {
	env := append(os.Environ(),
		"PYTORCH_CUDA_ALLOC_CONF=max_split_size_mb:512",
		"CUDA_LAUNCH_BLOCKING=1",
	)
	if len(additionalEnv) > 0 {
		env = append(env, additionalEnv...)
	}
	return env
}

func (r *ScriptRunner) executeCommand(cmd *exec.Cmd, logger *zerolog.Logger) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrOutput := stderr.String()
		logger.Error().
			Err(err).
			Str("stderr", stderrOutput).
			Msg("Script execution failed")
		return nil, fmt.Errorf("%v (stderr: %s)", err, stderrOutput)
	}

	output := stdout.Bytes()
	if err := validateJSONOutput(output); err != nil {
		logger.Error().
			Err(err).
			Str("output", string(output)).
			Msg("Invalid JSON output")
		return nil, err
	}

	return output, nil
}

func unmarshalResult(data []byte, v interface{}) error {
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to unmarshal result: %w", err)
	}
	return nil
}

func validateJSONOutput(output []byte) error {
	var jsonTest interface{}
	if err := json.Unmarshal(output, &jsonTest); err != nil {
		return fmt.Errorf("invalid JSON output: %v", err)
	}
	return nil
}
