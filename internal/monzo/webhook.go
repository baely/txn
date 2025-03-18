// Package monzo handles transaction events and webhooks for the Monzo banking service
package monzo

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/baely/txn/internal/common/errors"
	commonHttp "github.com/baely/txn/internal/common/http"
)

// WebhookService handles webhook events from Monzo
type WebhookService struct {
	monzoClient         *MonzoClient
	rawChan             chan []byte
	router              chi.Router
	transactionHandlers []TransactionEventHandler
	logger              *slog.Logger
}

// Config contains configuration for the WebhookService
type WebhookConfig struct {
	MonzoAccessToken string
	Logger           *slog.Logger
}

// New creates a new WebhookService with default configuration
func NewWebhook() *WebhookService {
	return NewWebhookWithConfig(&WebhookConfig{
		MonzoAccessToken: os.Getenv("MONZO_ACCESS_TOKEN"),
		Logger:           slog.Default(),
	})
}

// NewWithConfig creates a new WebhookService with custom configuration
func NewWebhookWithConfig(cfg *WebhookConfig) *WebhookService {
	service := &WebhookService{
		monzoClient: NewMonzoClient(cfg.MonzoAccessToken),
		rawChan:     make(chan []byte, 100), // Buffered channel to handle bursts
		logger:      cfg.Logger,
	}

	// Setup router with standard middleware
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Register routes
	r.Post("/monzo/event", service.handleWebhook)
	r.Post("/event", service.handleWebhook)

	service.router = r

	// Start processing goroutine
	go service.processEvents()

	return service
}

// Chi returns the router for this service
func (s *WebhookService) Chi() chi.Router {
	return s.router
}

// RegisterHandler registers a handler for transaction events
func (s *WebhookService) RegisterHandler(handler TransactionEventHandler) {
	s.logger.Info("Registering transaction handler", "handler", handler)
	s.transactionHandlers = append(s.transactionHandlers, handler)
}

// handleWebhook processes incoming webhook requests
func (s *WebhookService) handleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.logger.Error("Failed to read request body", "error", err)
		commonHttp.Error(w, errors.Wrap(err, "failed to read request body"), http.StatusInternalServerError)
		return
	}

	signature := r.Header.Get("X-Monzo-Signature")
	if !ValidateWebhookEvent(body, signature) {
		s.logger.Warn("Invalid webhook signature", "signature", signature)
		commonHttp.Error(w, errors.ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	// Queue event for processing
	s.rawChan <- body

	// Return success immediately
	commonHttp.Success(w, map[string]string{"status": "accepted"})
}

// processEvents listens for events and processes them asynchronously
func (s *WebhookService) processEvents() {
	s.logger.Info("Starting webhook event processor")
	for raw := range s.rawChan {
		s.processEvent(raw)
	}
}

// processEvent handles a single event
func (s *WebhookService) processEvent(raw []byte) {
	ctx := context.Background()
	event := parseEvent(raw)
	s.logger.Info("Processing event", "type", event.Type)

	// We're only interested in transaction created events
	if event.Type != "transaction.created" {
		s.logger.Info("Ignoring non-transaction event", "type", event.Type)
		return
	}

	// Parse transaction ID from data
	var transactionData struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(event.Data, &transactionData); err != nil {
		s.logger.Error("Failed to parse transaction data", "error", err)
		return
	}

	// Get transaction details
	transaction, err := s.monzoClient.GetTransaction(ctx, transactionData.ID)
	if err != nil {
		s.logger.Error("Failed to retrieve transaction", "id", transactionData.ID, "error", err)
		return
	}

	// Get account details
	account, err := s.monzoClient.GetAccount(ctx, event.Account)
	if err != nil {
		s.logger.Error("Failed to retrieve account", "id", event.Account, "error", err)
		return
	}

	// Create event data
	data := TransactionEvent{
		Account:     account,
		Transaction: transaction,
	}

	// Notify all handlers
	for _, handler := range s.transactionHandlers {
		go func(h TransactionEventHandler, d TransactionEvent) {
			if err := h.HandleEvent(d); err != nil {
				s.logger.Error("Handler failed to process event", "handler", h, "error", err)
			}
		}(handler, data)
	}
}

// parseEvent converts JSON data to a webhook event
func parseEvent(value []byte) MonzoWebhookEvent {
	event := MonzoWebhookEvent{}
	if err := json.Unmarshal(value, &event); err != nil {
		slog.Error("Failed to parse webhook event", "error", err)
	}
	return event
}