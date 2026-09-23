package httpapi

import (
	"errors"
	"net/http"

	"github.com/magomedcoder/kwiki/internal/domain"
	"github.com/magomedcoder/kwiki/internal/usecase"
)

func (h *Handler) users(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r)
	if !actor.Admin {
		http.Error(w, "недостаточно прав", http.StatusForbidden)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.renderUsers(w, r, "", noticeText(r.URL.Query().Get("notice")), http.StatusOK)
	case http.MethodPost:
		h.submitUsers(w, r, actor)
	default:
		http.Error(w, "метод не разрешен", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) submitUsers(w http.ResponseWriter, r *http.Request, actor usecase.Actor) {
	var err error
	var notice string
	switch r.FormValue("action") {
	case "create":
		err = h.auth.CreateUser(r.Context(), usecase.Account{
			FirstName: r.FormValue("first_name"),
			LastName:  r.FormValue("last_name"),
			Email:     r.FormValue("email"),
			Password:  r.FormValue("password"),
			Admin:     r.FormValue("admin") == "1",
		})
		notice = "created"
	case "delete":
		err = h.auth.DeleteUser(r.Context(), actor.ID, r.FormValue("email"))
		notice = "deleted"
	case "block":
		err = h.auth.SetBlocked(r.Context(), actor.ID, r.FormValue("email"), true)
		notice = "blocked"
	case "unblock":
		err = h.auth.SetBlocked(r.Context(), actor.ID, r.FormValue("email"), false)
		notice = "unblocked"
	default:
		http.Error(w, "некорректный запрос", http.StatusBadRequest)
		return
	}

	if err != nil {
		message, status := userFailure(err)
		if status == http.StatusInternalServerError {
			http.Error(w, "ошибка пользователей", status)
			return
		}
		h.renderUsers(w, r, message, "", status)
		return
	}

	http.Redirect(w, r, "/users?notice="+notice, http.StatusSeeOther)
}

func (h *Handler) renderUsers(w http.ResponseWriter, r *http.Request, message, notice string, status int) {
	users, err := h.auth.ListUsers(r.Context(), actorFrom(r).ID)
	if err != nil {
		http.Error(w, "ошибка пользователей", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(status)
	h.view.Users(w, usecase.UsersPage{
		Users:  users,
		Error:  message,
		Notice: notice,
	}, actorFrom(r))
}

func userFailure(err error) (string, int) {
	switch {
	case errors.Is(err, domain.ErrInvalidName):
		return "Укажите имя и фамилию.", http.StatusBadRequest
	case errors.Is(err, domain.ErrInvalidEmail):
		return "Некорректная почта.", http.StatusBadRequest
	case errors.Is(err, domain.ErrWeakPassword):
		return err.Error(), http.StatusBadRequest
	case errors.Is(err, domain.ErrEmailTaken):
		return "Пользователь с такой почтой уже есть.", http.StatusConflict
	case errors.Is(err, domain.ErrNotFound):
		return "Пользователь не найден.", http.StatusNotFound
	case errors.Is(err, domain.ErrLastAdmin):
		return "Нельзя удалить или заблокировать последнего администратора.", http.StatusConflict
	case errors.Is(err, domain.ErrSelfAction):
		return "Нельзя изменить свою учётную запись.", http.StatusConflict
	default:
		return "", http.StatusInternalServerError
	}
}

func noticeText(code string) string {
	switch code {
	case "created":
		return "Пользователь создан."
	case "deleted":
		return "Пользователь удалён."
	case "blocked":
		return "Пользователь заблокирован."
	case "unblocked":
		return "Блокировка снята."
	default:
		return ""
	}
}
