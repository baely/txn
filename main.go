package main

import (
	"github.com/baely/txn/internal/balance"
	"github.com/baely/txn/internal/ibbitot"
	"github.com/baely/txn/internal/server"
	"github.com/baely/txn/internal/tracker"
)

func main() {
	s := server.New()

	serviceWebhook := balance.New()
	serviceIbbitot := ibbitot.New()
	serviceTracker := tracker.New()

	serviceWebhook.RegisterHandler(serviceIbbitot)
	serviceWebhook.RegisterHandler(serviceTracker)

	s.RegisterDomain("events.baileys.dev", serviceWebhook.Chi())
	s.RegisterDomain("isbaileybutlerintheoffice.today", serviceIbbitot.Chi())
	s.RegisterDomain("baileyneeds.coffee", serviceTracker.Chi())

	if err := s.ListenAndServe(); err != nil {
		panic(err)
	}
}
