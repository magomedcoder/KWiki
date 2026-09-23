package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/magomedcoder/kwiki/internal/domain"
)

func TestMediaPrivateDenied(t *testing.T) {
	h := New(&stubWiki{}, &stubAuth{}, &stubView{}, false, "README")
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/b/main/media/a.png", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("код %d", rec.Code)
	}
}

func TestSplitMediaPath(t *testing.T) {
	branch, rel, ok := splitMediaPath("/b/docs/media/shots/a.png")
	if !ok || branch != "docs" || rel != "media/shots/a.png" {
		t.Fatalf("%s %s %v", branch, rel, ok)
	}

	if _, _, ok := splitMediaPath("/b/docs/page"); ok {
		t.Fatal("не медиа")
	}
}

func TestMediaPublicOK(t *testing.T) {
	wiki := &mediaWiki{data: []byte("png"), ctype: "image/png"}
	h := New(wiki, &stubAuth{}, &stubView{}, false, "README")
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/b/docs/media/a.png", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "png" {
		t.Fatalf("код %d тело %q", rec.Code, rec.Body.String())
	}

	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("тип %s", ct)
	}
}

type mediaWiki struct {
	stubWiki
	data  []byte
	ctype string
}

func (m *mediaWiki) ReadMedia(context.Context, string, string, string) ([]byte, string, error) {
	if len(m.data) == 0 {
		return nil, "", domain.ErrNotFound
	}

	return m.data, m.ctype, nil
}
