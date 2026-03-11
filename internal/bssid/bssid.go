// Package bssid provides a handler that creates coffee events based on BSSID proximity
package bssid

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/baely/txn/internal/tracker/database"
	"github.com/baely/txn/internal/tracker/models"
)

// Handler processes BSSID POST requests and creates coffee events when matched
type Handler struct {
	db          *database.Client
	targetBSSID string
	lastSeen    time.Time
	mu          sync.Mutex
	logger      *slog.Logger
}

// New creates a new Handler that reads the target BSSID from the COFFEE_BSSID env var
func New(db *database.Client) *Handler {
	return &Handler{
		db:          db,
		targetBSSID: os.Getenv("COFFEE_BSSID"),
		logger:      slog.Default(),
	}
}

type bssidRequest struct {
	BSSID string `json:"bssid"`
}

// HandleBSSID processes a POST request containing a JSON payload with a "bssid" key
func (h *Handler) HandleBSSID(w http.ResponseWriter, r *http.Request) {
	dump, err := httputil.DumpRequest(r, false)
	if err != nil {
		h.logger.Error("Failed to dump request", "error", err)
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	h.logger.Info(fmt.Sprintf("BSSID request:\n%s\nBody: %s", string(dump), string(body)))

	var req bssidRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	bssid := strings.TrimSpace(req.BSSID)

	if !strings.EqualFold(bssid, h.targetBSSID) {
		w.WriteHeader(http.StatusOK)
		return
	}

	h.mu.Lock()
	if time.Since(h.lastSeen) < 15*time.Minute {
		h.lastSeen = time.Now()
		h.mu.Unlock()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("debounced"))
		return
	}
	h.lastSeen = time.Now()
	h.mu.Unlock()

	event := models.CaffeineEvent{
		Timestamp:   time.Now(),
		Description: "Atlassian Office Coffee",
		Amount:      160,
		Cost:        0,
	}
	if err := h.db.AddEvent(event); err != nil {
		h.logger.Error("Failed to add coffee event", "error", err)
		http.Error(w, "failed to create event", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Coffee event created", "bssid", bssid)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("created"))
}
