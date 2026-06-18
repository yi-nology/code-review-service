package errors

import (
	"fmt"
	stderrors "errors"
)

type Code int

const (
	CodeNotFound     Code = 404
	CodeInvalidState Code = 409
	CodeValidation   Code = 422
	CodeInternal     Code = 500
)

type AppError struct {
	Code    Code
	Message string
	Cause   error
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

func NewAppError(code Code, msg string) *AppError {
	return &AppError{Code: code, Message: msg}
}

func IsAppError(err error) bool {
	var appErr *AppError
	return stderrors.As(err, &appErr)
}
