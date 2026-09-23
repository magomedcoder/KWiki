package httpapi

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/magomedcoder/kwiki/internal/domain"
	"github.com/magomedcoder/kwiki/internal/usecase"
)

type Wiki interface {
	ListPages(ctx context.Context) ([]domain.Page, error)

	ViewPage(ctx context.Context, slug string) (usecase.PageView, error)

	EditForm(ctx context.Context, slug string) (usecase.EditForm, error)

	SavePage(ctx context.Context, slug, content string) (string, error)
}

type View interface {
	Index(w http.ResponseWriter, pages []domain.Page)
	Page(w http.ResponseWriter, view usecase.PageView)
	Edit(w http.ResponseWriter, form usecase.EditForm)
}

type Handler struct {
	wiki Wiki
	view View
}

func New(wiki Wiki, view View) *Handler {
	return &Handler{
		wiki: wiki,
		view: view,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/", h.index)
	mux.HandleFunc("/edit", h.edit)
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		h.viewPage(w, r)
		return
	}

	pages, err := h.wiki.ListPages(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.view.Index(w, pages)
}

func (h *Handler) viewPage(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/")
	view, err := h.wiki.ViewPage(r.Context(), slug)
	if errors.Is(err, domain.ErrInvalidSlug) {
		http.NotFound(w, r)
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.view.Page(w, view)
}

func (h *Handler) edit(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.editForm(w, r)
	case http.MethodPost:
		h.savePage(w, r)
	default:
		http.Error(w, "метод не разрешен", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) editForm(w http.ResponseWriter, r *http.Request) {
	form, err := h.wiki.EditForm(r.Context(), r.URL.Query().Get("slug"))
	if errors.Is(err, domain.ErrInvalidSlug) {
		http.Error(w, "некорректный слаг", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.view.Edit(w, form)
}

func (h *Handler) savePage(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slug, err := h.wiki.SavePage(r.Context(), r.FormValue("slug"), r.FormValue("content"))
	if err != nil && !errors.Is(err, usecase.ErrIndexSync) {
		if errors.Is(err, domain.ErrInvalidSlug) {
			http.Error(w, "некорректный слаг", http.StatusBadRequest)
			return
		}

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err != nil {
		log.Printf("sync index: %v", err)
	}

	http.Redirect(w, r, "/"+slug, http.StatusSeeOther)
}
