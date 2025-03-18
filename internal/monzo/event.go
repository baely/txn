// Package monzo handles transaction events and webhooks for the Monzo banking service
package monzo

import (
	"time"
)

// Transaction represents a Monzo bank transaction
type Transaction struct {
	ID          string    // Transaction ID
	Description string    // Transaction description
	Amount      int       // Amount in pence (negative for debit)
	Created     time.Time // Transaction creation time
	Category    string    // Transaction category
	MerchantID  string    // ID of merchant
	Merchant    Merchant  // Merchant details
}

// Merchant represents a merchant in a Monzo transaction
type Merchant struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Logo     string `json:"logo"`
	Category string `json:"category"`
	Address  struct {
		Address   string  `json:"address"`
		City      string  `json:"city"`
		Country   string  `json:"country"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Postcode  string  `json:"postcode"`
	} `json:"address"`
}

// Account represents a Monzo account
type Account struct {
	ID       string
	Created  time.Time
	Currency string
}

// TransactionEvent contains information about a bank transaction
type TransactionEvent struct {
	Account     Account     // Account details
	Transaction Transaction // Transaction details
}

// TransactionEventHandler defines the interface for handling transaction events
type TransactionEventHandler interface {
	// HandleEvent processes a transaction event
	// Returns an error if the handling fails
	HandleEvent(event TransactionEvent) error
}