package models

import "time"

type CaffeineEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	Description string    `json:"description"`
	Amount      int       `json:"amount"`
	Cost        int       `json:"cost"`
}

type CaffeineRow struct {
	Timestamp   int     `json:"timestamp"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Cost        int     `json:"cost"`
}

func ToEvent(row CaffeineRow) CaffeineEvent {
	return CaffeineEvent{
		Timestamp:   time.Unix(int64(row.Timestamp), 0),
		Description: row.Description,
		Amount:      int(row.Amount),
		Cost:        row.Cost,
	}
}
