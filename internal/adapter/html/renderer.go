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
	login *template.Template
}

type shell struct {
	Title string
	Email string
	CSRF  string
}

type indexData struct {
	shell
	Pages []domain.Page
}

type pageData struct {
	shell
	Slug      string
	Body      template.HTML
	Revisions []domain.Revision
	Missing   bool
}

type editData struct {
	shell
	Slug    string
	Content string
	IsNew   bool
}

type loginData struct {
	shell
	Error string
	Next  string
	Value string
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

	login, err := template.ParseFiles(filepath.Join(dir, "layout.tmpl"), filepath.Join(dir, "login.tmpl"))
	if err != nil {
		return nil, err
	}

	return &Renderer{
		index: index,
		page:  page,
		edit:  edit,
		login: login,
	}, nil
}

func (r *Renderer) Index(w http.ResponseWriter, pages []domain.Page, actor usecase.Actor) {
	exec(w, r.index, indexData{
		shell: shell{Title: "KWiki", Email: actor.Email, CSRF: actor.CSRF},
		Pages: pages,
	})
}

func (r *Renderer) Page(w http.ResponseWriter, view usecase.PageView, actor usecase.Actor) {
	var body template.HTML
	if !view.Missing {
		body = template.HTML(markdown.HTML([]byte(view.Markdown)))
	}

	exec(w, r.page, pageData{
		shell:     shell{Title: view.Title, Email: actor.Email, CSRF: actor.CSRF},
		Slug:      view.Slug,
		Body:      body,
		Revisions: view.Revisions,
		Missing:   view.Missing,
	})
}

func (r *Renderer) Edit(w http.ResponseWriter, form usecase.EditForm, actor usecase.Actor) {
	title := "Новая страница"
	if !form.IsNew {
		title = "Редактирование: " + form.Slug
	}

	exec(w, r.edit, editData{
		shell:   shell{Title: title, Email: actor.Email, CSRF: actor.CSRF},
		Slug:    form.Slug,
		Content: form.Content,
		IsNew:   form.IsNew,
	})
}

func (r *Renderer) Login(w http.ResponseWriter, page usecase.LoginPage) {
	exec(w, r.login, loginData{
		shell: shell{Title: "Вход", CSRF: page.CSRF},
		Error: page.Error,
		Next:  page.Next,
		Value: page.Email,
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
