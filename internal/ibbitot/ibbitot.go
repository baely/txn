// Package ibbitot provides a service that determines if Bailey is in the office
package ibbitot

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/baely/txn/internal/balance"
)

// Melbourne timezone for all operations
var melbourneLocation = must(time.LoadLocation("Australia/Melbourne"))

// PresenceService tracks presence based on manual admin updates
type PresenceService struct {
	router          chi.Router
	logger          *slog.Logger
	mutex           sync.RWMutex
	isInOffice      bool
	subtitle        string
	lastUpdated     time.Time
	indexPage       []byte
	adminPage       []byte
	slackWebhookURL string
	adminSecretCode string
	cacheFilePath   string
}

// Config contains configuration for the PresenceService
type Config struct {
	Logger          *slog.Logger
	SlackWebhookURL string
	AdminSecretCode string
	CacheDir        string
}

// DefaultConfig returns the default service configuration
func DefaultConfig() *Config {
	cacheDir := os.Getenv("CACHE_DIR")
	if cacheDir == "" {
		cacheDir = "/data"
	}
	return &Config{
		Logger:          slog.Default(),
		SlackWebhookURL: os.Getenv("SLACK_WEBHOOK"),
		AdminSecretCode: os.Getenv("ADMIN_SECRET_CODE"),
		CacheDir:        cacheDir,
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
		adminSecretCode: strings.TrimSpace(cfg.AdminSecretCode),
		cacheFilePath:   filepath.Join(cfg.CacheDir, "ibbitot-cache.json"),
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
	r.Get("/admin", s.handleAdminPage)
	r.Post("/admin", s.handleAdminPage)
	r.Post("/admin/update", s.handleAdminUpdate)

	s.router = r

	// Load cached state from file if it exists
	s.loadCacheFromFile()

	// Initialize pages
	s.refreshPage()
	s.refreshAdminPage()

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
// This is now a no-op since we don't use transaction data anymore
func (s *PresenceService) HandleEvent(event balance.TransactionEvent) error {
	s.logger.Debug("Received transaction event (ignored)",
		"description", event.Transaction.Attributes.Description,
		"amount", event.Transaction.Attributes.Amount.Value)
	return nil
}

// Embedded static assets
var (
	//go:embed index.html
	indexHTML string

	//go:embed admin.html
	adminHTML string

	//go:embed coffee-cup.png
	coffeeCup []byte
)

// handleRawStatus returns a simple yes/no response indicating presence
func (s *PresenceService) handleRawStatus(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("Raw status request received")

	status := s.getPresenceStatus()
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

// handleAdminPage serves the admin interface
func (s *PresenceService) handleAdminPage(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("Admin page request received", "method", r.Method)

	// Check secret code
	var providedCode string
	if r.Method == "POST" {
		r.ParseForm()
		providedCode = r.FormValue("secret_code")
	} else {
		providedCode = r.URL.Query().Get("code")
	}

	// If no code provided or wrong code, show password entry
	if providedCode != s.adminSecretCode || s.adminSecretCode == "" {
		s.logger.Warn("Invalid admin access attempt")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
	<title>Admin Access</title>
	<style>
		body { font-family: 'Helvetica Neue', Arial, sans-serif; max-width: 400px; margin: 100px auto; padding: 20px; }
		input[type="password"] { width: 100%; padding: 10px; margin: 10px 0; font-size: 16px; }
		button { width: 100%; padding: 12px; background: #007AFF; color: white; border: none; font-size: 16px; cursor: pointer; border-radius: 5px; }
		button:hover { background: #0051D5; }
	</style>
</head>
<body>
	<h2>Admin Access Required</h2>
	<form method="POST">
		<input type="password" name="secret_code" placeholder="Enter secret code" required>
		<button type="submit">Access</button>
	</form>
</body>
</html>`))
		return
	}

	// Valid code - show admin interface
	s.mutex.RLock()
	page := s.adminPage
	s.mutex.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Write(page)
}

// handleAdminUpdate processes admin form submissions
func (s *PresenceService) handleAdminUpdate(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("Admin update request received")

	r.ParseForm()

	// Verify secret code
	secretCode := r.FormValue("secret_code")
	if secretCode != s.adminSecretCode || s.adminSecretCode == "" {
		s.logger.Warn("Invalid admin update attempt")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get form values
	status := r.FormValue("status")
	subtitle := r.FormValue("subtitle")

	isInOffice := status == "yes"

	s.logger.Info("Updating office status",
		"is_in_office", isInOffice,
		"subtitle", subtitle)

	s.updateStatus(isInOffice, subtitle)

	// Redirect back to admin page
	http.Redirect(w, r, "/admin?code="+html.EscapeString(secretCode), http.StatusSeeOther)
}

// updateStatus updates the office status and subtitle
func (s *PresenceService) updateStatus(isInOffice bool, subtitle string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.isInOffice = isInOffice
	s.subtitle = strings.TrimSpace(subtitle)
	s.lastUpdated = time.Now()

	s.refreshPageWithoutLock()
	s.refreshAdminPageWithoutLock()

	// Persist cache to file asynchronously
	go s.saveCacheToFile()
}

// getPresenceStatus returns the current presence status as a string
func (s *PresenceService) getPresenceStatus() string {
	s.mutex.RLock()
	isInOffice := s.isInOffice
	s.mutex.RUnlock()

	if isInOffice {
		return "yes"
	}
	return "no"
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
	status := "no"
	if s.isInOffice {
		status = "yes"
	}

	description := s.getPresenceDescription()
	newPage := []byte(fmt.Sprintf(indexHTML, status, description))

	// Check if the page content has changed
	changed := !bytes.Equal(s.indexPage, newPage)
	s.indexPage = newPage

	// Notify Slack if the page changed
	if changed && s.slackWebhookURL != "" {
		// Create local copies of variables needed for the goroutine
		statusCopy := status
		descCopy := description

		go func(status, description string) {
			s.notifySlack(status, description)
		}(statusCopy, descCopy)
	}
}

// refreshAdminPage updates the admin page with current data
func (s *PresenceService) refreshAdminPage() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.refreshAdminPageWithoutLock()
}

// refreshAdminPageWithoutLock updates the admin page without acquiring the mutex
// Caller must hold the mutex lock before calling this function
func (s *PresenceService) refreshAdminPageWithoutLock() {
	yesChecked := ""
	noChecked := ""
	if s.isInOffice {
		yesChecked = "checked"
	} else {
		noChecked = "checked"
	}

	subtitle := html.EscapeString(s.subtitle)
	s.adminPage = []byte(fmt.Sprintf(adminHTML, yesChecked, noChecked, subtitle))
}

// getPresenceDescription returns the current subtitle
func (s *PresenceService) getPresenceDescription() string {
	if !s.isInOffice || s.subtitle == "" {
		return ""
	}
	return s.subtitle
}

// notifySlack sends a notification to Slack when presence status changes
func (s *PresenceService) notifySlack(status, description string) {
	if s.slackWebhookURL == "" {
		return
	}

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

// runDailyRefresher refreshes the page once per day at midnight and resets status
func (s *PresenceService) runDailyRefresher() {
	s.logger.Info("Starting daily page refresher")

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

		// Reset status to "no" at midnight
		s.logger.Info("Daily reset: setting status to 'no'")
		s.updateStatus(false, "")

		// Short sleep to avoid potential race conditions
		time.Sleep(time.Second)
	}
}

// Helper functions

// must panics if the given error is not nil
func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}

// cacheData represents the structure of the cached data file
type cacheData struct {
	IsInOffice  bool      `json:"is_in_office"`
	Subtitle    string    `json:"subtitle"`
	LastUpdated time.Time `json:"last_updated"`
}

// saveCacheToFile persists the cached state to disk
func (s *PresenceService) saveCacheToFile() {
	s.mutex.RLock()
	cache := cacheData{
		IsInOffice:  s.isInOffice,
		Subtitle:    s.subtitle,
		LastUpdated: s.lastUpdated,
	}
	s.mutex.RUnlock()

	data, err := json.Marshal(cache)
	if err != nil {
		s.logger.Error("Failed to marshal cache data", "error", err)
		return
	}

	// Ensure the directory exists
	dir := filepath.Dir(s.cacheFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		s.logger.Error("Failed to create cache directory", "error", err, "path", dir)
		return
	}

	// Write the file
	if err := os.WriteFile(s.cacheFilePath, data, 0644); err != nil {
		s.logger.Error("Failed to write cache file", "error", err, "path", s.cacheFilePath)
		return
	}

	s.logger.Info("Cache saved to file", "path", s.cacheFilePath)
}

// loadCacheFromFile loads the cached state from disk
func (s *PresenceService) loadCacheFromFile() {
	data, err := os.ReadFile(s.cacheFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.logger.Info("No cache file found, starting fresh", "path", s.cacheFilePath)
		} else {
			s.logger.Error("Failed to read cache file", "error", err, "path", s.cacheFilePath)
		}
		return
	}

	var cache cacheData
	if err := json.Unmarshal(data, &cache); err != nil {
		s.logger.Error("Failed to unmarshal cache data", "error", err)
		return
	}

	s.mutex.Lock()
	s.isInOffice = cache.IsInOffice
	s.subtitle = cache.Subtitle
	s.lastUpdated = cache.LastUpdated
	s.mutex.Unlock()

	s.logger.Info("Cache loaded from file",
		"path", s.cacheFilePath,
		"is_in_office", cache.IsInOffice,
		"subtitle", cache.Subtitle)
}
