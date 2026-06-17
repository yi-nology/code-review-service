package errors

import (
	stderrors "errors"
	"fmt"
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

func NotFound(msg string) *AppError {
	return &AppError{Code: CodeNotFound, Message: msg}
}

func InvalidState(msg string) *AppError {
	return &AppError{Code: CodeInvalidState, Message: msg}
}

func Validation(msg string) *AppError {
	return &AppError{Code: CodeValidation, Message: msg}
}

func Internal(msg string, cause error) *AppError {
	return &AppError{Code: CodeInternal, Message: msg, Cause: cause}
}

func IsAppError(err error) bool {
	var appErr *AppError
	return stderrors.As(err, &appErr)
}

func GetCode(err error) Code {
	var appErr *AppError
	if stderrors.As(err, &appErr) {
		return appErr.Code
	}
	return CodeInternal
}
