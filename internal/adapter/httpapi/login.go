package httpapi

import (
	"errors"
	"log"
	"net/http"

	"github.com/magomedcoder/kwiki/internal/domain"
	"github.com/magomedcoder/kwiki/internal/usecase"
)

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.showLogin(w, r)
	case http.MethodPost:
		h.submitLogin(w, r)
	default:
		http.Error(w, "метод не разрешен", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) showLogin(w http.ResponseWriter, r *http.Request) {
	if actor, err := h.resume(r); err == nil && actor.Email != "" {
		http.Redirect(w, r, safeNext(r.URL.Query().Get("next")), http.StatusSeeOther)
		return
	}

	issued, err := h.auth.BeginLogin(r.Context(), h.sessionToken(r), clientHint(r))
	if err != nil {
		http.Error(w, "ошибка входа", http.StatusInternalServerError)
		return
	}
	h.setCookie(w, r, issued.Token, issued.ExpiresAt)
	h.view.Login(w, usecase.LoginPage{
		CSRF: issued.CSRF,
		Next: safeNext(r.URL.Query().Get("next")),
	})
}

func (h *Handler) submitLogin(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "некорректный запрос", http.StatusBadRequest)
		return
	}

	issued, err := h.auth.Login(
		r.Context(),
		h.sessionToken(r),
		r.FormValue("csrf"),
		r.FormValue("email"),
		r.FormValue("password"),
		clientIP(r),
		clientHint(r),
	)
	if err != nil {
		h.renderLoginError(w, r, err)
		return
	}

	h.setCookie(w, r, issued.Token, issued.ExpiresAt)
	http.Redirect(w, r, safeNext(r.FormValue("next")), http.StatusSeeOther)
}

func (h *Handler) renderLoginError(w http.ResponseWriter, r *http.Request, reason error) {
	message, status := loginFailure(reason)
	if status == http.StatusInternalServerError {
		http.Error(w, "ошибка входа", status)
		return
	}
	log.Printf("неудачный вход с %s", clientIP(r))

	issued, err := h.auth.BeginLogin(r.Context(), "", clientHint(r))
	if err != nil {
		http.Error(w, "ошибка входа", http.StatusInternalServerError)
		return
	}
	h.setCookie(w, r, issued.Token, issued.ExpiresAt)
	w.WriteHeader(status)
	h.view.Login(w, usecase.LoginPage{
		CSRF:  issued.CSRF,
		Email: r.FormValue("email"),
		Next:  safeNext(r.FormValue("next")),
		Error: message,
	})
}

func loginFailure(err error) (string, int) {
	switch {
	case errors.Is(err, domain.ErrTooManyAttempts):
		return "Слишком много попыток. Повторите позже.", http.StatusTooManyRequests
	case errors.Is(err, domain.ErrInvalidCredentials):
		return "Неверная почта или пароль.", http.StatusUnauthorized
	case errors.Is(err, domain.ErrBlocked):
		return "Учётная запись заблокирована.", http.StatusForbidden
	case errors.Is(err, domain.ErrCSRF):
		return "Сессия формы истекла. Отправьте её ещё раз.", http.StatusBadRequest
	default:
		return "", http.StatusInternalServerError
	}
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "метод не разрешен", http.StatusMethodNotAllowed)
		return
	}
	if err := h.auth.Logout(r.Context(), h.sessionToken(r)); err != nil {
		http.Error(w, "ошибка выхода", http.StatusInternalServerError)
		return
	}
	h.clearCookie(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
