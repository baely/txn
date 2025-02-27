// Package server provides the HTTP server infrastructure for the TXN application
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/hostrouter"

	"github.com/baely/txn/internal/common/errors"
)

// Server represents the HTTP server for the application
type Server struct {
	*http.Server
	hostRouter hostrouter.Routes
	logger     *slog.Logger
}

// Config contains server configuration
type Config struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Logger       *slog.Logger
}

// DefaultConfig returns the default server configuration
func DefaultConfig() *Config {
	return &Config{
		Addr:         ":8080",
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		Logger:       slog.Default(),
	}
}

// New creates a new server with default configuration
func New() *Server {
	return NewWithConfig(DefaultConfig())
}

// NewWithConfig creates a new server with the provided configuration
func NewWithConfig(cfg *Config) *Server {
	hr := hostrouter.New()

	// Create the main router with standard middleware
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Mount("/", hr)

	s := &Server{
		Server: &http.Server{
			Addr:         cfg.Addr,
			Handler:      r,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
		},
		hostRouter: hr,
		logger:     cfg.Logger,
	}

	return s
}

// RegisterDomain maps a domain to a specific router
func (s *Server) RegisterDomain(domain string, router chi.Router) {
	s.logger.Info("Registering domain", "domain", domain)
	s.hostRouter.Map(domain, router)
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down server")
	return s.Server.Shutdown(ctx)
}

// ListenAndServe starts the server
func (s *Server) ListenAndServe() error {
	s.logger.Info("Server listening", "addr", s.Addr)
	if err := s.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return errors.Wrap(err, "server failed to start")
	}
	return nil
}
