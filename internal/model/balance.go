package model

import "time"

type Balance struct {
	UserID    int       `json:"-"`
	Current   float64   `json:"current"`
	Withdrawn float64   `json:"withdrawn"`
	UpdatedAt time.Time `json:"-"`
}