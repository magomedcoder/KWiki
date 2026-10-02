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
		http.Error(w, h.t(r, "errors.forbidden"), http.StatusForbidden)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.renderBranches(w, r, "", h.branchNotice(r, r.URL.Query().Get("notice")), http.StatusOK, "", "", false)
	case http.MethodPost:
		h.submitBranch(w, r)
	default:
		http.Error(w, h.t(r, "errors.method_not_allowed"), http.StatusMethodNotAllowed)
	}
}

func (h *Handler) submitBranch(w http.ResponseWriter, r *http.Request) {
	action := r.FormValue("action")
	name := r.FormValue("name")
	public := r.FormValue("public") == "1"
	var err error
	var notice string
	switch action {
	case "create":
		_, err = h.wiki.CreateBranch(r.Context(), name, public)
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
		http.Error(w, h.t(r, "errors.bad_request"), http.StatusBadRequest)
		return
	}

	if err != nil {
		message, status := h.branchFailure(r, err)
		if status == http.StatusInternalServerError {
			log.Printf("ветка: %v", err)
			http.Error(w, h.t(r, "errors.branches"), status)
			return
		}
		h.renderBranches(w, r, message, "", status, action, name, public)
		return
	}

	http.Redirect(w, r, "/branches?notice="+notice, http.StatusSeeOther)
}

func (h *Handler) renderBranches(w http.ResponseWriter, r *http.Request, message, notice string, status int, action, name string, public bool) {
	branches, err := h.wiki.ListBranches(r.Context())
	if err != nil {
		log.Printf("список веток: %v", err)
		http.Error(w, h.t(r, "errors.branches"), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(status)
	h.view.Branches(w, usecase.BranchesPage{
		Branches: branches,
		Error:    message,
		Notice:   notice,
		Form:     action,
		Draft: usecase.BranchDraft{
			Name:   name,
			Public: public,
		},
	}, actorFrom(r), langFrom(r))
}

func (h *Handler) branchFailure(r *http.Request, err error) (string, int) {
	switch {
	case errors.Is(err, domain.ErrInvalidBranch):
		return h.t(r, "errors.invalid_branch"), http.StatusBadRequest
	case errors.Is(err, domain.ErrBranchTaken):
		return h.t(r, "errors.branch_taken"), http.StatusConflict
	case errors.Is(err, domain.ErrDefaultBranch):
		return h.t(r, "errors.default_branch"), http.StatusConflict
	case errors.Is(err, domain.ErrNotFound):
		return h.t(r, "errors.branch_missing"), http.StatusNotFound
	default:
		return "", http.StatusInternalServerError
	}
}

func (h *Handler) branchNotice(r *http.Request, code string) string {
	switch code {
	case "created":
		return h.t(r, "notice.branch_created")
	case "public":
		return h.t(r, "notice.branch_public")
	case "private":
		return h.t(r, "notice.branch_private")
	case "deleted":
		return h.t(r, "notice.branch_deleted")
	default:
		return ""
	}
}
