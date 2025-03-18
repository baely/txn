// Package main is the entry point for the James in the Office service
package main

import (
	"log/slog"
	"os"

	"github.com/baely/txn/internal/common/errors"
	"github.com/baely/txn/internal/common/logger"
	"github.com/baely/txn/internal/monzo"
	"github.com/baely/txn/internal/server"
)

func main() {
	// Initialize logger
	log := logger.New(
		logger.WithLevel(logger.LevelInfo),
	)
	slog.SetDefault(log)

	// Initialize server
	s := server.New()

	// Initialize services
	webhookService := monzo.NewWebhook()
	presenceService := monzo.New()

	// Register event handlers
	webhookService.RegisterHandler(presenceService)

	// Register domain handlers
	s.RegisterDomain("events.james.dev", webhookService.Chi())
	s.RegisterDomain("isjamesintheoffice.today", presenceService.Chi())

	// Start server
	log.Info("Starting James in the Office server")
	if err := s.ListenAndServe(); err != nil {
		log.Error("Server failed", "error", err)
		errors.Must(err) // This will panic
		os.Exit(1)
	}
}