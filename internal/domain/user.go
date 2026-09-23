package domain

import (
	"strings"
	"time"
)

type User struct {
	ID           string
	Email        string
	FirstName    string
	LastName     string
	PasswordHash string
	Admin        bool
	Blocked      bool
	CreatedAt    time.Time
}

func (u User) DisplayName() string {
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name == "" {
		return u.Email
	}

	return name
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
