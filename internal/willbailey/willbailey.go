// Package willbailey provides a service that predicts if Bailey will be in the office
package willbailey

import (
	_ "embed"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/baely/txn/internal/tracker/database"
)

// Melbourne timezone for all operations
var melbourneLocation = must(time.LoadLocation("Australia/Melbourne"))

// WillBaileyService tracks and predicts office presence
type WillBaileyService struct {
	router       chi.Router
	logger       *slog.Logger
	mutex        sync.RWMutex
	db           *database.Client
	indexPage    []byte
	ibbitotCheck func() bool // Function to check ibbitot status
}

// Config contains configuration for the WillBaileyService
type Config struct {
	Logger       *slog.Logger
	DBUser       string
	DBPassword   string
	DBHost       string
	DBPort       string
	DBName       string
	IbbitotCheck func() bool
}

// DefaultConfig returns the default service configuration
func DefaultConfig() *Config {
	return &Config{
		Logger:     slog.Default(),
		DBUser:     os.Getenv("DB_USER"),
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBHost:     os.Getenv("DB_HOST"),
		DBPort:     os.Getenv("DB_PORT"),
		DBName:     os.Getenv("DB_NAME"),
	}
}

// New creates a new WillBaileyService with default configuration
func New() *WillBaileyService {
	return NewWithConfig(DefaultConfig())
}

// NewWithConfig creates a new WillBaileyService with custom configuration
func NewWithConfig(cfg *Config) *WillBaileyService {
	// Initialize database connection
	db, err := database.NewClient(
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
	)
	if err != nil {
		cfg.Logger.Error("Failed to connect to database", "error", err)
		// We'll continue without database for now
		db = nil
	}

	s := &WillBaileyService{
		logger:       cfg.Logger,
		db:           db,
		ibbitotCheck: cfg.IbbitotCheck,
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
func (s *WillBaileyService) Chi() chi.Router {
	return s.router
}

// SetIbbitotCheck sets the function to check ibbitot status
func (s *WillBaileyService) SetIbbitotCheck(check func() bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.ibbitotCheck = check
	s.refreshPageWithoutLock()
}

// Embedded static assets
var (
	//go:embed index.html
	indexHTML string

	//go:embed calendar.png
	calendarIcon []byte
)

// handleRawStatus returns a simple yes/no response indicating predicted presence
func (s *WillBaileyService) handleRawStatus(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("Raw status request received")

	status := s.getPredictedStatus()
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Write([]byte(status))
}

// handleIndexPage serves the main HTML page
func (s *WillBaileyService) handleIndexPage(w http.ResponseWriter, r *http.Request) {
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
func (s *WillBaileyService) handleFavicon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	w.Write(calendarIcon)
}

// getPredictedStatus returns the predicted presence status
func (s *WillBaileyService) getPredictedStatus() string {
	// Check if today is a weekday
	now := time.Now().In(melbourneLocation)
	weekday := now.Weekday()

	// Not in office on weekends
	if weekday == time.Saturday || weekday == time.Sunday {
		return "no"
	}

	// Check if it's within typical office hours (8 AM - 6 PM)
	hour := now.Hour()
	if hour < 8 || hour > 18 {
		return "no"
	}

	// If we have historical data, we could analyze patterns here
	// For now, assume Bailey will be in the office on weekdays
	return "yes"
}

// refreshPage updates the index page with current data
func (s *WillBaileyService) refreshPage() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.refreshPageWithoutLock()
}

// refreshPageWithoutLock updates the index page with current data without acquiring the mutex
func (s *WillBaileyService) refreshPageWithoutLock() {
	prediction := s.getPredictedStatus()

	// Check if Bailey is currently in the office
	var description string
	if prediction == "yes" {
		description = ""
		// Check ibbitot status if available
		if s.ibbitotCheck != nil && s.ibbitotCheck() {
			description = "and he is already in"
		}
	}

	newPage := []byte(fmt.Sprintf(indexHTML, prediction, description))
	s.indexPage = newPage
}

// runDailyRefresher refreshes the page once per day at midnight
func (s *WillBaileyService) runDailyRefresher() {
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

// must panics if the given error is not nil
func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}
