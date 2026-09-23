package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/magomedcoder/kwiki/internal/domain"
)

func TestLoginSuccessAndClientBinding(t *testing.T) {
	ctx := context.Background()
	auth, users, sessions := newTestAuth()
	createUser(t, auth, "Admin@Example.com", "S3cure-Wiki-Pass")

	saved, ok := users.byEmail["admin@example.com"]
	if !ok || saved.FirstName != "Иван" || saved.LastName != "Иванов" || !saved.Admin {
		t.Fatalf("сохранённый пользователь %+v", saved)
	}

	challenge, err := auth.BeginLogin(ctx, "", "browser")
	if err != nil {
		t.Fatal(err)
	}

	issued, err := auth.Login(ctx, challenge.Token, challenge.CSRF, "admin@example.com", "S3cure-Wiki-Pass", "127.0.0.1", "browser")
	if err != nil {
		t.Fatal(err)
	}

	if issued.Token == challenge.Token || issued.Token == "" {
		t.Fatalf("сессия не сменилась: %+v", issued)
	}

	if _, err := sessions.Find(ctx, hashToken(challenge.Token)); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("проверочная сессия ещё есть: %v", err)
	}

	actor, err := auth.Resume(ctx, issued.Token, "browser")
	if err != nil || actor.Email != "admin@example.com" || actor.CSRF != issued.CSRF {
		t.Fatalf("участник %+v ошибка %v", actor, err)
	}

	if _, err := auth.Resume(ctx, issued.Token, "other"); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("другой клиент, ошибка %v", err)
	}
}

func TestBeginLoginReusesOpenChallenge(t *testing.T) {
	ctx := context.Background()
	auth, _, sessions := newTestAuth()
	first, err := auth.BeginLogin(ctx, "", "browser")
	if err != nil {
		t.Fatal(err)
	}

	second, err := auth.BeginLogin(ctx, first.Token, "browser")
	if err != nil {
		t.Fatal(err)
	}

	if second.Token != first.Token || second.CSRF != first.CSRF {
		t.Fatalf("проверочная сессия сменилась: %+v затем %+v", first, second)
	}

	if len(sessions.items) != 1 {
		t.Fatalf("сессий %d", len(sessions.items))
	}

	other, err := auth.BeginLogin(ctx, first.Token, "other-browser")
	if err != nil {
		t.Fatal(err)
	}

	if other.Token == first.Token || other.CSRF == first.CSRF {
		t.Fatalf("чужой клиент повторил проверочную сессию: %+v", other)
	}
}

func TestLoginRejectsBadCSRF(t *testing.T) {
	auth, _, _ := newTestAuth()
	challenge, err := auth.BeginLogin(context.Background(), "", "browser")
	if err != nil {
		t.Fatal(err)
	}

	_, err = auth.Login(context.Background(), challenge.Token, "wrong", "admin@example.com", "S3cure-Wiki-Pass", "127.0.0.1", "browser")
	if !errors.Is(err, domain.ErrCSRF) {
		t.Fatalf("ошибка %v", err)
	}
}

func TestLoginHidesUnknownUser(t *testing.T) {
	auth, _, _ := newTestAuth()
	createUser(t, auth, "admin@example.com", "S3cure-Wiki-Pass")

	unknown := loginOnce(t, auth, "missing@example.com", "S3cure-Wiki-Pass")
	wrong := loginOnce(t, auth, "admin@example.com", "Wrong-Password-1")
	if !errors.Is(unknown, domain.ErrInvalidCredentials) || !errors.Is(wrong, domain.ErrInvalidCredentials) {
		t.Fatalf("неизвестный %v неверный %v", unknown, wrong)
	}
}

func TestLoginLockout(t *testing.T) {
	auth, _, _ := newTestAuth()
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	auth.now = func() time.Time { return now }
	createUser(t, auth, "admin@example.com", "S3cure-Wiki-Pass")

	var last error
	for range maxFailures {
		last = loginOnce(t, auth, "admin@example.com", "Wrong-Password-1")
	}

	if !errors.Is(last, domain.ErrTooManyAttempts) {
		t.Fatalf("последняя ошибка %v", last)
	}

	if err := loginOnce(t, auth, "admin@example.com", "S3cure-Wiki-Pass"); !errors.Is(err, domain.ErrTooManyAttempts) {
		t.Fatalf("вход при ограничении, ошибка %v", err)
	}

	now = now.Add(lockDuration + time.Second)
	issued := mustLogin(t, auth, "admin@example.com", "S3cure-Wiki-Pass")
	if issued.Token == "" {
		t.Fatal("пустой ключ после ограничения")
	}
}

func TestChangePasswordRevokesSession(t *testing.T) {
	ctx := context.Background()
	auth, _, _ := newTestAuth()
	createUser(t, auth, "admin@example.com", "S3cure-Wiki-Pass")

	issued := mustLogin(t, auth, "admin@example.com", "S3cure-Wiki-Pass")
	if err := auth.ChangePassword(ctx, "admin@example.com", "N3w-Wiki-Password"); err != nil {
		t.Fatal(err)
	}

	if _, err := auth.Resume(ctx, issued.Token, "browser"); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("старая сессия, ошибка %v", err)
	}

	if err := loginOnce(t, auth, "admin@example.com", "S3cure-Wiki-Pass"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("старый пароль, ошибка %v", err)
	}
}

func TestUpdateUserAndOwnPassword(t *testing.T) {
	ctx := context.Background()
	auth, users, _ := newTestAuth()
	createUser(t, auth, "admin@example.com", "S3cure-Wiki-Pass")
	if err := auth.CreateUser(ctx, Account{
		FirstName: "Пётр",
		LastName:  "Петров",
		Email:     "petr@example.com",
		Password:  "S3cure-Wiki-Pass",
	}); err != nil {
		t.Fatal(err)
	}

	admin := users.byEmail["admin@example.com"]
	issued := mustLogin(t, auth, "petr@example.com", "S3cure-Wiki-Pass")
	if err := auth.UpdateUser(ctx, admin.ID, "petr@example.com", Account{
		FirstName: "Пётр",
		LastName:  "Сидоров",
		Email:     "sidor@example.com",
		Admin:     true,
		Password:  "N3w-Wiki-Password",
	}, true); err != nil {
		t.Fatal(err)
	}

	saved, err := users.FindByEmail(ctx, "sidor@example.com")
	if err != nil || saved.LastName != "Сидоров" || !saved.Admin || !saved.Blocked {
		t.Fatalf("профиль %+v ошибка %v", saved, err)
	}

	if _, err := users.FindByEmail(ctx, "petr@example.com"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("старая почта, ошибка %v", err)
	}

	if _, err := auth.Resume(ctx, issued.Token, "browser"); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("сессия после смены пароля, ошибка %v", err)
	}

	if err := auth.UpdateUser(ctx, admin.ID, "sidor@example.com", Account{
		FirstName: "Пётр",
		LastName:  "Сидоров",
		Email:     "admin@example.com",
	}, false); !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("занятая почта, ошибка %v", err)
	}

	if err := auth.UpdateUser(ctx, admin.ID, "admin@example.com", Account{
		FirstName: "Иван",
		LastName:  "Иванов",
		Email:     "admin@example.com",
	}, false); !errors.Is(err, domain.ErrLastAdmin) {
		t.Fatalf("снятие прав последнего администратора, ошибка %v", err)
	}

	if err := auth.UpdateUser(ctx, admin.ID, "admin@example.com", Account{
		FirstName: "Иван",
		LastName:  "Иванов",
		Email:     "admin@example.com",
		Admin:     true,
	}, true); !errors.Is(err, domain.ErrSelfAction) {
		t.Fatalf("блокировка себя, ошибка %v", err)
	}

	own := mustLogin(t, auth, "admin@example.com", "S3cure-Wiki-Pass")
	if err := auth.ChangeOwnPassword(ctx, admin.ID, "wrong-password", "N3w-Wiki-Password"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("чужой текущий пароль, ошибка %v", err)
	}

	if err := auth.ChangeOwnPassword(ctx, admin.ID, "S3cure-Wiki-Pass", "N3w-Wiki-Password"); err != nil {
		t.Fatal(err)
	}

	if _, err := auth.Resume(ctx, own.Token, "browser"); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("сессия после своего пароля, ошибка %v", err)
	}

	if err := loginOnce(t, auth, "admin@example.com", "N3w-Wiki-Password"); err != nil {
		t.Fatal(err)
	}
}

func TestBlockAndDeleteUser(t *testing.T) {
	ctx := context.Background()
	auth, users, sessions := newTestAuth()
	createUser(t, auth, "admin@example.com", "S3cure-Wiki-Pass")
	if err := auth.CreateUser(ctx, Account{
		FirstName: "Пётр",
		LastName:  "Петров",
		Email:     "petr@example.com",
		Password:  "S3cure-Wiki-Pass",
	}); err != nil {
		t.Fatal(err)
	}
	if users.byEmail["petr@example.com"].Admin {
		t.Fatal("второй пользователь стал администратором")
	}

	issued := mustLogin(t, auth, "petr@example.com", "S3cure-Wiki-Pass")
	admin := users.byEmail["admin@example.com"]
	if err := auth.SetBlocked(ctx, admin.ID, "petr@example.com", true); err != nil {
		t.Fatal(err)
	}

	if _, err := auth.Resume(ctx, issued.Token, "browser"); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("сессия заблокированного, ошибка %v", err)
	}

	if err := loginOnce(t, auth, "petr@example.com", "S3cure-Wiki-Pass"); !errors.Is(err, domain.ErrBlocked) {
		t.Fatalf("вход заблокированного, ошибка %v", err)
	}

	if err := auth.SetBlocked(ctx, admin.ID, "admin@example.com", true); !errors.Is(err, domain.ErrSelfAction) {
		t.Fatalf("блокировка себя, ошибка %v", err)
	}

	if err := auth.SetBlocked(ctx, "", "admin@example.com", true); !errors.Is(err, domain.ErrLastAdmin) {
		t.Fatalf("блокировка последнего администратора, ошибка %v", err)
	}

	if err := auth.SetBlocked(ctx, "", "petr@example.com", false); err != nil {
		t.Fatal(err)
	}

	if err := auth.DeleteUser(ctx, admin.ID, "petr@example.com"); err != nil {
		t.Fatal(err)
	}

	if _, err := users.FindByEmail(ctx, "petr@example.com"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("удалённый пользователь, ошибка %v", err)
	}

	if err := auth.DeleteUser(ctx, "", "admin@example.com"); !errors.Is(err, domain.ErrLastAdmin) {
		t.Fatalf("удаление последнего администратора, ошибка %v", err)
	}

	if len(sessions.items) != 0 {
		t.Fatalf("осталось сессий: %d", len(sessions.items))
	}
}

func createUser(t *testing.T, auth *UserUseCase, email, password string) {
	t.Helper()
	err := auth.CreateUser(context.Background(), Account{
		FirstName: "Иван",
		LastName:  "Иванов",
		Email:     email,
		Password:  password,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func loginOnce(t *testing.T, auth *UserUseCase, email, password string) error {
	t.Helper()
	challenge, err := auth.BeginLogin(context.Background(), "", "browser")
	if err != nil {
		t.Fatal(err)
	}

	_, err = auth.Login(context.Background(), challenge.Token, challenge.CSRF, email, password, "127.0.0.1", "browser")
	return err
}

func mustLogin(t *testing.T, auth *UserUseCase, email, password string) IssuedSession {
	t.Helper()
	challenge, err := auth.BeginLogin(context.Background(), "", "browser")
	if err != nil {
		t.Fatal(err)
	}

	issued, err := auth.Login(context.Background(), challenge.Token, challenge.CSRF, email, password, "127.0.0.1", "browser")
	if err != nil {
		t.Fatal(err)
	}

	return issued
}

func newTestAuth() (*UserUseCase, *memUsers, *memSessions) {
	users := newMemUsers()
	sessions := newMemSessions()
	attempts := newMemAttempts()
	return NewAuth(users, sessions, attempts, textHasher{}), users, sessions
}

type textHasher struct{}

func (textHasher) Hash(password string) (string, error) {
	return "h:" + password, nil
}

func (textHasher) Verify(password, encoded string) (bool, error) {
	return encoded == "h:"+password, nil
}

func (textHasher) Dummy() string {
	return "h:$dummy"
}

type memUsers struct {
	byEmail map[string]domain.User
	byID    map[string]domain.User
}

func newMemUsers() *memUsers {
	return &memUsers{byEmail: map[string]domain.User{}, byID: map[string]domain.User{}}
}

func (m *memUsers) Create(_ context.Context, user domain.User) error {
	if _, ok := m.byEmail[user.Email]; ok {
		return domain.ErrEmailTaken
	}

	m.byEmail[user.Email] = user
	m.byID[user.ID] = user
	return nil
}

func (m *memUsers) FindByEmail(_ context.Context, email string) (domain.User, error) {
	user, ok := m.byEmail[email]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}

	return user, nil
}

func (m *memUsers) FindByID(_ context.Context, id string) (domain.User, error) {
	user, ok := m.byID[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}

	return user, nil
}

func (m *memUsers) UpdateProfile(_ context.Context, id, firstName, lastName, email string) error {
	user, ok := m.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	if other, exists := m.byEmail[email]; exists && other.ID != id {
		return domain.ErrEmailTaken
	}
	delete(m.byEmail, user.Email)
	user.FirstName = firstName
	user.LastName = lastName
	user.Email = email
	m.byID[id] = user
	m.byEmail[email] = user
	return nil
}

func (m *memUsers) UpdatePasswordHash(_ context.Context, id, hash string) error {
	user, ok := m.byID[id]
	if !ok {
		return domain.ErrNotFound
	}

	user.PasswordHash = hash
	m.byID[id] = user
	m.byEmail[user.Email] = user
	return nil
}

func (m *memUsers) SetBlocked(_ context.Context, id string, blocked bool) error {
	user, ok := m.byID[id]
	if !ok {
		return domain.ErrNotFound
	}

	user.Blocked = blocked
	m.byID[id] = user
	m.byEmail[user.Email] = user
	return nil
}

func (m *memUsers) SetAdmin(_ context.Context, id string, admin bool) error {
	user, ok := m.byID[id]
	if !ok {
		return domain.ErrNotFound
	}

	user.Admin = admin
	m.byID[id] = user
	m.byEmail[user.Email] = user
	return nil
}

func (m *memUsers) DeleteUser(_ context.Context, id string) error {
	user, ok := m.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	delete(m.byID, id)
	delete(m.byEmail, user.Email)
	return nil
}

func (m *memUsers) ListUsers(context.Context) ([]domain.User, error) {
	out := make([]domain.User, 0, len(m.byID))
	for _, user := range m.byID {
		out = append(out, user)
	}
	return out, nil
}

func (m *memUsers) Count(context.Context) (int64, error) {
	return int64(len(m.byID)), nil
}

type memSessions struct {
	items map[string]domain.Session
}

func newMemSessions() *memSessions {
	return &memSessions{items: map[string]domain.Session{}}
}

func (m *memSessions) Save(_ context.Context, session domain.Session) error {
	m.items[session.ID] = session
	return nil
}

func (m *memSessions) Find(_ context.Context, id string) (domain.Session, error) {
	session, ok := m.items[id]
	if !ok {
		return domain.Session{}, domain.ErrNotFound
	}

	return session, nil
}

func (m *memSessions) Delete(_ context.Context, id string) error {
	delete(m.items, id)
	return nil
}

func (m *memSessions) DeleteByUser(_ context.Context, userID string) error {
	for id, session := range m.items {
		if session.UserID == userID {
			delete(m.items, id)
		}
	}

	return nil
}

func (m *memSessions) DeleteExpired(_ context.Context, now time.Time) error {
	for id, session := range m.items {
		if !session.ExpiresAt.After(now) {
			delete(m.items, id)
		}
	}

	return nil
}

type memAttempts struct {
	items map[string]domain.Attempt
}

func newMemAttempts() *memAttempts {
	return &memAttempts{items: map[string]domain.Attempt{}}
}

func (m *memAttempts) Get(_ context.Context, key string) (domain.Attempt, error) {
	item, ok := m.items[key]
	if !ok {
		return domain.Attempt{}, domain.ErrNotFound
	}

	return item, nil
}

func (m *memAttempts) Put(_ context.Context, attempt domain.Attempt) error {
	m.items[attempt.Key] = attempt
	return nil
}

func (m *memAttempts) Clear(_ context.Context, key string) error {
	delete(m.items, key)
	return nil
}
