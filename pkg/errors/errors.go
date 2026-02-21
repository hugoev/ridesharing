// Package apperrors provides custom error types with HTTP status mapping
// and machine-readable error codes for API consumers.
package apperrors

import (
	"fmt"
	"net/http"
)

// ErrorCode is a machine-readable string identifying the error type.
// API consumers can switch on this value for programmatic error handling.
type ErrorCode string

// Defined error codes — grouped by domain.
const (
	// General
	CodeInternal   ErrorCode = "INTERNAL_ERROR"
	CodeBadRequest ErrorCode = "BAD_REQUEST"
	CodeValidation ErrorCode = "VALIDATION_ERROR"

	// Auth
	CodeUnauthorized       ErrorCode = "UNAUTHORIZED"
	CodeForbidden          ErrorCode = "FORBIDDEN"
	CodeUserExists         ErrorCode = "USER_ALREADY_EXISTS"
	CodeInvalidCredentials ErrorCode = "INVALID_CREDENTIALS"

	// Ride
	CodeRideNotFound       ErrorCode = "RIDE_NOT_FOUND"
	CodeRideAlreadyActive  ErrorCode = "RIDE_ALREADY_ACTIVE"
	CodeInvalidTransition  ErrorCode = "INVALID_RIDE_TRANSITION"
	CodeNoDriversAvailable ErrorCode = "NO_DRIVERS_AVAILABLE"

	// Payment
	CodePaymentFailed   ErrorCode = "PAYMENT_FAILED"
	CodePaymentNotFound ErrorCode = "PAYMENT_NOT_FOUND"
	CodeAlreadyCharged  ErrorCode = "ALREADY_CHARGED"

	// User
	CodeUserNotFound ErrorCode = "USER_NOT_FOUND"

	// Location
	CodeInvalidCoordinates ErrorCode = "INVALID_COORDINATES"

	// Rate limiting
	CodeRateLimited   ErrorCode = "RATE_LIMITED"
	CodeAccountLocked ErrorCode = "ACCOUNT_LOCKED"

	// Request
	CodeBodyTooLarge ErrorCode = "BODY_TOO_LARGE"
)

// AppError represents an application-level error with an HTTP status code
// and a machine-readable error code.
type AppError struct {
	Code      int       `json:"code"`
	ErrorCode ErrorCode `json:"error_code"`
	Message   string    `json:"error"`
	Err       error     `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// --- Constructors ---

func NotFound(msg string) *AppError {
	return &AppError{Code: http.StatusNotFound, ErrorCode: CodeRideNotFound, Message: msg}
}

func NotFoundWithCode(code ErrorCode, msg string) *AppError {
	return &AppError{Code: http.StatusNotFound, ErrorCode: code, Message: msg}
}

func BadRequest(msg string) *AppError {
	return &AppError{Code: http.StatusBadRequest, ErrorCode: CodeBadRequest, Message: msg}
}

func BadRequestWithCode(code ErrorCode, msg string) *AppError {
	return &AppError{Code: http.StatusBadRequest, ErrorCode: code, Message: msg}
}

func Unauthorized(msg string) *AppError {
	return &AppError{Code: http.StatusUnauthorized, ErrorCode: CodeUnauthorized, Message: msg}
}

func Forbidden(msg string) *AppError {
	return &AppError{Code: http.StatusForbidden, ErrorCode: CodeForbidden, Message: msg}
}

func Conflict(msg string) *AppError {
	return &AppError{Code: http.StatusConflict, ErrorCode: CodeUserExists, Message: msg}
}

func ConflictWithCode(code ErrorCode, msg string) *AppError {
	return &AppError{Code: http.StatusConflict, ErrorCode: code, Message: msg}
}

func Internal(msg string, err error) *AppError {
	return &AppError{Code: http.StatusInternalServerError, ErrorCode: CodeInternal, Message: msg, Err: err}
}

// Wrap attaches an underlying error to an AppError.
func Wrap(appErr *AppError, err error) *AppError {
	return &AppError{
		Code:      appErr.Code,
		ErrorCode: appErr.ErrorCode,
		Message:   appErr.Message,
		Err:       err,
	}
}

func TooManyRequests(msg string) *AppError {
	return &AppError{Code: http.StatusTooManyRequests, ErrorCode: CodeAccountLocked, Message: msg}
}
