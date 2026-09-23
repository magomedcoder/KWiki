package domain

import (
	"context"
	"time"
)

type UserRepository interface {
	Create(ctx context.Context, user User) error

	FindByEmail(ctx context.Context, email string) (User, error)

	FindByID(ctx context.Context, id string) (User, error)

	UpdatePasswordHash(ctx context.Context, id, hash string) error

	SetBlocked(ctx context.Context, id string, blocked bool) error

	SetAdmin(ctx context.Context, id string, admin bool) error

	DeleteUser(ctx context.Context, id string) error

	ListUsers(ctx context.Context) ([]User, error)

	Count(ctx context.Context) (int64, error)
}

type SessionRepository interface {
	Save(ctx context.Context, session Session) error

	Find(ctx context.Context, id string) (Session, error)

	Delete(ctx context.Context, id string) error

	DeleteByUser(ctx context.Context, userID string) error

	DeleteExpired(ctx context.Context, now time.Time) error
}

type AttemptRepository interface {
	Get(ctx context.Context, key string) (Attempt, error)

	Put(ctx context.Context, attempt Attempt) error

	Clear(ctx context.Context, key string) error
}

type PasswordHasher interface {
	Hash(password string) (string, error)

	Verify(password, encoded string) (bool, error)

	Dummy() string
}
