// Package tracker provides caffeine consumption tracking services
package tracker

import (
	"log/slog"
	"os"

	"github.com/go-chi/chi/v5"

	"github.com/baely/txn/internal/balance"
	"github.com/baely/txn/internal/common/errors"
	"github.com/baely/txn/internal/tracker/database"
	"github.com/baely/txn/internal/tracker/server"
)

// TrackerService tracks caffeine consumption events
type TrackerService struct {
	db        *database.Client
	router    chi.Router
	logger    *slog.Logger
}

// Config contains configuration for the TrackerService
type Config struct {
	DBUser     string
	DBPassword string
	DBHost     string
	DBPort     string
	DBName     string
	Logger     *slog.Logger
}

// DefaultConfig returns the default service configuration
func DefaultConfig() *Config {
	return &Config{
		DBUser:     os.Getenv("DB_USER"),
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBHost:     os.Getenv("DB_HOST"),
		DBPort:     os.Getenv("DB_PORT"),
		DBName:     os.Getenv("DB_NAME"),
		Logger:     slog.Default(),
	}
}

// New creates a new TrackerService with default configuration
func New() *TrackerService {
	return NewWithConfig(DefaultConfig())
}

// NewWithConfig creates a new TrackerService with custom configuration
func NewWithConfig(cfg *Config) *TrackerService {
	db, err := database.NewClient(
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
	)
	if err != nil {
		errors.Must(err) // This will panic with database connection errors
	}

	t := &TrackerService{
		db:     db,
		logger: cfg.Logger,
	}

	// Initialize router
	t.router = server.NewServer(db)

	return t
}

// Chi returns the router for this service
func (t *TrackerService) Chi() chi.Router {
	return t.router
}

// HandleEvent processes transaction events from the webhook service
// It implements the balance.TransactionEventHandler interface
func (t *TrackerService) HandleEvent(event balance.TransactionEvent) error {
	t.logger.Info("Processing transaction event",
		"description", event.Transaction.Attributes.Description,
		"amount", event.Transaction.Attributes.Amount.Value)
		
	return server.ProcessEvent(t.db, event)
}
