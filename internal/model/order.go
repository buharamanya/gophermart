package model

import "time"

type Order struct {
	ID          int        `json:"-"`
	UserID      int        `json:"-"`
	Number      string     `json:"number"`
	Status      string     `json:"status"`
	Accrual     float64    `json:"accrual,omitempty"`
	UploadedAt  time.Time  `json:"uploaded_at"`
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
}
