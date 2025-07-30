package model

import "time"

type Withdrawal struct {
	ID          int       `json:"-"`
	UserID      int       `json:"-"`
	OrderNumber string    `json:"order"`
	Sum         float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
}