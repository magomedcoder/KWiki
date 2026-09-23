package domain

import "time"

type User struct {
	ID           string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

type Session struct {
	ID         string
	UserID     string
	CSRF       string
	ClientHash string
	ExpiresAt  time.Time
}

type Attempt struct {
	Key         string
	Failures    int
	LockedUntil time.Time
	UpdatedAt   time.Time
}
