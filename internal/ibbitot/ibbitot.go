// Package ibbitot provides a service that determines if Bailey is in the office
package ibbitot

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/baely/balance/pkg/model"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/baely/txn/internal/balance"
)

// Melbourne timezone for all operations
var melbourneLocation = must(time.LoadLocation("Australia/Melbourne"))

// PresenceService tracks presence based on transaction events
type PresenceService struct {
	router             chi.Router
	logger             *slog.Logger
	mutex              sync.RWMutex
	cachedTransaction  model.TransactionResource
	indexPage          []byte
	slackWebhookURL    string
	transactionFilters []TransactionFilter
}

// Config contains configuration for the PresenceService
type Config struct {
	Logger          *slog.Logger
	SlackWebhookURL string
}

// DefaultConfig returns the default service configuration
func DefaultConfig() *Config {
	return &Config{
		Logger:          slog.Default(),
		SlackWebhookURL: os.Getenv("SLACK_WEBHOOK"),
	}
}

// New creates a new PresenceService with default configuration
func New() *PresenceService {
	return NewWithConfig(DefaultConfig())
}

// NewWithConfig creates a new PresenceService with custom configuration
func NewWithConfig(cfg *Config) *PresenceService {
	s := &PresenceService{
		logger:          cfg.Logger,
		slackWebhookURL: strings.TrimSpace(cfg.SlackWebhookURL),
		transactionFilters: []TransactionFilter{
			AmountBetween(-700, -400),         // between -$7 and -$4
			Weekday(),                         // on a weekday
			NotForeign(),                      // not a foreign transaction
			Category("restaurants-and-cafes"), // in the restaurants-and-cafes category
		},
	}

	// Setup router with standard middleware
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Register routes
	r.Get("/raw", s.handleRawStatus)
	r.Get("/", s.handleIndexPage)
	r.Get("/favicon.ico", s.handleFavicon)

	s.router = r

	// Initialize page
	s.refreshPage()

	// Start daily refresher
	go s.runDailyRefresher()

	return s
}

// Chi returns the router for this service
func (s *PresenceService) Chi() chi.Router {
	return s.router
}

// HandleEvent processes transaction events from the webhook service
// It implements the balance.TransactionEventHandler interface
func (s *PresenceService) HandleEvent(event balance.TransactionEvent) error {
	s.logger.Info("Received transaction event",
		"description", event.Transaction.Attributes.Description,
		"amount", event.Transaction.Attributes.Amount.Value,
		"created_at", event.Transaction.Attributes.CreatedAt)

	s.processTransaction(event.Transaction)
	return nil
}

// Embedded static assets
var (
	//go:embed index.html
	indexHTML string

	//go:embed coffee-cup.png
	coffeeCup []byte
)

// handleRawStatus returns a simple yes/no response indicating presence
func (s *PresenceService) handleRawStatus(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("Raw status request received")

	status := s.GetPresenceStatus()
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Write([]byte(status))
}

// handleIndexPage serves the main HTML page
func (s *PresenceService) handleIndexPage(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("Index page request received")

	s.mutex.RLock()
	page := s.indexPage
	s.mutex.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Write(page)
}

// handleFavicon serves the favicon
func (s *PresenceService) handleFavicon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	w.Write(coffeeCup)
}

// processTransaction determines if a transaction indicates presence
func (s *PresenceService) processTransaction(transaction model.TransactionResource) {
	// Apply all filters to the transaction
	if !s.meetsAllCriteria(transaction) {
		s.logger.Info("Transaction does not meet presence criteria",
			"description", transaction.Attributes.Description)
		return
	}

	s.storeTransaction(transaction)
}

// meetsAllCriteria checks if a transaction meets all filter criteria
func (s *PresenceService) meetsAllCriteria(transaction model.TransactionResource) bool {
	for _, filter := range s.transactionFilters {
		if !filter(transaction) {
			return false
		}
	}
	return true
}

// GetPresenceStatus returns the current presence status as a string
func (s *PresenceService) GetPresenceStatus() string {
	s.mutex.RLock()
	transaction := s.cachedTransaction
	s.mutex.RUnlock()

	if isTransactionToday(transaction) {
		return "yes"
	}
	return "no"
}

// storeTransaction stores a new transaction if it's more recent than the current one
func (s *PresenceService) storeTransaction(transaction model.TransactionResource) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Only update if the new transaction is more recent
	if !s.cachedTransaction.Attributes.CreatedAt.IsZero() &&
		s.cachedTransaction.Attributes.CreatedAt.After(transaction.Attributes.CreatedAt) {
		return
	}

	s.logger.Info("Updating cached transaction",
		"description", transaction.Attributes.Description,
		"created_at", transaction.Attributes.CreatedAt.Format(time.RFC3339))

	s.cachedTransaction = transaction
	s.refreshPageWithoutLock()
}

// refreshPage updates the index page with current data
func (s *PresenceService) refreshPage() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.refreshPageWithoutLock()
}

// refreshPageWithoutLock updates the index page with current data without acquiring the mutex
// Caller must hold the mutex lock before calling this function
func (s *PresenceService) refreshPageWithoutLock() {
	isPresent := isTransactionToday(s.cachedTransaction)
	status := "no"
	if isPresent {
		status = "yes"
	}

	description := s.getPresenceDescription(isPresent, s.cachedTransaction)
	newPage := []byte(fmt.Sprintf(indexHTML, status, description))

	// Check if the page content has changed
	changed := !bytes.Equal(s.indexPage, newPage)
	s.indexPage = newPage

	// Notify Slack if the page changed
	if changed && s.slackWebhookURL != "" {
		// Create local copies of variables needed for the goroutine
		slackURL := s.slackWebhookURL
		statusCopy := status
		descCopy := description

		go func(url, status, description string) {
			s.notifySlack(status, description)
		}(slackURL, statusCopy, descCopy)
	}
}

// getPresenceDescription formats a description for the presence status
func (s *PresenceService) getPresenceDescription(isPresent bool, transaction model.TransactionResource) string {
	if !isPresent || transaction.Id == "" {
		return ""
	}

	amount := fmt.Sprintf("$%.2f", -float64(transaction.Attributes.Amount.ValueInBaseUnits)/100.0)
	timeStr := transaction.Attributes.CreatedAt.In(melbourneLocation).Format(time.Kitchen)
	details := fmt.Sprintf("%s at %s", transaction.Attributes.Description, timeStr)

	return fmt.Sprintf("<img src=\"/favicon.ico\" />%s on %s", amount, details)
}

// notifySlack sends a notification to Slack when presence status changes
func (s *PresenceService) notifySlack(status, description string) {
	if s.slackWebhookURL == "" {
		return
	}

	// Clean description for Slack
	description = strings.Replace(description, "<img src=\"/favicon.ico\" />", "", -1)

	payload := struct {
		Status      string `json:"status"`
		Description string `json:"description"`
	}{
		Status:      status,
		Description: description,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		s.logger.Error("Failed to marshal Slack payload", "error", err)
		return
	}

	resp, err := http.Post(s.slackWebhookURL, "application/json", bytes.NewReader(data))
	if err != nil {
		s.logger.Error("Failed to send Slack notification", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.logger.Error("Slack notification failed", "status", resp.Status)
	}
}

// runDailyRefresher refreshes the page once per day at midnight
func (s *PresenceService) runDailyRefresher() {
	s.logger.Info("Starting daily page refresher")

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		now := time.Now().In(melbourneLocation)
		nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 10, 0, melbourneLocation)
		timeToWait := nextMidnight.Sub(now)

		s.logger.Debug("Daily refresher cycle",
			"current_time", now.Format(time.RFC3339),
			"next_refresh", nextMidnight.Format(time.RFC3339),
			"wait_duration", timeToWait.String())

		// Wait until midnight
		time.Sleep(timeToWait)

		// Refresh the page to clear yesterday's status
		s.refreshPage()

		// Short sleep to avoid potential race conditions
		time.Sleep(time.Second)
	}
}

// Helper functions and types

// TransactionFilter is a function that determines if a transaction meets a specific criterion
type TransactionFilter func(model.TransactionResource) bool

// AmountBetween creates a filter that checks if the transaction amount is between the given values
func AmountBetween(minBaseUnits, maxBaseUnits int) TransactionFilter {
	return func(transaction model.TransactionResource) bool {
		valueInBaseUnits := transaction.Attributes.Amount.ValueInBaseUnits
		return valueInBaseUnits >= minBaseUnits && valueInBaseUnits <= maxBaseUnits
	}
}

// TimeBetween creates a filter that checks if the transaction time is between specific hours
func TimeBetween(minHour, maxHour int) TransactionFilter {
	return func(transaction model.TransactionResource) bool {
		hour := transaction.Attributes.CreatedAt.In(melbourneLocation).Hour()
		return hour >= minHour && hour <= maxHour
	}
}

// Weekday creates a filter that checks if the transaction occurred on a weekday
func Weekday() TransactionFilter {
	return func(transaction model.TransactionResource) bool {
		day := transaction.Attributes.CreatedAt.In(melbourneLocation).Weekday()
		return day >= time.Monday && day <= time.Friday
	}
}

// NotForeign creates a filter that checks if the transaction is not a foreign transaction
func NotForeign() TransactionFilter {
	return func(transaction model.TransactionResource) bool {
		return transaction.Attributes.ForeignAmount == nil
	}
}

// Category creates a filter that checks if the transaction belongs to a specific category
func Category(categoryID string) TransactionFilter {
	return func(transaction model.TransactionResource) bool {
		if transaction.Relationships.Category.Data == nil {
			return false
		}
		return transaction.Relationships.Category.Data.Id == categoryID
	}
}

// isTransactionToday checks if a transaction occurred today
func isTransactionToday(transaction model.TransactionResource) bool {
	if transaction.Id == "" {
		return false
	}

	now := time.Now().In(melbourneLocation)
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, melbourneLocation)
	return transaction.Attributes.CreatedAt.In(melbourneLocation).After(midnight)
}

// must panics if the given error is not nil
func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}
