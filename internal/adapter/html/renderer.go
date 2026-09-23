package html

import (
	"bytes"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"regexp"

	"github.com/magomedcoder/kwiki/internal/adapter/markdown"
	"github.com/magomedcoder/kwiki/internal/domain"
	"github.com/magomedcoder/kwiki/internal/usecase"
)

var reTags = regexp.MustCompile(`>\s+<`)

type Renderer struct {
	index *template.Template
	page  *template.Template
	edit  *template.Template
}

type indexData struct {
	Title string
	Pages []domain.Page
}

type pageData struct {
	Title     string
	Slug      string
	Body      template.HTML
	Revisions []domain.Revision
	Missing   bool
}

type editData struct {
	Title   string
	Slug    string
	Content string
	IsNew   bool
}

func Load(dir string) (*Renderer, error) {
	index, err := template.ParseFiles(filepath.Join(dir, "layout.tmpl"), filepath.Join(dir, "index.tmpl"))
	if err != nil {
		return nil, err
	}

	page, err := template.ParseFiles(filepath.Join(dir, "layout.tmpl"), filepath.Join(dir, "page.tmpl"))
	if err != nil {
		return nil, err
	}

	edit, err := template.ParseFiles(filepath.Join(dir, "layout.tmpl"), filepath.Join(dir, "edit.tmpl"))
	if err != nil {
		return nil, err
	}

	return &Renderer{
		index: index,
		page:  page,
		edit:  edit,
	}, nil
}

func (r *Renderer) Index(w http.ResponseWriter, pages []domain.Page) {
	exec(w, r.index, indexData{
		Title: "KWiki",
		Pages: pages,
	})
}

func (r *Renderer) Page(w http.ResponseWriter, view usecase.PageView) {
	var body template.HTML
	if !view.Missing {
		body = template.HTML(markdown.HTML([]byte(view.Markdown)))
	}

	exec(w, r.page, pageData{
		Title:     view.Title,
		Slug:      view.Slug,
		Body:      body,
		Revisions: view.Revisions,
		Missing:   view.Missing,
	})
}

func (r *Renderer) Edit(w http.ResponseWriter, form usecase.EditForm) {
	title := "Новая страница"
	if !form.IsNew {
		title = "Редактирование: " + form.Slug
	}

	exec(w, r.edit, editData{
		Title:   title,
		Slug:    form.Slug,
		Content: form.Content,
		IsNew:   form.IsNew,
	})
}

func exec(w http.ResponseWriter, t *template.Template, data any) {
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout.tmpl", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("template error: %v", err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(minifyHTML(buf.String())))
}

func minifyHTML(s string) string {
	return reTags.ReplaceAllString(s, "><")
}
