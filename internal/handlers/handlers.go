package handlers

import (
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/magomedcoder/kwiki/internal/db"
	"github.com/magomedcoder/kwiki/internal/gitstore"
	"github.com/magomedcoder/kwiki/internal/markdown"
	"github.com/magomedcoder/kwiki/internal/models"
	"github.com/magomedcoder/kwiki/internal/render"

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
		render.RenderPage(w, models.PageData{
			Title:   strings.ReplaceAll(filepath.Base(slug), "-", " "),
			Slug:    slug,
			Body:    "",
			Missing: true,
		})
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
	slug := strings.Trim(r.URL.Query().Get("slug"), "/")
	if strings.Contains(slug, "..") {
		http.Error(w, "некорректный слаг", http.StatusBadRequest)
		return
	}

	var content string
	isNew := true

	if slug != "" {
		path := slug
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}

		if data, err := h.store.ReadFile(path); err == nil {
			content = string(data)
			isNew = false
		}
	}

	title := "Новая страница"
	if !isNew {
		title = "Редактирование: " + slug
	}

	render.RenderEdit(w, models.EditData{
		Title:   title,
		Slug:    slug,
		Content: content,
		IsNew:   isNew,
	})
}

func (h *Handler) savePage(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slug := strings.Trim(r.FormValue("slug"), "/")
	if slug == "" || strings.Contains(slug, "..") {
		http.Error(w, "некорректный слаг", http.StatusBadRequest)
		return
	}

	content := []byte(r.FormValue("content"))

	path := slug
	if !strings.HasSuffix(path, ".md") {
		path += ".md"
	}

	_, readErr := h.store.ReadFile(path)
	action := "edit"
	if readErr != nil {
		action = "create"
	}
	msg := action + ": " + slug

	if err := h.store.WriteFile(path, content, msg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := db.SyncIndex(h.db, h.store); err != nil {
		log.Printf("sync index: %v", err)
	}

	http.Redirect(w, r, "/"+slug, http.StatusSeeOther)
}
