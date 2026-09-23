package httpapi

import (
	"errors"
	"net/http"
	"strings"

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
		h.renderUsers(w, r, "", noticeText(r.URL.Query().Get("notice")), http.StatusOK, "", usecase.UserDraft{})
	case http.MethodPost:
		h.submitUsers(w, r, actor)
	default:
		http.Error(w, "метод не разрешен", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) submitUsers(w http.ResponseWriter, r *http.Request, actor usecase.Actor) {
	action := r.FormValue("action")
	draft := usecase.UserDraft{
		Email:         r.FormValue("email"),
		OriginalEmail: r.FormValue("original_email"),
		FirstName:     r.FormValue("first_name"),
		LastName:      r.FormValue("last_name"),
		Admin:         r.FormValue("admin") == "1",
		Blocked:       r.FormValue("blocked") == "1",
	}
	var err error
	var notice string
	switch action {
	case "create":
		err = h.auth.CreateUser(r.Context(), usecase.Account{
			FirstName: draft.FirstName,
			LastName:  draft.LastName,
			Email:     draft.Email,
			Password:  r.FormValue("password"),
			Admin:     draft.Admin,
		})
		notice = "created"
	case "update":
		err = h.auth.UpdateUser(r.Context(), actor.ID, draft.OriginalEmail, usecase.Account{
			FirstName: draft.FirstName,
			LastName:  draft.LastName,
			Email:     draft.Email,
			Password:  r.FormValue("password"),
			Admin:     draft.Admin,
		}, draft.Blocked)
		notice = "updated"
	case "delete":
		draft.OriginalEmail = draft.Email
		err = h.auth.DeleteUser(r.Context(), actor.ID, draft.Email)
		notice = "deleted"
	case "block":
		err = h.auth.SetBlocked(r.Context(), actor.ID, draft.Email, true)
		notice = "blocked"
	case "unblock":
		err = h.auth.SetBlocked(r.Context(), actor.ID, draft.Email, false)
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
		h.renderUsers(w, r, message, "", status, action, draft)
		return
	}

	http.Redirect(w, r, "/users?notice="+notice, http.StatusSeeOther)
}

func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	if r.FormValue("password") != r.FormValue("password_confirm") {
		http.Error(w, "Пароли не совпадают.", http.StatusBadRequest)
		return
	}

	actor := actorFrom(r)
	err := h.auth.ChangeOwnPassword(r.Context(), actor.ID, r.FormValue("current_password"), r.FormValue("password"))
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			http.Error(w, "Неверный текущий пароль.", http.StatusUnauthorized)
			return
		}

		message, status := userFailure(err)
		if status == http.StatusInternalServerError {
			http.Error(w, "ошибка смены пароля", status)
			return
		}

		http.Error(w, message, status)
		return
	}

	h.clearCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) renderUsers(w http.ResponseWriter, r *http.Request, message, notice string, status int, action string, draft usecase.UserDraft) {
	actor := actorFrom(r)
	users, err := h.auth.ListUsers(r.Context(), actor.ID)
	if err != nil {
		http.Error(w, "ошибка пользователей", http.StatusInternalServerError)
		return
	}

	if draft.OriginalEmail != "" && strings.EqualFold(draft.OriginalEmail, actor.Email) {
		draft.Self = true
	}

	if action == "delete" {
		for _, user := range users {
			if strings.EqualFold(user.Email, draft.Email) {
				draft.FirstName = user.FirstName
				draft.LastName = user.LastName
				break
			}
		}
	}

	w.WriteHeader(status)
	h.view.Users(w, usecase.UsersPage{
		Users:  users,
		Error:  message,
		Notice: notice,
		Form:   action,
		Draft:  draft,
	}, actor)
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
		return "Нельзя снять права, удалить или заблокировать последнего администратора.", http.StatusConflict
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
	case "updated":
		return "Пользователь сохранён."
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
