package sqlite

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/magomedcoder/kwiki/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	_ domain.UserRepository    = (*Repository)(nil)
	_ domain.SessionRepository = (*Repository)(nil)
	_ domain.AttemptRepository = (*Repository)(nil)
)

type userRow struct {
	ID           string `gorm:"primaryKey;size:32"`
	Email        string `gorm:"uniqueIndex;size:254"`
	FirstName    string `gorm:"size:80"`
	LastName     string `gorm:"size:80"`
	PasswordHash string `gorm:"size:512"`
	Admin        bool
	Blocked      bool
	CreatedAt    time.Time
}

func (userRow) TableName() string {
	return "users"
}

type sessionRow struct {
	ID         string    `gorm:"primaryKey;size:64"`
	UserID     string    `gorm:"index;size:32"`
	CSRF       string    `gorm:"size:128"`
	ClientHash string    `gorm:"size:64"`
	ExpiresAt  time.Time `gorm:"index"`
}

func (sessionRow) TableName() string {
	return "sessions"
}

type attemptRow struct {
	Key         string `gorm:"primaryKey;size:80"`
	Failures    int
	LockedUntil time.Time
	UpdatedAt   time.Time
}

func (attemptRow) TableName() string {
	return "login_attempts"
}

func (r *Repository) Create(ctx context.Context, user domain.User) error {
	err := r.db.WithContext(ctx).Create(&userRow{
		ID:           user.ID,
		Email:        user.Email,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		PasswordHash: user.PasswordHash,
		Admin:        user.Admin,
		Blocked:      user.Blocked,
		CreatedAt:    user.CreatedAt,
	}).Error
	if err != nil && (errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(err.Error(), "UNIQUE")) {
		return domain.ErrEmailTaken
	}

	return err
}

func (r *Repository) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	var row userRow
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.User{}, domain.ErrNotFound
	}

	if err != nil {
		return domain.User{}, err
	}

	return toUser(row), nil
}

func (r *Repository) FindByID(ctx context.Context, id string) (domain.User, error) {
	var row userRow
	err := r.db.WithContext(ctx).First(&row, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.User{}, domain.ErrNotFound
	}

	if err != nil {
		return domain.User{}, err
	}

	return toUser(row), nil
}

func (r *Repository) UpdatePasswordHash(ctx context.Context, id, hash string) error {
	res := r.db.WithContext(ctx).Model(&userRow{}).Where("id = ?", id).Update("password_hash", hash)
	if res.Error != nil {
		return res.Error
	}

	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *Repository) SetBlocked(ctx context.Context, id string, blocked bool) error {
	return r.updateUserFlag(ctx, id, "blocked", blocked)
}

func (r *Repository) SetAdmin(ctx context.Context, id string, admin bool) error {
	return r.updateUserFlag(ctx, id, "admin", admin)
}

func (r *Repository) updateUserFlag(ctx context.Context, id, column string, value bool) error {
	res := r.db.WithContext(ctx).Model(&userRow{}).Where("id = ?", id).Update(column, value)
	if res.Error != nil {
		return res.Error
	}

	if res.RowsAffected > 0 {
		return nil
	}

	var n int64
	if err := r.db.WithContext(ctx).Model(&userRow{}).Where("id = ?", id).Count(&n).Error; err != nil {
		return err
	}

	if n == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *Repository) DeleteUser(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Delete(&userRow{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}

	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *Repository) ListUsers(ctx context.Context) ([]domain.User, error) {
	var rows []userRow
	err := r.db.WithContext(ctx).Order("last_name ASC, first_name ASC, email ASC").Find(&rows).Error
	if err != nil {
		return nil, err
	}

	users := make([]domain.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, toUser(row))
	}

	return users, nil
}

func (r *Repository) Count(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&userRow{}).Count(&n).Error
	return n, err
}

func (r *Repository) Save(ctx context.Context, session domain.Session) error {
	row := sessionRow{
		ID:         session.ID,
		UserID:     session.UserID,
		CSRF:       session.CSRF,
		ClientHash: session.ClientHash,
		ExpiresAt:  session.ExpiresAt,
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_id", "csrf", "client_hash", "expires_at"}),
	}).Create(&row).Error
}

func (r *Repository) Find(ctx context.Context, id string) (domain.Session, error) {
	var row sessionRow
	err := r.db.WithContext(ctx).First(&row, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Session{}, domain.ErrNotFound
	}

	if err != nil {
		return domain.Session{}, err
	}

	return domain.Session{
		ID:         row.ID,
		UserID:     row.UserID,
		CSRF:       row.CSRF,
		ClientHash: row.ClientHash,
		ExpiresAt:  row.ExpiresAt,
	}, nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&sessionRow{}, "id = ?", id).Error
}

func (r *Repository) DeleteByUser(ctx context.Context, userID string) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&sessionRow{}).Error
}

func (r *Repository) DeleteExpired(ctx context.Context, now time.Time) error {
	return r.db.WithContext(ctx).Where("expires_at <= ?", now).Delete(&sessionRow{}).Error
}

func (r *Repository) Get(ctx context.Context, key string) (domain.Attempt, error) {
	var row attemptRow
	err := r.db.WithContext(ctx).First(&row, "key = ?", key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Attempt{}, domain.ErrNotFound
	}

	if err != nil {
		return domain.Attempt{}, err
	}

	return domain.Attempt{
		Key:         row.Key,
		Failures:    row.Failures,
		LockedUntil: row.LockedUntil,
		UpdatedAt:   row.UpdatedAt,
	}, nil
}

func (r *Repository) Put(ctx context.Context, attempt domain.Attempt) error {
	row := attemptRow{
		Key:         attempt.Key,
		Failures:    attempt.Failures,
		LockedUntil: attempt.LockedUntil,
		UpdatedAt:   attempt.UpdatedAt,
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"failures", "locked_until", "updated_at"}),
	}).Create(&row).Error
}

func (r *Repository) Clear(ctx context.Context, key string) error {
	return r.db.WithContext(ctx).Delete(&attemptRow{}, "key = ?", key).Error
}

func toUser(row userRow) domain.User {
	return domain.User{
		ID:           row.ID,
		Email:        row.Email,
		FirstName:    row.FirstName,
		LastName:     row.LastName,
		PasswordHash: row.PasswordHash,
		Admin:        row.Admin,
		Blocked:      row.Blocked,
		CreatedAt:    row.CreatedAt,
	}
}
