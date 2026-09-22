package handlers

import (
	"github.com/magomedcoder/kwiki/internal/gitstore"
	"github.com/magomedcoder/kwiki/internal/markdown"
	"github.com/magomedcoder/kwiki/internal/models"
	"github.com/magomedcoder/kwiki/internal/render"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"gorm.io/gorm"
)

type Handler struct {
	store *gitstore.Store
	db    *gorm.DB
}

func New(store *gitstore.Store, db *gorm.DB) *Handler {
	return &Handler{
		store: store,
		db:    db,
	}
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		h.Page(w, r)
		return
	}

	var pages []models.PageIndex
	h.db.Order("slug ASC").Find(&pages)
	render.RenderIndex(w, models.IndexData{
		Title: "KWiki",
		Pages: pages,
	})
}

func (h *Handler) Page(w http.ResponseWriter, r *http.Request) {
	slug := strings.Trim(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if slug == "" {
		h.Index(w, r)
		return
	}

	if strings.Contains(slug, "..") {
		http.NotFound(w, r)
		return
	}

	path := slug
	if !strings.HasSuffix(path, ".md") {
		path += ".md"
	}

	content, err := h.store.ReadFile(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	body := markdown.Convert(content)

	var revs []models.Revision
	h.db.Where("slug = ?", slug).
		Order("created_at DESC").
		Limit(20).
		Find(&revs)

	title := strings.ReplaceAll(filepath.Base(slug), "-", " ")
	render.RenderPage(w, models.PageData{
		Title:     title,
		Slug:      slug,
		Body:      body,
		Revisions: revs,
	})
}

func (h *Handler) Edit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	slug := strings.Trim(r.FormValue("slug"), "/")
	if slug == "" || strings.Contains(slug, "..") {
		http.Error(w, "некорректный слаг", http.StatusBadRequest)
		return
	}

	content, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	path := slug
	if !strings.HasSuffix(path, ".md") {
		path += ".md"
	}

	msg := "edit: " + slug
	if err := h.store.WriteFile(path, content, msg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/"+slug, http.StatusSeeOther)
}
