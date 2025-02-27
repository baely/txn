package server

import (
	_ "embed"
	"encoding/json"
	"math"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/baely/txn/internal/tracker/database"
	"github.com/baely/txn/internal/tracker/models"
)

// caffeine levels
// caffeine events
// caffeine all events

var (
	loc, _ = time.LoadLocation("Australia/Melbourne")
)

type TimeWrapper struct {
	time.Time
}

func (t TimeWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Unix())
}

type Server struct {
	db *database.Client
}

func NewServer(db *database.Client) chi.Router {
	s := &Server{
		db: db,
	}
	return s.registerApiEndpoints()
}

var (
	//go:embed index.html
	indexHTML string

	//go:embed app.js
	appJS string
)

func (s *Server) registerApiEndpoints() chi.Router {
	r := chi.NewRouter()

	r.HandleFunc("/api/levels", s.GetLevels)
	r.HandleFunc("/api/events", s.GetEvents)
	r.HandleFunc("/api/events/summary", s.GetEventsSummary)

	r.HandleFunc("/api/predefined-event", s.GetPredefinedEvent)

	r.HandleFunc("/static/app.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/javascript")
		w.Write([]byte(appJS))
	})
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		w.Write([]byte(indexHTML))
	})

	return r
}

func (s *Server) GetLevels(w http.ResponseWriter, r *http.Request) {
	startString := r.URL.Query().Get("start")
	endString := r.URL.Query().Get("end")
	start, err := time.Parse(time.RFC3339, startString)
	if err != nil {
		http.Error(w, "invalid start time", http.StatusBadRequest)
		return
	}
	end, err := time.Parse(time.RFC3339, endString)
	if err != nil {
		http.Error(w, "invalid end time", http.StatusBadRequest)
		return
	}

	caffeineLevels := s.calculateCaffeineLevels(start, end)
	json.NewEncoder(w).Encode(caffeineLevels)
}

func (s *Server) GetEvents(w http.ResponseWriter, r *http.Request) {
	startString := r.URL.Query().Get("start")
	endString := r.URL.Query().Get("end")
	start, err := time.Parse(time.RFC3339, startString)
	if err != nil {
		http.Error(w, "invalid start time", http.StatusBadRequest)
		return
	}
	end, err := time.Parse(time.RFC3339, endString)
	if err != nil {
		http.Error(w, "invalid end time", http.StatusBadRequest)
		return
	}

	events := s.db.GetEvents(start, end)
	json.NewEncoder(w).Encode(events)
}

func (s *Server) GetEventsSummary(w http.ResponseWriter, r *http.Request) {
	startString := r.URL.Query().Get("start")
	endString := r.URL.Query().Get("end")

	start := time.Time{}
	end := time.Now().AddDate(0, 0, 1) // just to be sure.

	var err error

	if startString != "" {
		start, err = time.Parse(time.RFC3339, startString)
		if err != nil {
			http.Error(w, "invalid start time", http.StatusBadRequest)
			return
		}
	}
	if endString != "" {
		end, err = time.Parse(time.RFC3339, endString)
		if err != nil {
			http.Error(w, "invalid end time", http.StatusBadRequest)
			return
		}
	}

	intake := s.db.GetTotalIntake(start, end)
	cost := s.db.GetTotalCost(start, end)

	resp := struct {
		Intake int `json:"intake"`
		Cost   int `json:"cost"`
	}{
		Intake: intake,
		Cost:   cost,
	}
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) GetPredefinedEvent(w http.ResponseWriter, r *http.Request) {
	typeString := r.URL.Query().Get("type")
	var coffeeType int
	var err error
	if typeString != "" {
		coffeeType, err = strconv.Atoi(typeString)
		if err != nil {
			http.Error(w, "invalid type", http.StatusBadRequest)
			return
		}
	}

	types := map[int]models.CaffeineEvent{
		1: {
			Timestamp:   time.Now(),
			Description: "Homemade Double Oat Latte",
			Amount:      160,
			Cost:        250,
		},
		2: {
			Timestamp:   time.Now(),
			Description: "The Jolly Miller",
			Amount:      80,
			Cost:        600,
		},
	}

	event, ok := types[coffeeType]
	if ok {
		s.db.AddEvent(event)
	}

	w.Write([]byte("ok"))
}

type LevelEvent struct {
	Timestamp TimeWrapper `json:"timestamp"`
	Level     float64     `json:"level"`
}

func (s *Server) calculateCaffeineLevels(start, end time.Time) []LevelEvent {
	const halfLife = 4
	caffeineLevels := make([]LevelEvent, 0)

	eventStart := start.Add(-72 * time.Hour) // 3 days before start
	caffeineEvents := s.db.GetEvents(eventStart, end)

	// add a level event for each snap time in the range.
	for t := range rangeTimes(start, end) {
		caffeineLevels = append(caffeineLevels, LevelEvent{
			Timestamp: TimeWrapper{t},
			Level:     calculateSumCaffeineLevel(halfLife, t, caffeineEvents),
		})
	}

	slices.SortFunc(caffeineLevels, func(a, b LevelEvent) int {
		return int(a.Timestamp.Time.Sub(b.Timestamp.Time).Seconds())
	})
	//fmt.Println(caffeineLevels)

	// add a level event for each event time and the minute before.
	for _, e := range caffeineEvents {
		t := e.Timestamp

		//fmt.Println(
		//	"t: ", t,
		//	"e.Timestamp: ", e.Timestamp,
		//	"sum: ", calculateSumCaffeineLevel(halfLife, t, caffeineEvents),
		//	"sum-1: ", calculateSumCaffeineLevel(halfLife, t.Add(-1*time.Minute), caffeineEvents),
		//)

		caffeineLevels = append(caffeineLevels, LevelEvent{
			Timestamp: TimeWrapper{t},
			Level:     calculateSumCaffeineLevel(halfLife, t, caffeineEvents),
		})

		t = e.Timestamp.Add(-1 * time.Minute)
		caffeineLevels = append(caffeineLevels, LevelEvent{
			Timestamp: TimeWrapper{t},
			Level:     calculateSumCaffeineLevel(halfLife, t, caffeineEvents),
		})
	}

	slices.SortFunc(caffeineLevels, func(a, b LevelEvent) int {
		return int(a.Timestamp.Time.Sub(b.Timestamp.Time).Seconds())
	})

	//fmt.Println(caffeineLevels)
	return caffeineLevels
}

func calculateSumCaffeineLevel(halfLife float64, t time.Time, events []models.CaffeineEvent) float64 {
	totalCaffeine := 0.0
	for _, e := range events {
		elapsed := t.Sub(e.Timestamp)
		totalCaffeine += calculateCaffeineLevel(e.Amount, halfLife, elapsed)
	}
	return totalCaffeine
}

func calculateCaffeineLevel(amount int, halfLife float64, elapsed time.Duration) float64 {
	hours := elapsed.Hours()
	if hours < 0 {
		return 0
	}
	return float64(amount) * math.Pow(0.5, float64(hours)/halfLife)
}
