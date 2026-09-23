package httpapi

import (
	"errors"
	"log"
	"net/http"

	"github.com/magomedcoder/kwiki/internal/domain"
	"github.com/magomedcoder/kwiki/internal/usecase"
)

func (h *Handler) branches(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r)
	if !actor.Admin {
		http.Error(w, "недостаточно прав", http.StatusForbidden)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.renderBranches(w, r, "", branchNotice(r.URL.Query().Get("notice")), http.StatusOK)
	case http.MethodPost:
		h.submitBranch(w, r)
	default:
		http.Error(w, "метод не разрешен", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) submitBranch(w http.ResponseWriter, r *http.Request) {
	var err error
	var notice string
	switch r.FormValue("action") {
	case "create":
		_, err = h.wiki.CreateBranch(r.Context(), r.FormValue("name"), r.FormValue("public") == "1")
		notice = "created"
	case "public":
		err = h.wiki.SetBranchPublic(r.Context(), r.FormValue("name"), true)
		notice = "public"
	case "private":
		err = h.wiki.SetBranchPublic(r.Context(), r.FormValue("name"), false)
		notice = "private"
	case "delete":
		err = h.wiki.DeleteBranch(r.Context(), r.FormValue("name"))
		notice = "deleted"
	default:
		http.Error(w, "некорректный запрос", http.StatusBadRequest)
		return
	}

	if err != nil {
		message, status := branchFailure(err)
		if status == http.StatusInternalServerError {
			log.Printf("ветка: %v", err)
			http.Error(w, "ошибка веток", status)
			return
		}
		h.renderBranches(w, r, message, "", status)
		return
	}

	http.Redirect(w, r, "/branches?notice="+notice, http.StatusSeeOther)
}

func (h *Handler) renderBranches(w http.ResponseWriter, r *http.Request, message, notice string, status int) {
	branches, err := h.wiki.ListBranches(r.Context())
	if err != nil {
		log.Printf("список веток: %v", err)
		http.Error(w, "ошибка веток", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(status)
	h.view.Branches(w, usecase.BranchesPage{
		Branches: branches,
		Error:    message,
		Notice:   notice,
	}, actorFrom(r))
}

func branchFailure(err error) (string, int) {
	switch {
	case errors.Is(err, domain.ErrInvalidBranch):
		return "Некорректное имя ветки. Латинские буквы, цифры и дефис, до 32 символов.", http.StatusBadRequest
	case errors.Is(err, domain.ErrBranchTaken):
		return "Ветка с таким именем уже есть.", http.StatusConflict
	case errors.Is(err, domain.ErrDefaultBranch):
		return "Нельзя удалить основную ветку.", http.StatusConflict
	case errors.Is(err, domain.ErrNotFound):
		return "Ветка не найдена.", http.StatusNotFound
	default:
		return "", http.StatusInternalServerError
	}
}

func branchNotice(code string) string {
	switch code {
	case "created":
		return "Ветка создана."
	case "public":
		return "Ветка открыта для всех и для поисковых систем."
	case "private":
		return "Ветка снова видна только после входа."
	case "deleted":
		return "Ветка удалена."
	default:
		return ""
	}
}
