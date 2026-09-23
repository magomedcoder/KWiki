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
	h := New(&stubWiki{}, auth, view, true, "README")
	mux := http.NewServeMux()
	h.Register(mux)
	srv := h.Protect(mux)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/guides/intro", nil))
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "/login?next=") {
		t.Fatalf("код %d адрес %q", rec.Code, rec.Header().Get("Location"))
	}

	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("заголовок кадра %q", rec.Header().Get("X-Frame-Options"))
	}

	if rec.Header().Get("Strict-Transport-Security") == "" {
		t.Fatal("нет строгого заголовка транспорта")
	}

	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login", nil))
	res := rec.Result()
	defer res.Body.Close()
	cookies := res.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("куки %d", len(cookies))
	}

	c := cookies[0]
	if c.Name != "__Host-kwiki_session" || !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode {
		t.Fatalf("кука %+v", c)
	}

	if !strings.Contains(view.login.CSRF, "csrf-token") {
		t.Fatalf("страница входа %+v", view.login)
	}

	body := "email=admin@example.com&password=secret&csrf=csrf-token&next=https://evil.example"
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "__Host-kwiki_session", Value: "challenge-token"})
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("вход, код %d адрес %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestEditRequiresCSRF(t *testing.T) {
	auth := &stubAuth{actor: usecase.Actor{Email: "admin@example.com", CSRF: "session-csrf"}}
	wiki := &stubWiki{}
	h := New(wiki, auth, &stubView{}, false, "README")
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
		t.Fatalf("код %d", rec.Code)
	}

	if wiki.saved {
		t.Fatal("страница сохранилась без проверочного ключа")
	}
}

func TestSafeNext(t *testing.T) {
	if got := safeNext("/guides/intro"); got != "/guides/intro" {
		t.Fatalf("получено %q", got)
	}
	for _, raw := range []string{"https://evil.example", "//evil.example", "/\\evil", "", "/a\nb"} {
		if got := safeNext(raw); got != "/" {
			t.Fatalf("из %q в %q", raw, got)
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

func (stubWiki) ViewPage(_ context.Context, branch, slug string) (usecase.PageView, error) {
	return usecase.PageView{
		Title:    "intro",
		Slug:     slug,
		Branch:   branch,
		Markdown: "Текст страницы",
		Public:   branch == "docs",
		Missing:  slug == "gone",
	}, nil
}

func (stubWiki) ListPages(_ context.Context, branch string) ([]domain.Page, error) {
	if branch != "docs" {
		return nil, nil
	}

	return []domain.Page{
		{
			Branch: branch,
			Slug:   "README",
			Title:  "Обзор",
		},
		{
			Branch: branch,
			Slug:   "intro",
			Title:  "Введение",
		},
	}, nil
}

func (stubWiki) EditForm(context.Context, string, string) (usecase.EditForm, error) {
	return usecase.EditForm{}, nil
}

func (s *stubWiki) SavePage(context.Context, string, string, string, string) (string, error) {
	s.saved = true
	return "home", nil
}

func (s *stubWiki) StageMedia(context.Context, string, string, string, string, []byte, bool) (usecase.MediaItem, error) {
	return usecase.MediaItem{}, nil
}

func (s *stubWiki) ListMedia(context.Context, string, string) ([]usecase.MediaItem, error) {
	return nil, nil
}

func (s *stubWiki) DeleteMedia(context.Context, string, string, string) error {
	return nil
}

func (s *stubWiki) ReadMedia(context.Context, string, string, string) ([]byte, string, error) {
	return nil, "", domain.ErrNotFound
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

func (s *stubAuth) UpdateUser(context.Context, string, string, usecase.Account, bool) error {
	return nil
}

func (s *stubAuth) ChangeOwnPassword(context.Context, string, string, string) error {
	return nil
}

func (s *stubAuth) SetBlocked(context.Context, string, string, bool) error {
	return nil
}

type stubView struct {
	login     usecase.LoginPage
	index     usecase.IndexView
	page      usecase.PageScreen
	previewed string
}

func (s *stubView) Index(_ http.ResponseWriter, view usecase.IndexView, _ usecase.Actor) {
	s.index = view
}

func (s *stubView) Page(_ http.ResponseWriter, view usecase.PageScreen, _ usecase.Actor) {
	s.page = view
}

func (stubView) Edit(http.ResponseWriter, usecase.EditForm, []domain.Branch, usecase.Actor) {}

func (s *stubView) Preview(w http.ResponseWriter, content, _ string) {
	s.previewed = content
	_, _ = w.Write([]byte(content))
}

func (stubView) Users(http.ResponseWriter, usecase.UsersPage, usecase.Actor) {}

func (stubView) Branches(http.ResponseWriter, usecase.BranchesPage, usecase.Actor) {}

func (stubView) NotFound(w http.ResponseWriter, _ usecase.Actor) {
	w.WriteHeader(http.StatusNotFound)
}

func (s *stubView) History(_ http.ResponseWriter, view usecase.PageScreen, _ usecase.Actor) {
	s.page = view
}

func (s *stubView) Login(_ http.ResponseWriter, page usecase.LoginPage) {
	s.login = page
}

func TestEditPreviewRequiresCSRF(t *testing.T) {
	view := &stubView{}
	auth := &stubAuth{actor: usecase.Actor{Email: "admin@example.com", CSRF: "session-csrf"}}
	h := New(&stubWiki{}, auth, view, false, "README")
	mux := http.NewServeMux()
	h.Register(mux)
	srv := h.Protect(mux)

	req := httptest.NewRequest(http.MethodPost, "/edit/preview", strings.NewReader("content=%23+Hi"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "kwiki_session", Value: "session-token"})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || view.previewed != "" {
		t.Fatalf("код %d просмотр %q", rec.Code, view.previewed)
	}

	req = httptest.NewRequest(http.MethodPost, "/edit/preview", strings.NewReader("csrf=session-csrf&content=%23+Hi"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "kwiki_session", Value: "session-token"})
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || view.previewed != "# Hi" {
		t.Fatalf("код %d просмотр %q", rec.Code, view.previewed)
	}
}

func TestHistoryPage(t *testing.T) {
	view := &stubView{}
	h := New(&stubWiki{}, &stubAuth{}, view, false, "README")
	mux := http.NewServeMux()
	h.Register(mux)
	srv := h.Protect(mux)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/history?branch=docs&slug=intro", nil))
	if rec.Code != http.StatusOK || view.page.Slug != "intro" || view.page.ReadHref != "/b/docs/intro" || view.page.HistoryHref != "/history?branch=docs&slug=intro" {
		t.Fatalf("код %d страница %+v", rec.Code, view.page)
	}

	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/history?slug=intro", nil))
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/login?next=") {
		t.Fatalf("приватная история %d %q", rec.Code, rec.Header().Get("Location"))
	}

	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/history?branch=docs&slug=gone", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("пустая история, код %d", rec.Code)
	}
}

func TestMissingPublicPageIsNotFound(t *testing.T) {
	h := New(&stubWiki{}, &stubAuth{}, &stubView{}, false, "README")
	mux := http.NewServeMux()
	h.Register(mux)

	rec := httptest.NewRecorder()
	h.Protect(mux).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/b/docs/gone", nil))
	if rec.Code != http.StatusNotFound || strings.Contains(rec.Header().Get("Location"), "/login") {
		t.Fatalf("код %d адрес %q", rec.Code, rec.Header().Get("Location"))
	}

	rec = httptest.NewRecorder()
	h.Protect(mux).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/b", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("пустой адрес ветки, код %d", rec.Code)
	}
}

func TestFaviconDoesNotOpenLogin(t *testing.T) {
	h := New(&stubWiki{}, &stubAuth{}, &stubView{}, false, "README")
	mux := http.NewServeMux()
	h.Register(mux)

	rec := httptest.NewRecorder()
	h.Protect(mux).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/favicon.ico", nil))
	if rec.Code != http.StatusNotFound || strings.Contains(rec.Header().Get("Location"), "/login") {
		t.Fatalf("код %d адрес %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestPublicBranchIsIndexed(t *testing.T) {
	view := &stubView{}
	h := New(&stubWiki{}, &stubAuth{}, view, true, "README")
	mux := http.NewServeMux()
	h.Register(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/b/docs/intro", nil)
	req.Host = "wiki.example"
	h.Protect(mux).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("код %d", rec.Code)
	}

	if !strings.Contains(rec.Header().Get("X-Robots-Tag"), "index") {
		t.Fatalf("роботы %q", rec.Header().Get("X-Robots-Tag"))
	}

	if !view.page.Indexable || view.page.Canonical != "https://wiki.example/b/docs/intro" {
		t.Fatalf("страница %+v", view.page)
	}

	if !publicPageListed(view.page.Pages, "/b/docs/intro", true) || !publicPageListed(view.page.Pages, "/b/docs", false) {
		t.Fatalf("страницы публичной ветки %+v", view.page.Pages)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "wiki.example"
	h.Protect(mux).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !view.index.Catalog || !view.index.Indexable {
		t.Fatalf("каталог, код %d вид %+v", rec.Code, view.index)
	}

	if len(view.index.Branches) != 1 || view.index.Branches[0].Name != "docs" || !publicPageListed(view.index.Branches[0].Pages, "/b/docs/intro", false) {
		t.Fatalf("каталог страниц %+v", view.index.Branches)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	req.Host = "wiki.example"
	h.Protect(mux).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "https://wiki.example/b/docs/intro") {
		t.Fatalf("карта сайта %d %s", rec.Code, rec.Body.String())
	}

	if strings.Contains(rec.Body.String(), "/b/secret") {
		t.Fatal("в списке есть приватная ветка")
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	req.Host = "wiki.example"
	h.Protect(mux).ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "Sitemap: https://wiki.example/sitemap.xml") || !strings.Contains(body, "Disallow: /edit") || !strings.Contains(body, "Disallow: /history") || !strings.Contains(body, "Disallow: /media") {
		t.Fatalf("файл роботов %s", body)
	}

	rec = httptest.NewRecorder()
	h.Protect(mux).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/b/main/intro", nil))
	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/intro" {
		t.Fatalf("перенаправление %d %q", rec.Code, rec.Header().Get("Location"))
	}

	rec = httptest.NewRecorder()
	h.Protect(mux).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/intro", nil))
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/login?next=") {
		t.Fatalf("приватная страница %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func publicPageListed(pages []usecase.PageLink, href string, current bool) bool {
	for _, page := range pages {
		if page.Href == href && page.Current == current {
			return true
		}
	}

	return false
}

func TestBranchRootShowsHomeFile(t *testing.T) {
	view := &stubView{}
	h := New(&stubWiki{}, &stubAuth{}, view, true, "README")
	mux := http.NewServeMux()
	h.Register(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/b/docs", nil)
	req.Host = "wiki.example"
	h.Protect(mux).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !view.page.Home || view.page.Slug != "README" || view.page.Canonical != "https://wiki.example/b/docs" {
		t.Fatalf("код %d страница %+v", rec.Code, view.page)
	}

	rec = httptest.NewRecorder()
	h.Protect(mux).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/b/docs/README", nil))
	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/b/docs" {
		t.Fatalf("перенаправление %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestChangePasswordClearsSession(t *testing.T) {
	auth := &stubAuth{
		actor: usecase.Actor{
			ID:    "user-1",
			Email: "admin@example.com",
			CSRF:  "session-csrf",
		},
	}
	h := New(&stubWiki{}, auth, &stubView{}, false, "README")
	mux := http.NewServeMux()
	h.Register(mux)
	srv := h.Protect(mux)

	mismatch := httptest.NewRequest(http.MethodPost, "/account/password", strings.NewReader("csrf=session-csrf&current_password=old&password=N3w-Wiki-Password&password_confirm=other"))
	mismatch.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mismatch.AddCookie(&http.Cookie{
		Name:  "kwiki_session",
		Value: "session-token",
	})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, mismatch)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "Пароли не совпадают") {
		t.Fatalf("несовпадение %d %s", rec.Code, rec.Body.String())
	}

	ok := httptest.NewRequest(http.MethodPost, "/account/password", strings.NewReader("csrf=session-csrf&current_password=old&password=N3w-Wiki-Password&password_confirm=N3w-Wiki-Password"))
	ok.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ok.AddCookie(&http.Cookie{
		Name:  "kwiki_session",
		Value: "session-token",
	})
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, ok)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("смена пароля %d %s", rec.Code, rec.Body.String())
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge >= 0 {
		t.Fatalf("кука %+v", cookies)
	}
}

func TestEmbeddedAssets(t *testing.T) {
	h := New(&stubWiki{}, &stubAuth{}, &stubView{}, false, "README")
	mux := http.NewServeMux()
	h.Register(mux)

	css := httptest.NewRecorder()
	mux.ServeHTTP(css, httptest.NewRequest(http.MethodGet, "/css/tailwindcss.css", nil))
	if css.Code != http.StatusOK || !strings.Contains(css.Body.String(), "wiki-btn") || !strings.Contains(css.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("стиль %d %s", css.Code, css.Header().Get("Content-Type"))
	}

	script := httptest.NewRecorder()
	mux.ServeHTTP(script, httptest.NewRequest(http.MethodGet, "/js/main.js", nil))
	if script.Code != http.StatusOK || !strings.Contains(script.Body.String(), "openDialog") {
		t.Fatalf("скрипт %d", script.Code)
	}

	missing := httptest.NewRecorder()
	mux.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/css/missing.css", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("чужой файл %d", missing.Code)
	}
}

func TestFoldHome(t *testing.T) {
	updated := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	got := foldHome([]usecase.SitemapEntry{
		{Path: "/"},
		{Path: "/README", Updated: updated},
		{Path: "/b/docs"},
		{Path: "/b/docs/README", Updated: updated},
		{Path: "/b/docs/guide"},
	}, "README")
	if len(got) != 3 || got[0].Path != "/" || !got[0].Updated.Equal(updated) || got[1].Path != "/b/docs" || got[2].Path != "/b/docs/guide" {
		t.Fatalf("карта сайта %+v", got)
	}
}
