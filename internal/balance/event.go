// Package balance handles transaction events and webhooks for the Up banking service
package balance

import (
	"context"

	"github.com/baely/balance/pkg/model"
)

// TransactionEvent contains information about a bank transaction
type TransactionEvent struct {
	Account     model.AccountResource     // Account details
	Transaction model.TransactionResource // Transaction details
}

// TransactionEventHandler defines the interface for handling transaction events
type TransactionEventHandler interface {
	// HandleEvent processes a transaction event
	// Returns an error if the handling fails
	HandleEvent(event TransactionEvent) error
}

// EventService defines the interface for event processing services
type EventService interface {
	// RegisterHandler registers a handler for transaction events
	RegisterHandler(handler TransactionEventHandler)

	// ProcessEvent processes a transaction event
	ProcessEvent(ctx context.Context, event TransactionEvent) error
}
