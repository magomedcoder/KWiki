package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/magomedcoder/kwiki/internal/domain"
	"github.com/magomedcoder/kwiki/internal/usecase"
)

func TestAnonymousRedirectAndSecureCookie(t *testing.T) {
	auth := &stubAuth{
		begin: usecase.IssuedSession{
			Token:     "challenge-token",
			CSRF:      "csrf-token",
			ExpiresAt: time.Now().Add(time.Minute),
		},
		issued: usecase.IssuedSession{
			Token:     "session-token",
			CSRF:      "session-csrf",
			ExpiresAt: time.Now().Add(time.Hour),
		},
	}
	view := &stubView{}
	h := New(&stubWiki{}, auth, view, true)
	mux := http.NewServeMux()
	h.Register(mux)
	srv := h.Protect(mux)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/guides/intro", nil))
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "/login?next=") {
		t.Fatalf("status %d location %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("frame header %q", rec.Header().Get("X-Frame-Options"))
	}
	if rec.Header().Get("Strict-Transport-Security") == "" {
		t.Fatal("missing HSTS")
	}

	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login", nil))
	res := rec.Result()
	defer res.Body.Close()
	cookies := res.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies %d", len(cookies))
	}
	c := cookies[0]
	if c.Name != "__Host-kwiki_session" || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie %+v", c)
	}
	if !strings.Contains(view.login.CSRF, "csrf-token") {
		t.Fatalf("login page %+v", view.login)
	}

	body := "email=admin@example.com&password=secret&csrf=csrf-token&next=https://evil.example"
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "__Host-kwiki_session", Value: "challenge-token"})
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("login status %d location %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestEditRequiresCSRF(t *testing.T) {
	auth := &stubAuth{actor: usecase.Actor{Email: "admin@example.com", CSRF: "session-csrf"}}
	wiki := &stubWiki{}
	h := New(wiki, auth, &stubView{}, false)
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodPost, "/edit", strings.NewReader("slug=home&content=text"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "kwiki_session", Value: "session-token"})
	rec := httptest.NewRecorder()
	h.Protect(mux).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d", rec.Code)
	}
	if wiki.saved {
		t.Fatal("page was saved without csrf")
	}
}

func TestSafeNext(t *testing.T) {
	if got := safeNext("/guides/intro"); got != "/guides/intro" {
		t.Fatalf("got %q", got)
	}
	for _, raw := range []string{"https://evil.example", "//evil.example", "/\\evil", "", "/a\nb"} {
		if got := safeNext(raw); got != "/" {
			t.Fatalf("%q -> %q", raw, got)
		}
	}
}

type stubWiki struct {
	saved bool
}

func (stubWiki) ListPages(context.Context) ([]domain.Page, error) { return nil, nil }
func (stubWiki) ViewPage(context.Context, string) (usecase.PageView, error) {
	return usecase.PageView{}, nil
}
func (stubWiki) EditForm(context.Context, string) (usecase.EditForm, error) {
	return usecase.EditForm{}, nil
}
func (s *stubWiki) SavePage(context.Context, string, string) (string, error) {
	s.saved = true
	return "home", nil
}

type stubAuth struct {
	actor  usecase.Actor
	begin  usecase.IssuedSession
	issued usecase.IssuedSession
}

func (s *stubAuth) BeginLogin(context.Context, string, string) (usecase.IssuedSession, error) {
	return s.begin, nil
}
func (s *stubAuth) Login(context.Context, string, string, string, string, string, string) (usecase.IssuedSession, error) {
	return s.issued, nil
}
func (s *stubAuth) Resume(context.Context, string, string) (usecase.Actor, error) {
	if s.actor.Email == "" {
		return usecase.Actor{}, domain.ErrUnauthenticated
	}
	return s.actor, nil
}
func (s *stubAuth) Logout(context.Context, string) error { return nil }

type stubView struct {
	login usecase.LoginPage
}

func (stubView) Index(http.ResponseWriter, []domain.Page, usecase.Actor)   {}
func (stubView) Page(http.ResponseWriter, usecase.PageView, usecase.Actor) {}
func (stubView) Edit(http.ResponseWriter, usecase.EditForm, usecase.Actor) {}
func (s *stubView) Login(_ http.ResponseWriter, page usecase.LoginPage) {
	s.login = page
}
