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
	req.AddCookie(&http.Cookie{
		Name:  "kwiki_session",
		Value: "session-token",
	})
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

func (stubWiki) OpenBranch(_ context.Context, name string) (domain.Branch, error) {
	switch name {
	case domain.DefaultBranch:
		return domain.Branch{Name: name, Public: false}, nil
	case "docs":
		return domain.Branch{Name: name, Public: true}, nil
	default:
		return domain.Branch{}, domain.ErrNotFound
	}
}

func (stubWiki) VisibleBranches(_ context.Context, authenticated bool) ([]domain.Branch, error) {
	docs := domain.Branch{Name: "docs", Public: true}
	if !authenticated {
		return []domain.Branch{docs}, nil
	}
	return []domain.Branch{
		{Name: domain.DefaultBranch, Public: false},
		docs,
	}, nil
}

func (stubWiki) ListBranches(context.Context) ([]domain.Branch, error) {
	return []domain.Branch{{Name: domain.DefaultBranch}, {Name: "docs", Public: true}}, nil
}

func (stubWiki) ListPages(context.Context, string) ([]domain.Page, error) {
	return nil, nil
}

func (stubWiki) ViewPage(_ context.Context, branch, slug string) (usecase.PageView, error) {
	return usecase.PageView{
		Title:    "intro",
		Slug:     slug,
		Branch:   branch,
		Markdown: "Текст страницы",
		Public:   branch == "docs",
	}, nil
}

func (stubWiki) EditForm(context.Context, string, string) (usecase.EditForm, error) {
	return usecase.EditForm{}, nil
}

func (s *stubWiki) SavePage(context.Context, string, string, string) (string, error) {
	s.saved = true
	return "home", nil
}

func (stubWiki) CreateBranch(context.Context, string, bool) (domain.Branch, error) {
	return domain.Branch{}, nil
}

func (stubWiki) SetBranchPublic(context.Context, string, bool) error {
	return nil
}

func (stubWiki) DeleteBranch(context.Context, string) error {
	return nil
}

func (stubWiki) Sitemap(context.Context) ([]usecase.SitemapEntry, error) {
	return []usecase.SitemapEntry{{Path: "/b/docs"}, {Path: "/b/docs/intro"}}, nil
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

func (s *stubAuth) Logout(context.Context, string) error {
	return nil
}

func (s *stubAuth) CreateUser(context.Context, usecase.Account) error {
	return nil
}

func (s *stubAuth) ListUsers(context.Context, string) ([]usecase.ManagedUser, error) {
	return nil, nil
}

func (s *stubAuth) DeleteUser(context.Context, string, string) error {
	return nil
}

func (s *stubAuth) SetBlocked(context.Context, string, string, bool) error {
	return nil
}

type stubView struct {
	login usecase.LoginPage
	index usecase.IndexView
	page  usecase.PageScreen
}

func (s *stubView) Index(_ http.ResponseWriter, view usecase.IndexView, _ usecase.Actor) {
	s.index = view
}

func (s *stubView) Page(_ http.ResponseWriter, view usecase.PageScreen, _ usecase.Actor) {
	s.page = view
}

func (stubView) Edit(http.ResponseWriter, usecase.EditForm, []domain.Branch, usecase.Actor) {}

func (stubView) Users(http.ResponseWriter, usecase.UsersPage, usecase.Actor) {}

func (stubView) Branches(http.ResponseWriter, usecase.BranchesPage, usecase.Actor) {}

func (s *stubView) Login(_ http.ResponseWriter, page usecase.LoginPage) {
	s.login = page
}

func TestPublicBranchIsIndexed(t *testing.T) {
	view := &stubView{}
	h := New(&stubWiki{}, &stubAuth{}, view, true)
	mux := http.NewServeMux()
	h.Register(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/b/docs/intro", nil)
	req.Host = "wiki.example"
	h.Protect(mux).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}

	if !strings.Contains(rec.Header().Get("X-Robots-Tag"), "index") {
		t.Fatalf("robots %q", rec.Header().Get("X-Robots-Tag"))
	}

	if !view.page.Indexable || view.page.Canonical != "https://wiki.example/b/docs/intro" {
		t.Fatalf("page %+v", view.page)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "wiki.example"
	h.Protect(mux).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !view.index.Catalog || !view.index.Indexable {
		t.Fatalf("catalog status %d view %+v", rec.Code, view.index)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	req.Host = "wiki.example"
	h.Protect(mux).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "https://wiki.example/b/docs/intro") {
		t.Fatalf("sitemap %d %s", rec.Code, rec.Body.String())
	}

	if strings.Contains(rec.Body.String(), "/b/secret") {
		t.Fatal("private branch listed")
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	req.Host = "wiki.example"
	h.Protect(mux).ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "Sitemap: https://wiki.example/sitemap.xml") || !strings.Contains(body, "Disallow: /edit") {
		t.Fatalf("robots.txt %s", body)
	}

	rec = httptest.NewRecorder()
	h.Protect(mux).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/b/main/intro", nil))
	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/intro" {
		t.Fatalf("redirect %d %q", rec.Code, rec.Header().Get("Location"))
	}
}
