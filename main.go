// Package main is the entry point for the TXN application
package main

import (
	"log/slog"
	"os"

	"github.com/baely/txn/internal/balance"
	"github.com/baely/txn/internal/common/errors"
	"github.com/baely/txn/internal/common/logger"
	"github.com/baely/txn/internal/ibbitot"
	"github.com/baely/txn/internal/server"
	"github.com/baely/txn/internal/tracker"
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
	webhookService := balance.New()
	presenceService := ibbitot.New()
	trackerService := tracker.New()

	// Register event handlers
	webhookService.RegisterHandler(presenceService)
	webhookService.RegisterHandler(trackerService)

	// Register domain handlers
	s.RegisterDomain("events.baileys.dev", webhookService.Chi())
	s.RegisterDomain("isbaileybutlerintheoffice.today", presenceService.Chi())
	s.RegisterDomain("baileyneeds.coffee", trackerService.Chi())
	s.RegisterDomain("caffeine-api.baileys.dev", trackerService.Chi())
	s.RegisterDomain("caffeine.baileys.app", trackerService.Chi())

	// Start server
	log.Info("Starting server")
	if err := s.ListenAndServe(); err != nil {
		log.Error("Server failed", "error", err)
		errors.Must(err) // This will panic
		os.Exit(1)
	}
}
