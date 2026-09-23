package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/magomedcoder/kwiki/internal/domain"
	"github.com/magomedcoder/kwiki/internal/usecase"
)

type actorKey struct{}

func (h *Handler) Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'unsafe-inline'; script-src 'none'; img-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'self'")
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		w.Header().Set("X-Permitted-Cross-Domain-Policies", "none")
		w.Header().Set("Cache-Control", "no-store")
		if h.secure {
			w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		actor, err := h.resume(r)
		if errors.Is(err, domain.ErrUnauthenticated) {
			h.clearCookie(w, r)
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
			return
		}
		if err != nil {
			http.Error(w, "ошибка входа", http.StatusInternalServerError)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "некорректный запрос", http.StatusBadRequest)
				return
			}
			if !usecase.TokenEqual(r.FormValue("csrf"), actor.CSRF) {
				http.Error(w, "некорректный запрос", http.StatusForbidden)
				return
			}
		}
		ctx := context.WithValue(r.Context(), actorKey{}, actor)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) optionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, err := h.resume(r)
		if err != nil && !errors.Is(err, domain.ErrUnauthenticated) {
			http.Error(w, "ошибка входа", http.StatusInternalServerError)
			return
		}

		if err == nil {
			r = r.WithContext(context.WithValue(r.Context(), actorKey{}, actor))
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) resume(r *http.Request) (usecase.Actor, error) {
	token := h.sessionToken(r)
	if token == "" {
		return usecase.Actor{}, domain.ErrUnauthenticated
	}
	return h.auth.Resume(r.Context(), token, clientHint(r))
}

func actorFrom(r *http.Request) usecase.Actor {
	actor, _ := r.Context().Value(actorKey{}).(usecase.Actor)
	return actor
}
