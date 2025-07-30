package model

import "time"

type User struct {
	ID           int       `json:"-"`
	Login        string    `json:"login"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"-"`
}