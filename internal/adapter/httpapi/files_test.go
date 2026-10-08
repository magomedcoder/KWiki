package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/magomedcoder/kwiki/internal/usecase"
)

func TestFilesPageRequiresAuth(t *testing.T) {
	h := New(&stubWiki{}, &stubAuth{}, &stubView{}, false, "README", testBundle(t))
	mux := http.NewServeMux()
	h.Register(mux)

	rec := httptest.NewRecorder()
	h.Protect(mux).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/b/docs/files", nil))
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "/login") {
		t.Fatalf("код %d loc=%s", rec.Code, rec.Header().Get("Location"))
	}

	auth := &stubAuth{actor: usecase.Actor{Email: "a@example.com", CSRF: "token"}}
	h = New(&stubWiki{}, auth, &stubView{}, false, "README", testBundle(t))
	mux = http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest(http.MethodGet, "/b/docs/files", nil)
	req.AddCookie(&http.Cookie{Name: "kwiki_session", Value: "session"})
	rec = httptest.NewRecorder()
	h.Protect(mux).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "files-ok") {
		t.Fatalf("код %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestFilesCreateRequiresCSRF(t *testing.T) {
	auth := &stubAuth{actor: usecase.Actor{Email: "a@example.com", CSRF: "token"}}
	h := New(&stubWiki{}, auth, &stubView{}, false, "README", testBundle(t))
	mux := http.NewServeMux()
	h.Register(mux)

	form := url.Values{"action": {"create"}, "name": {"a.md"}, "content": {"x"}, "csrf": {"bad"}}
	req := httptest.NewRequest(http.MethodPost, "/files", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "kwiki_session", Value: "session"})
	rec := httptest.NewRecorder()
	h.Protect(mux).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("код %d", rec.Code)
	}
}

func TestSplitFilesPath(t *testing.T) {
	branch, rel, ok := splitFilesPath("/files")
	if !ok || branch != "main" || rel != "" {
		t.Fatalf("%s %s %v", branch, rel, ok)
	}

	branch, rel, ok = splitFilesPath("/files/media/a.png")
	if !ok || branch != "main" || rel != "media/a.png" {
		t.Fatalf("%s %s %v", branch, rel, ok)
	}

	branch, rel, ok = splitFilesPath("/b/docs/files/notes.md")
	if !ok || branch != "docs" || rel != "notes.md" {
		t.Fatalf("%s %s %v", branch, rel, ok)
	}
}
