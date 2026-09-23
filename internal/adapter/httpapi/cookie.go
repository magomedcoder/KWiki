package httpapi

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func cookieName(secure bool) string {
	if secure {
		return "__Host-kwiki_session"
	}
	return "kwiki_session"
}

func (h *Handler) setCookie(w http.ResponseWriter, r *http.Request, token string, exp time.Time) {
	maxAge := max(int(time.Until(exp).Seconds()), 0)
	http.SetCookie(w, &http.Cookie{
		Name:     h.cookieName,
		Value:    token,
		Path:     "/",
		Expires:  exp,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.cookieSecure(r),
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *Handler) clearCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSecure(r),
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *Handler) cookieSecure(r *http.Request) bool {
	return h.secure || r.TLS != nil
}

func (h *Handler) sessionToken(r *http.Request) string {
	c, err := r.Cookie(h.cookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || host == "" {
		return r.RemoteAddr
	}
	return host
}

func clientHint(r *http.Request) string {
	ua := r.UserAgent()
	if len(ua) > 512 {
		ua = ua[:512]
	}
	return ua
}

func safeNext(raw string) string {
	if raw == "" || len(raw) > 2048 || strings.ContainsAny(raw, "\\\r\n\t ") {
		return "/"
	}
	if !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
		return "/"
	}
	u, err := url.Parse(raw)
	if err != nil || u.IsAbs() || u.Host != "" || u.Scheme != "" {
		return "/"
	}
	if !strings.HasPrefix(u.Path, "/") || strings.HasPrefix(u.Path, "//") {
		return "/"
	}
	return u.RequestURI()
}
