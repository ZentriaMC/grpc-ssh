package main

import "fmt"

type ExitError struct {
	cause error
	code  int
}

func NewExitError(cause error, code int) error {
	return &ExitError{cause: cause, code: code}
}

func (e *ExitError) Error() string {
	if e.cause == nil {
		return fmt.Sprintf("exit %d", e.code)
	}
	return fmt.Sprintf("exit %d: %s", e.code, e.cause)
}

func (e *ExitError) Unwrap() error {
	return e.cause
}
