package app

import "time"

type User struct {
	ID           int64
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

type Link struct {
	ID         int64      `json:"id"`
	Code       string     `json:"code"`
	OwnerUserID int64     `json:"owner_user_id"`
	LongURL    string     `json:"long_url"`
	IsActive   bool       `json:"is_active"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type DailyStat struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}

