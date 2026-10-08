package httpapi

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/magomedcoder/kwiki/internal/domain"
	"github.com/magomedcoder/kwiki/internal/usecase"
)

type GitBackend interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)

	CloneURL(r *http.Request, secure bool) string
}

type gitLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
}

func newGitLimiter() *gitLimiter {
	return &gitLimiter{
		hits:   map[string][]time.Time{},
		limit:  60,
		window: time.Minute,
	}
}

func (l *gitLimiter) allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	cut := now.Add(-l.window)
	list := l.hits[key]
	n := 0

	for _, t := range list {
		if t.After(cut) {
			list[n] = t
			n++
		}
	}

	list = list[:n]
	if len(list) >= l.limit {
		l.hits[key] = list
		return false
	}

	l.hits[key] = append(list, now)
	return true
}

func (h *Handler) gitHTTP(w http.ResponseWriter, r *http.Request) {
	if h.git == nil {
		http.Error(w, "git unavailable", http.StatusServiceUnavailable)
		return
	}

	ip := clientIP(r)
	if !h.gitLimit.allow(ip) {
		http.Error(w, h.t(r, "errors.rate_limited"), http.StatusTooManyRequests)
		return
	}

	service := r.URL.Query().Get("service")
	isReceive := strings.Contains(r.URL.Path, "git-receive-pack") || service == "git-receive-pack"
	isUpload := strings.Contains(r.URL.Path, "git-upload-pack") || service == "git-upload-pack"

	actor, err := h.gitActor(r)
	if err != nil {
		w.Header().Set("WWW-Authenticate", `Basic realm="KWiki git"`)
		http.Error(w, "auth required", http.StatusUnauthorized)
		return
	}

	if actor.Email == "" {
		w.Header().Set("WWW-Authenticate", `Basic realm="KWiki git"`)
		http.Error(w, "auth required", http.StatusUnauthorized)
		return
	}

	if isReceive {
		r.Header.Set("X-Remote-User", actor.Email)
	} else if !isUpload && !strings.HasSuffix(r.URL.Path, "/info/refs") && r.URL.Path != "/git" && r.URL.Path != "/git/" {

	}

	h.git.ServeHTTP(w, r)

	if isReceive && r.Method == http.MethodPost {
		if err := h.afterGitPush(r); err != nil {
			log.Printf("после push: %v", err)
		}
	}
}

func (h *Handler) gitActor(r *http.Request) (usecase.Actor, error) {
	if actor := actorFrom(r); actor.Email != "" {
		return actor, nil
	}

	if token := h.sessionToken(r); token != "" {
		actor, err := h.auth.Resume(r.Context(), token, clientHint(r))
		if err == nil {
			return actor, nil
		}
	}

	user, pass, ok := r.BasicAuth()
	if !ok {
		return usecase.Actor{}, domain.ErrUnauthenticated
	}

	return h.auth.Authenticate(r.Context(), user, pass, clientIP(r))
}

func (h *Handler) afterGitPush(r *http.Request) error {
	if h.resetter != nil {
		if err := h.resetter.ResetWorktree(); err != nil {
			return err
		}
	}

	return h.wiki.Sync(r.Context())
}
