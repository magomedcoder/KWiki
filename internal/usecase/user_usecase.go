package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/magomedcoder/kwiki/internal/domain"
)

const (
	maxFailures       = 5
	lockDuration      = 15 * time.Minute
	failureWindow     = 15 * time.Minute
	sessionTTL        = 12 * time.Hour
	loginChallengeTTL = 10 * time.Minute
)

type Actor struct {
	ID    string
	Email string
	Name  string
	Admin bool
	CSRF  string
}

type Account struct {
	FirstName string
	LastName  string
	Email     string
	Password  string
	Admin     bool
}

type ManagedUser struct {
	Email     string
	FirstName string
	LastName  string
	Admin     bool
	Blocked   bool
	Self      bool
}

type UsersPage struct {
	Users  []ManagedUser
	Error  string
	Notice string
}

type IssuedSession struct {
	Token     string
	CSRF      string
	ExpiresAt time.Time
}

type LoginPage struct {
	CSRF  string
	Email string
	Next  string
	Error string
}

type UserUseCase struct {
	users     domain.UserRepository
	sessions  domain.SessionRepository
	attempts  domain.AttemptRepository
	passwords domain.PasswordHasher
	now       func() time.Time
}

func NewAuth(users domain.UserRepository, sessions domain.SessionRepository, attempts domain.AttemptRepository, passwords domain.PasswordHasher) *UserUseCase {
	return &UserUseCase{
		users:     users,
		sessions:  sessions,
		attempts:  attempts,
		passwords: passwords,
	}
}

func (a *UserUseCase) UserCount(ctx context.Context) (int64, error) {
	return a.users.Count(ctx)
}

func (a *UserUseCase) CreateUser(ctx context.Context, account Account) error {
	firstName, err := domain.NormalizeName(account.FirstName)
	if err != nil {
		return err
	}
	lastName, err := domain.NormalizeName(account.LastName)
	if err != nil {
		return err
	}

	normalized, err := domain.NormalizeEmail(account.Email)
	if err != nil {
		return err
	}

	if err := domain.ValidatePassword(normalized, account.Password); err != nil {
		return err
	}

	if _, err := a.users.FindByEmail(ctx, normalized); err == nil {
		return domain.ErrEmailTaken
	} else if !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	count, err := a.users.Count(ctx)
	if err != nil {
		return err
	}

	hash, err := a.passwords.Hash(account.Password)
	if err != nil {
		return err
	}

	id, err := newID()
	if err != nil {
		return err
	}

	return a.users.Create(ctx, domain.User{
		ID:           id,
		Email:        normalized,
		FirstName:    firstName,
		LastName:     lastName,
		PasswordHash: hash,
		Admin:        account.Admin || count == 0,
		CreatedAt:    a.clock(),
	})
}

func (a *UserUseCase) ListUsers(ctx context.Context, actorID string) ([]ManagedUser, error) {
	users, err := a.users.ListUsers(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]ManagedUser, 0, len(users))
	for _, user := range users {
		out = append(out, ManagedUser{
			Email:     user.Email,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Admin:     user.Admin,
			Blocked:   user.Blocked,
			Self:      actorID != "" && user.ID == actorID,
		})
	}

	return out, nil
}

func (a *UserUseCase) DeleteUser(ctx context.Context, actorID, email string) error {
	user, err := a.userByEmail(ctx, email)
	if err != nil {
		return err
	}

	if actorID != "" && actorID == user.ID {
		return domain.ErrSelfAction
	}

	if user.Admin {
		other, err := a.hasAnotherActiveAdmin(ctx, user.ID)
		if err != nil {
			return err
		}

		if !other {
			return domain.ErrLastAdmin
		}
	}

	if err := a.sessions.DeleteByUser(ctx, user.ID); err != nil {
		return err
	}

	return a.users.DeleteUser(ctx, user.ID)
}

func (a *UserUseCase) SetBlocked(ctx context.Context, actorID, email string, blocked bool) error {
	user, err := a.userByEmail(ctx, email)
	if err != nil {
		return err
	}

	if actorID != "" && actorID == user.ID {
		return domain.ErrSelfAction
	}

	if blocked && user.Admin && !user.Blocked {
		other, err := a.hasAnotherActiveAdmin(ctx, user.ID)
		if err != nil {
			return err
		}

		if !other {
			return domain.ErrLastAdmin
		}
	}

	if err := a.users.SetBlocked(ctx, user.ID, blocked); err != nil {
		return err
	}

	if blocked {
		return a.sessions.DeleteByUser(ctx, user.ID)
	}

	return nil
}

func (a *UserUseCase) EnsureAdmin(ctx context.Context) error {
	users, err := a.users.ListUsers(ctx)
	if err != nil || len(users) == 0 {
		return err
	}

	var oldest domain.User
	found := false
	for _, user := range users {
		if user.Admin {
			return nil
		}

		if !found || user.CreatedAt.Before(oldest.CreatedAt) {
			oldest = user
			found = true
		}
	}
	if !found {
		return nil
	}

	return a.users.SetAdmin(ctx, oldest.ID, true)
}

func (a *UserUseCase) ChangePassword(ctx context.Context, email, password string) error {
	normalized, err := domain.NormalizeEmail(email)
	if err != nil {
		return err
	}

	if err := domain.ValidatePassword(normalized, password); err != nil {
		return err
	}

	user, err := a.users.FindByEmail(ctx, normalized)
	if err != nil {
		return err
	}

	hash, err := a.passwords.Hash(password)
	if err != nil {
		return err
	}

	if err := a.users.UpdatePasswordHash(ctx, user.ID, hash); err != nil {
		return err
	}

	return a.sessions.DeleteByUser(ctx, user.ID)
}

func (a *UserUseCase) BeginLogin(ctx context.Context, previousToken, client string) (IssuedSession, error) {
	now := a.clock()
	if err := a.sessions.DeleteExpired(ctx, now); err != nil {
		return IssuedSession{}, err
	}
	if previousToken != "" {
		old, err := a.sessions.Find(ctx, hashToken(previousToken))
		if err == nil && old.UserID == "" {
			if err := a.sessions.Delete(ctx, old.ID); err != nil {
				return IssuedSession{}, err
			}
		} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return IssuedSession{}, err
		}
	}

	return a.issue(ctx, "", client, now.Add(loginChallengeTTL))
}

func (a *UserUseCase) Login(ctx context.Context, challengeToken, csrf, email, password, ip, client string) (IssuedSession, error) {
	now := a.clock()
	if err := a.sessions.DeleteExpired(ctx, now); err != nil {
		return IssuedSession{}, err
	}

	sess, err := a.sessions.Find(ctx, hashToken(challengeToken))
	if errors.Is(err, domain.ErrNotFound) {
		return IssuedSession{}, domain.ErrCSRF
	}

	if err != nil {
		return IssuedSession{}, err
	}

	if sess.UserID != "" {
		return IssuedSession{}, domain.ErrCSRF
	}

	if sess.ClientHash != clientHash(client) || !TokenEqual(sess.CSRF, csrf) {
		_ = a.sessions.Delete(ctx, sess.ID)
		return IssuedSession{}, domain.ErrCSRF
	}

	if err := a.sessions.Delete(ctx, sess.ID); err != nil {
		return IssuedSession{}, err
	}

	if locked, err := a.locked(ctx, attemptKey("ip", ip), now); err != nil {
		return IssuedSession{}, err
	} else if locked {
		_, _ = a.verify(password, a.passwords.Dummy())
		return IssuedSession{}, domain.ErrTooManyAttempts
	}

	normalized, emailErr := domain.NormalizeEmail(email)
	if emailErr != nil {
		_, _ = a.verify(password, a.passwords.Dummy())
		if _, err := a.bump(ctx, attemptKey("ip", ip), now); err != nil {
			return IssuedSession{}, err
		}
		return IssuedSession{}, domain.ErrInvalidCredentials
	}

	if locked, err := a.locked(ctx, attemptKey("mail", normalized), now); err != nil {
		return IssuedSession{}, err
	} else if locked {
		_, _ = a.verify(password, a.passwords.Dummy())
		return IssuedSession{}, domain.ErrTooManyAttempts
	}

	user, err := a.users.FindByEmail(ctx, normalized)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return IssuedSession{}, err
	}

	hash := a.passwords.Dummy()
	if err == nil {
		hash = user.PasswordHash
	}

	ok, err := a.verify(password, hash)
	if err != nil {
		return IssuedSession{}, err
	}

	if user.ID == "" || !ok {
		ipLock, err := a.bump(ctx, attemptKey("ip", ip), now)
		if err != nil {
			return IssuedSession{}, err
		}

		mailLock, err := a.bump(ctx, attemptKey("mail", normalized), now)
		if err != nil {
			return IssuedSession{}, err
		}

		if !ipLock.IsZero() || !mailLock.IsZero() {
			return IssuedSession{}, domain.ErrTooManyAttempts
		}

		return IssuedSession{}, domain.ErrInvalidCredentials
	}

	if user.Blocked {
		return IssuedSession{}, domain.ErrBlocked
	}

	if err := a.attempts.Clear(ctx, attemptKey("ip", ip)); err != nil {
		return IssuedSession{}, err
	}

	if err := a.attempts.Clear(ctx, attemptKey("mail", normalized)); err != nil {
		return IssuedSession{}, err
	}

	return a.issue(ctx, user.ID, client, now.Add(sessionTTL))
}

func (a *UserUseCase) Resume(ctx context.Context, token, client string) (Actor, error) {
	if token == "" {
		return Actor{}, domain.ErrUnauthenticated
	}

	now := a.clock()
	if err := a.sessions.DeleteExpired(ctx, now); err != nil {
		return Actor{}, err
	}

	sess, err := a.sessions.Find(ctx, hashToken(token))
	if errors.Is(err, domain.ErrNotFound) {
		return Actor{}, domain.ErrUnauthenticated
	}

	if err != nil {
		return Actor{}, err
	}

	if sess.UserID == "" || !sess.ExpiresAt.After(now) || sess.ClientHash != clientHash(client) {
		_ = a.sessions.Delete(ctx, sess.ID)
		return Actor{}, domain.ErrUnauthenticated
	}

	user, err := a.users.FindByID(ctx, sess.UserID)
	if errors.Is(err, domain.ErrNotFound) || user.Blocked {
		_ = a.sessions.Delete(ctx, sess.ID)
		return Actor{}, domain.ErrUnauthenticated
	}
	if err != nil {
		return Actor{}, err
	}

	return Actor{
		ID:    user.ID,
		Email: user.Email,
		Name:  user.DisplayName(),
		Admin: user.Admin,
		CSRF:  sess.CSRF,
	}, nil
}

func (a *UserUseCase) userByEmail(ctx context.Context, email string) (domain.User, error) {
	normalized, err := domain.NormalizeEmail(email)
	if err != nil {
		return domain.User{}, err
	}

	return a.users.FindByEmail(ctx, normalized)
}

func (a *UserUseCase) hasAnotherActiveAdmin(ctx context.Context, exceptID string) (bool, error) {
	users, err := a.users.ListUsers(ctx)
	if err != nil {
		return false, err
	}

	for _, user := range users {
		if user.ID != exceptID && user.Admin && !user.Blocked {
			return true, nil
		}
	}

	return false, nil
}

func (a *UserUseCase) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}

	return a.sessions.Delete(ctx, hashToken(token))
}

func (a *UserUseCase) issue(ctx context.Context, userID, client string, exp time.Time) (IssuedSession, error) {
	token, err := newToken()
	if err != nil {
		return IssuedSession{}, err
	}

	csrf, err := newToken()
	if err != nil {
		return IssuedSession{}, err
	}

	if err := a.sessions.Save(ctx, domain.Session{
		ID:         hashToken(token),
		UserID:     userID,
		CSRF:       csrf,
		ClientHash: clientHash(client),
		ExpiresAt:  exp,
	}); err != nil {
		return IssuedSession{}, err
	}

	return IssuedSession{
		Token:     token,
		CSRF:      csrf,
		ExpiresAt: exp,
	}, nil
}

func (a *UserUseCase) verify(password, hash string) (bool, error) {
	if password == "" || len(password) > 512 || utf8.RuneCountInString(password) > 128 {
		return a.passwords.Verify("invalid-password", a.passwords.Dummy())
	}

	return a.passwords.Verify(password, hash)
}

func (a *UserUseCase) locked(ctx context.Context, key string, now time.Time) (bool, error) {
	until, err := a.lockTime(ctx, key, now)
	if err != nil {
		return false, err
	}

	return until.After(now), nil
}

func (a *UserUseCase) lockTime(ctx context.Context, key string, now time.Time) (time.Time, error) {
	att, err := a.attempts.Get(ctx, key)
	if errors.Is(err, domain.ErrNotFound) {
		return time.Time{}, nil
	}

	if err != nil {
		return time.Time{}, err
	}

	if att.LockedUntil.After(now) {
		return att.LockedUntil, nil
	}

	return time.Time{}, nil
}

func (a *UserUseCase) bump(ctx context.Context, key string, now time.Time) (time.Time, error) {
	att, err := a.attempts.Get(ctx, key)
	if errors.Is(err, domain.ErrNotFound) {
		att = domain.Attempt{Key: key}
	} else if err != nil {
		return time.Time{}, err
	}

	if att.LockedUntil.After(now) {
		return att.LockedUntil, nil
	}

	if !att.UpdatedAt.IsZero() && now.Sub(att.UpdatedAt) > failureWindow {
		att.Failures = 0
		att.LockedUntil = time.Time{}
	}

	att.Key = key
	att.Failures++
	att.UpdatedAt = now
	if att.Failures >= maxFailures {
		att.LockedUntil = now.Add(lockDuration)
	}

	if err := a.attempts.Put(ctx, att); err != nil {
		return time.Time{}, err
	}

	if att.LockedUntil.After(now) {
		return att.LockedUntil, nil
	}

	return time.Time{}, nil
}

func (a *UserUseCase) clock() time.Time {
	if a.now != nil {
		return a.now()
	}
	return time.Now()
}

func TokenEqual(a, b string) bool {
	da := sha256.Sum256([]byte(a))
	db := sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(da[:], db[:]) == 1
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func clientHash(client string) string {
	sum := sha256.Sum256([]byte(client))
	return hex.EncodeToString(sum[:])
}

func attemptKey(kind, value string) string {
	sum := sha256.Sum256([]byte(kind + "\x00" + value))
	return kind + ":" + hex.EncodeToString(sum[:])
}

func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func newID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return hex.EncodeToString(buf), nil
}
