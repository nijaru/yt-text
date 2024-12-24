package scripts

import "fmt"

type ScriptError struct {
	Op      string
	Err     error
	Message string
}

func (e *ScriptError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Message)
}

func (e *ScriptError) Unwrap() error {
	return e.Err
}

func newScriptError(op string, err error, message string) *ScriptError {
	return &ScriptError{
		Op:      op,
		Err:     err,
		Message: message,
	}
}
