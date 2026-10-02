package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/magomedcoder/kwiki/internal/adapter/i18n"
)

type langKey struct{}

func (h *Handler) WithLang(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := h.resolveLang(r)
		ctx := context.WithValue(r.Context(), langKey{}, lang)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) resolveLang(r *http.Request) string {
	cookie := ""
	if c, err := r.Cookie(i18n.LangCookie); err == nil {
		cookie = c.Value
	}

	return h.bundle.Resolve(cookie, r.Header.Get("Accept-Language"))
}

func langFrom(r *http.Request) string {
	if lang, ok := r.Context().Value(langKey{}).(string); ok && lang != "" {
		return lang
	}

	return i18n.DefaultLang
}

func (h *Handler) t(r *http.Request, key string, args ...any) string {
	return h.bundle.T(langFrom(r), key, args...)
}

func (h *Handler) setLang(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, h.t(r, "errors.method_not_allowed"), http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, h.t(r, "errors.bad_request"), http.StatusBadRequest)
		return
	}

	lang := i18n.Normalize(r.FormValue("lang"))
	if !h.bundle.Supported(lang) {
		http.Error(w, h.t(r, "errors.bad_request"), http.StatusBadRequest)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     i18n.LangCookie,
		Value:    lang,
		Path:     "/",
		MaxAge:   int((365 * 24 * time.Hour).Seconds()),
		HttpOnly: false,
		Secure:   h.cookieSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, safeReferer(r), http.StatusSeeOther)
}

func safeReferer(r *http.Request) string {
	ref := strings.TrimSpace(r.Header.Get("Referer"))
	if ref == "" {
		return "/"
	}

	u, err := url.Parse(ref)
	if err != nil {
		return "/"
	}

	if u.Host != "" && r.Host != "" && !strings.EqualFold(u.Host, r.Host) {
		return "/"
	}

	path := u.RequestURI()
	if path == "" {
		path = u.Path
	}

	return safeNext(path)
}
