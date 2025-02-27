// Package http provides standardized HTTP utilities for the TXN project
package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/baely/txn/internal/common/errors"
)

// Response is a standardized API response
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// JSON writes a JSON response
func JSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	}
}

// Success writes a successful JSON response
func Success(w http.ResponseWriter, data interface{}) {
	response := Response{
		Success: true,
		Data:    data,
	}
	JSON(w, http.StatusOK, response)
}

// Error writes an error JSON response
func Error(w http.ResponseWriter, err error, statusCode int) {
	response := Response{
		Success: false,
		Error:   err.Error(),
	}
	JSON(w, statusCode, response)
}

// HandleError determines the appropriate status code based on error type
func HandleError(w http.ResponseWriter, err error) {
	statusCode := http.StatusInternalServerError

	switch {
	case errors.Is(err, errors.ErrNotFound):
		statusCode = http.StatusNotFound
	case errors.Is(err, errors.ErrInvalidInput):
		statusCode = http.StatusBadRequest
	case errors.Is(err, errors.ErrUnauthorized):
		statusCode = http.StatusUnauthorized
	case errors.Is(err, errors.ErrAlreadyExists):
		statusCode = http.StatusConflict
	}

	Error(w, err, statusCode)
}

// NewRouter creates a new Chi router with standard middleware
func NewRouter() *chi.Mux {
	r := chi.NewRouter()

	// Standard middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	return r
}
