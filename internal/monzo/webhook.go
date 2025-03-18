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
	webhookURL          string
}

// Config contains configuration for the WebhookService
type WebhookConfig struct {
	MonzoAccessToken string
	Logger           *slog.Logger
	WebhookURL       string
}

// New creates a new WebhookService with default configuration
func NewWebhook() *WebhookService {
	return NewWebhookWithConfig(&WebhookConfig{
		MonzoAccessToken: os.Getenv("MONZO_ACCESS_TOKEN"),
		WebhookURL:       os.Getenv("MONZO_WEBHOOK_URL"),
		Logger:           slog.Default(),
	})
}

// NewWithConfig creates a new WebhookService with custom configuration
func NewWebhookWithConfig(cfg *WebhookConfig) *WebhookService {
	service := &WebhookService{
		monzoClient: NewMonzoClient(cfg.MonzoAccessToken),
		rawChan:     make(chan []byte, 100), // Buffered channel to handle bursts
		logger:      cfg.Logger,
		webhookURL:  cfg.WebhookURL,
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
	r.Get("/webhooks", service.listWebhooks)
	r.Post("/webhooks/register", service.registerWebhook)
	r.Delete("/webhooks/{id}", service.deleteWebhook)

	service.router = r

	// Start processing goroutine
	go service.processEvents()

	// Set up webhooks if URL is provided
	if service.webhookURL != "" {
		go service.setupWebhooks()
	}

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

// setupWebhooks is responsible for ensuring webhooks are registered
func (s *WebhookService) setupWebhooks() {
	ctx := context.Background()
	
	// Get all accounts
	accounts, err := s.getAccounts(ctx)
	if err != nil {
		s.logger.Error("Failed to fetch accounts", "error", err)
		return
	}
	
	for _, account := range accounts {
		if err := s.ensureWebhookForAccount(ctx, account.ID); err != nil {
			s.logger.Error("Failed to set up webhook for account", "account_id", account.ID, "error", err)
		} else {
			s.logger.Info("Webhook configured for account", "account_id", account.ID)
		}
	}
}

// getAccounts fetches all available accounts
func (s *WebhookService) getAccounts(ctx context.Context) ([]Account, error) {
	var response struct {
		Accounts []struct {
			ID          string    `json:"id"`
			Created     time.Time `json:"created"`
			Description string    `json:"description"`
			Type        string    `json:"type"`
			Currency    string    `json:"currency"`
		} `json:"accounts"`
	}

	err := s.monzoClient.request(ctx, http.MethodGet, "accounts", nil, &response)
	if err != nil {
		return nil, err
	}

	var accounts []Account
	for _, acc := range response.Accounts {
		// Only include active accounts (not closed)
		accounts = append(accounts, Account{
			ID:       acc.ID,
			Created:  acc.Created,
			Currency: acc.Currency,
		})
	}

	return accounts, nil
}

// ensureWebhookForAccount ensures the account has our webhook registered
func (s *WebhookService) ensureWebhookForAccount(ctx context.Context, accountID string) error {
	if s.webhookURL == "" {
		return errors.New("webhook URL not configured")
	}

	// List existing webhooks
	webhooks, err := s.monzoClient.ListWebhooks(ctx, accountID)
	if err != nil {
		return errors.Wrap(err, "failed to list webhooks")
	}

	// Check if our webhook is already registered
	for _, webhook := range webhooks {
		if webhook.URL == s.webhookURL {
			s.logger.Info("Webhook already registered", "webhook_id", webhook.ID, "url", webhook.URL)
			return nil
		}
	}

	// If not found, register a new webhook
	webhook, err := s.monzoClient.RegisterWebhook(ctx, accountID, s.webhookURL)
	if err != nil {
		return errors.Wrap(err, "failed to register webhook")
	}

	s.logger.Info("New webhook registered", "webhook_id", webhook.ID, "url", webhook.URL)
	return nil
}

// listWebhooks handles requests to list all webhooks for an account
func (s *WebhookService) listWebhooks(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("account_id")
	if accountID == "" {
		commonHttp.Error(w, errors.New("account_id query parameter is required"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	webhooks, err := s.monzoClient.ListWebhooks(ctx, accountID)
	if err != nil {
		s.logger.Error("Failed to list webhooks", "account_id", accountID, "error", err)
		commonHttp.Error(w, err, http.StatusInternalServerError)
		return
	}

	commonHttp.Success(w, map[string]interface{}{
		"webhooks": webhooks,
	})
}

// registerWebhook handles requests to register a new webhook
func (s *WebhookService) registerWebhook(w http.ResponseWriter, r *http.Request) {
	var request struct {
		AccountID string `json:"account_id"`
		URL       string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		commonHttp.Error(w, errors.Wrap(err, "invalid request body"), http.StatusBadRequest)
		return
	}

	if request.AccountID == "" || request.URL == "" {
		commonHttp.Error(w, errors.New("account_id and url are required"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	webhook, err := s.monzoClient.RegisterWebhook(ctx, request.AccountID, request.URL)
	if err != nil {
		s.logger.Error("Failed to register webhook", "account_id", request.AccountID, "url", request.URL, "error", err)
		commonHttp.Error(w, err, http.StatusInternalServerError)
		return
	}

	commonHttp.Success(w, map[string]interface{}{
		"webhook": webhook,
	})
}

// deleteWebhook handles requests to delete a webhook
func (s *WebhookService) deleteWebhook(w http.ResponseWriter, r *http.Request) {
	webhookID := chi.URLParam(r, "id")
	if webhookID == "" {
		commonHttp.Error(w, errors.New("webhook id is required"), http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	if err := s.monzoClient.DeleteWebhook(ctx, webhookID); err != nil {
		s.logger.Error("Failed to delete webhook", "webhook_id", webhookID, "error", err)
		commonHttp.Error(w, err, http.StatusInternalServerError)
		return
	}

	commonHttp.Success(w, map[string]string{
		"status": "webhook deleted",
	})
}