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
	"github.com/baely/txn/internal/willbailey"
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

	// Initialize willbailey service with ibbitot check function
	willBaileyService := willbailey.NewWithConfig(&willbailey.Config{
		Logger:     log,
		DBUser:     os.Getenv("DB_USER"),
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBHost:     os.Getenv("DB_HOST"),
		DBPort:     os.Getenv("DB_PORT"),
		DBName:     os.Getenv("DB_NAME"),
		IbbitotCheck: func() bool {
			return presenceService.GetPresenceStatus() == "yes"
		},
	})

	// Register event handlers
	webhookService.RegisterHandler(presenceService)
	webhookService.RegisterHandler(trackerService)

	// Register domain handlers
	s.RegisterDomain("events.baileys.dev", webhookService.Chi())
	s.RegisterDomain("isbaileybutlerintheoffice.today", presenceService.Chi())
	s.RegisterDomain("willbaileybutlerbeintheoffice.today", willBaileyService.Chi())
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
