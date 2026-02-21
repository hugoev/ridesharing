package apperrors

import (
	"errors"
	"net/http"
	"testing"
)

func TestAppError_Error(t *testing.T) {
	err := BadRequest("invalid input")
	if err.Error() != "invalid input" {
		t.Errorf("Error() = %q, want %q", err.Error(), "invalid input")
	}
}

func TestAppError_ErrorWithWrapped(t *testing.T) {
	inner := errors.New("connection refused")
	err := Internal("database error", inner)

	expected := "database error: connection refused"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

func TestAppError_Unwrap(t *testing.T) {
	inner := errors.New("original error")
	err := Internal("wrapped", inner)

	if !errors.Is(err, inner) {
		t.Error("Unwrap should return the inner error")
	}
}

func TestNotFound(t *testing.T) {
	err := NotFound("user not found")
	if err.Code != http.StatusNotFound {
		t.Errorf("Code = %d, want %d", err.Code, http.StatusNotFound)
	}
}

func TestBadRequest(t *testing.T) {
	err := BadRequest("missing field")
	if err.Code != http.StatusBadRequest {
		t.Errorf("Code = %d, want %d", err.Code, http.StatusBadRequest)
	}
}

func TestUnauthorized(t *testing.T) {
	err := Unauthorized("invalid credentials")
	if err.Code != http.StatusUnauthorized {
		t.Errorf("Code = %d, want %d", err.Code, http.StatusUnauthorized)
	}
}

func TestForbidden(t *testing.T) {
	err := Forbidden("no access")
	if err.Code != http.StatusForbidden {
		t.Errorf("Code = %d, want %d", err.Code, http.StatusForbidden)
	}
}

func TestConflict(t *testing.T) {
	err := Conflict("already exists")
	if err.Code != http.StatusConflict {
		t.Errorf("Code = %d, want %d", err.Code, http.StatusConflict)
	}
}

func TestInternal(t *testing.T) {
	err := Internal("server error", nil)
	if err.Code != http.StatusInternalServerError {
		t.Errorf("Code = %d, want %d", err.Code, http.StatusInternalServerError)
	}
}

func TestWrap(t *testing.T) {
	original := NotFound("not found")
	inner := errors.New("db error")
	wrapped := Wrap(original, inner)

	if wrapped.Code != http.StatusNotFound {
		t.Errorf("Code = %d, want %d", wrapped.Code, http.StatusNotFound)
	}
	if wrapped.Err != inner {
		t.Error("Wrap should preserve the inner error")
	}
}
