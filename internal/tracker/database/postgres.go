package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/lib/pq"

	"github.com/baely/txn/internal/tracker/models"
)

type Client struct {
	db *sql.DB
}

func NewClient(user, password, host, port, db string) (*Client, error) {
	connString := fmt.Sprintf("user=%s password=%s host=%s port=%s dbname=%s sslmode=disable", user, password, host, port, db)
	driver, err := sql.Open("postgres", connString)
	if err != nil {
		return nil, err
	}
	return &Client{
		db: driver,
	}, nil
}

func (c *Client) AddEvent(event models.CaffeineEvent) {
	t := event.Timestamp.Unix()
	q := `INSERT INTO caffeine_event (timestamp, description, amount, cost) VALUES ($1, $2, $3, $4)`
	_, err := c.db.Exec(q, t, event.Description, event.Amount, event.Cost)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to add event: %v", err))
	}
}

func (c *Client) GetEvents(start, end time.Time) []models.CaffeineEvent {
	events := make([]models.CaffeineEvent, 0)
	startSeconds := start.Unix()
	endSeconds := end.Unix()
	q := `SELECT * FROM caffeine_event WHERE timestamp > $1 AND timestamp < $2 ORDER BY timestamp ASC`
	rows, err := c.db.Query(q, startSeconds, endSeconds)
	if err != nil {
		return events
	}
	for rows.Next() {
		var event models.CaffeineRow
		err = rows.Scan(&event.Timestamp, &event.Description, &event.Amount, &event.Cost)
		if err != nil {
			return events
		}
		events = append(events, models.ToEvent(event))
	}
	return events
}

func (c *Client) GetTotalCost(start, end time.Time) int {
	cost := 0
	startSeconds := start.Unix()
	if startSeconds < 0 {
		startSeconds = 0
	}
	endSeconds := end.Unix()
	q := `SELECT SUM(cost) FROM caffeine_event WHERE timestamp > $1 AND timestamp < $2`
	err := c.db.QueryRow(q, startSeconds, endSeconds).Scan(&cost)
	if err != nil {
		return 0
	}
	return cost
}

func (c *Client) GetTotalIntake(start, end time.Time) int {
	intake := 0
	startSeconds := start.Unix()
	if startSeconds < 0 {
		startSeconds = 0
	}
	endSeconds := end.Unix()
	q := `SELECT SUM(amount) FROM caffeine_event WHERE timestamp > $1 AND timestamp < $2`
	err := c.db.QueryRow(q, startSeconds, endSeconds).Scan(&intake)
	if err != nil {
		return 0
	}
	return intake
}
