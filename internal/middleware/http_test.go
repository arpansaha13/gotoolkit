package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/arpansaha13/gotoolkit/internal/domain"
)

func TestHttpRecoveryMiddleware_WithPanic(t *testing.T) {
	handler := HttpRecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var errResp ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(t, err)
	assert.Equal(t, "Something went wrong!", errResp.Message)
	assert.Equal(t, "INTERNAL_ERROR", errResp.Code)
}

func TestHttpRecoveryMiddleware_WithPanicInt(t *testing.T) {
	handler := HttpRecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(42)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHttpRecoveryMiddleware_NoError(t *testing.T) {
	handler := HttpRecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHttpErrorMiddleware_ValidationError(t *testing.T) {
	handler := HttpErrorMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(&domain.ValidationError{Message: "invalid email", Field: "email"})
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var errResp ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(t, err)
	assert.Equal(t, "VALIDATION_ERROR", errResp.Code)
	assert.Contains(t, errResp.Message, "invalid email")
}

func TestHttpErrorMiddleware_ConflictError(t *testing.T) {
	handler := HttpErrorMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(&domain.ConflictError{Message: "email already exists"})
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	var errResp ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(t, err)
	assert.Equal(t, "CONFLICT_ERROR", errResp.Code)
}

func TestHttpErrorMiddleware_NotFoundError(t *testing.T) {
	handler := HttpErrorMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(&domain.NotFoundError{Message: "user not found"})
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	var errResp ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(t, err)
	assert.Equal(t, "NOT_FOUND_ERROR", errResp.Code)
}

func TestHttpErrorMiddleware_UnauthorizedError(t *testing.T) {
	handler := HttpErrorMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(&domain.UnauthorizedError{Message: "invalid credentials"})
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var errResp ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(t, err)
	assert.Equal(t, "UNAUTHORIZED_ERROR", errResp.Code)
}

func TestHttpErrorMiddleware_ForbiddenError(t *testing.T) {
	handler := HttpErrorMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(&domain.ForbiddenError{Message: "insufficient permissions"})
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	var errResp ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(t, err)
	assert.Equal(t, "FORBIDDEN_ERROR", errResp.Code)
	assert.Contains(t, errResp.Message, "insufficient permissions")
}

func TestHttpErrorMiddleware_InternalError(t *testing.T) {
	handler := HttpErrorMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(&domain.InternalError{Message: "database connection failed"})
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var errResp ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.NoError(t, err)
	assert.Equal(t, "INTERNAL_ERROR", errResp.Code)
	assert.Equal(t, "Something went wrong!", errResp.Message) // Generic message for 500s
}

func TestHttpErrorMiddleware_NonErrorPanic(t *testing.T) {
	handler := HttpErrorMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("string panic")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHttpErrorMiddleware_NoError(t *testing.T) {
	handler := HttpErrorMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHttpWriteErrorWithContext_ValidationError(t *testing.T) {
	w := httptest.NewRecorder()
	ctx := context.Background()
	err := &domain.ValidationError{Message: "test error", Field: "name"}

	HttpWriteErrorWithContext(w, ctx, err)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.Equal(t, "VALIDATION_ERROR", errResp.Code)
}
