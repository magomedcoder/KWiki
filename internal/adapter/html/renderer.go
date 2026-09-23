package html

import (
	"bytes"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"time"

	"github.com/magomedcoder/kwiki/internal/adapter/markdown"
	"github.com/magomedcoder/kwiki/internal/domain"
	"github.com/magomedcoder/kwiki/internal/usecase"
)

var reTags = regexp.MustCompile(`>\s+<`)

type Renderer struct {
	index    *template.Template
	page     *template.Template
	edit     *template.Template
	login    *template.Template
	users    *template.Template
	branches *template.Template
}

type shell struct {
	Title       string
	Email       string
	Name        string
	Admin       bool
	CSRF        string
	Home        string
	NewHref     string
	Indexable   bool
	Canonical   string
	Description string
	Modified    string
}

type indexData struct {
	shell
	Catalog  bool
	Pages    []usecase.PageItem
	Branches []usecase.BranchView
}

type pageData struct {
	shell
	Slug      string
	Body      template.HTML
	Revisions []domain.Revision
	Missing   bool
	EditHref  string
	Branches  []usecase.BranchView
}

type editData struct {
	shell
	Branch   string
	Slug     string
	Content  string
	IsNew    bool
	Branches []domain.Branch
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

	users, err := template.ParseFiles(filepath.Join(dir, "layout.tmpl"), filepath.Join(dir, "users.tmpl"))
	if err != nil {
		return nil, err
	}

	branches, err := template.ParseFiles(filepath.Join(dir, "layout.tmpl"), filepath.Join(dir, "branches.tmpl"))
	if err != nil {
		return nil, err
	}

	return &Renderer{
		index:    index,
		page:     page,
		edit:     edit,
		login:    login,
		users:    users,
		branches: branches,
	}, nil
}

func actorShell(title string, actor usecase.Actor) shell {
	return shell{
		Title:   title,
		Email:   actor.Email,
		Name:    actor.Name,
		Admin:   actor.Admin,
		CSRF:    actor.CSRF,
		Home:    "/",
		NewHref: "/edit",
	}
}

func (r *Renderer) Index(w http.ResponseWriter, view usecase.IndexView, actor usecase.Actor) {
	frame := actorShell(view.Title, actor)
	frame.Indexable = view.Indexable
	frame.Canonical = view.Canonical
	frame.Description = view.Description
	if view.Branch != "" {
		frame.Home = domain.PagePath(view.Branch, "")
		frame.NewHref = domain.EditPath(view.Branch, "")
	}
	exec(w, r.index, indexData{
		shell:    frame,
		Catalog:  view.Catalog,
		Pages:    view.Pages,
		Branches: view.Branches,
	})
}

func (r *Renderer) Page(w http.ResponseWriter, view usecase.PageScreen, actor usecase.Actor) {
	var body template.HTML
	if !view.Missing {
		body = template.HTML(markdown.HTML([]byte(view.Markdown)))
	}

	frame := actorShell(view.Title, actor)
	frame.Indexable = view.Indexable
	frame.Canonical = view.Canonical
	frame.Description = view.Description
	frame.Home = domain.PagePath(view.Branch, "")
	frame.NewHref = domain.EditPath(view.Branch, "")
	if !view.UpdatedAt.IsZero() {
		frame.Modified = view.UpdatedAt.UTC().Format(time.RFC3339)
	}

	exec(w, r.page, pageData{
		shell:     frame,
		Slug:      view.Slug,
		Body:      body,
		Revisions: view.Revisions,
		Missing:   view.Missing,
		EditHref:  view.EditHref,
		Branches:  view.Branches,
	})
}

func (r *Renderer) Edit(w http.ResponseWriter, form usecase.EditForm, branches []domain.Branch, actor usecase.Actor) {
	title := "Новая страница"
	if !form.IsNew {
		title = "Редактирование: " + form.Slug
	}

	frame := actorShell(title, actor)
	frame.Home = domain.PagePath(form.Branch, "")
	frame.NewHref = domain.EditPath(form.Branch, "")
	exec(w, r.edit, editData{
		shell:    frame,
		Branch:   form.Branch,
		Slug:     form.Slug,
		Content:  form.Content,
		IsNew:    form.IsNew,
		Branches: branches,
	})
}

func (r *Renderer) Users(w http.ResponseWriter, page usecase.UsersPage, actor usecase.Actor) {
	exec(w, r.users, struct {
		shell
		usecase.UsersPage
	}{
		shell:     actorShell("Пользователи", actor),
		UsersPage: page,
	})
}

func (r *Renderer) Branches(w http.ResponseWriter, page usecase.BranchesPage, actor usecase.Actor) {
	exec(w, r.branches, struct {
		shell
		usecase.BranchesPage
	}{
		shell:        actorShell("Ветки", actor),
		BranchesPage: page,
	})
}

func (r *Renderer) Login(w http.ResponseWriter, page usecase.LoginPage) {
	exec(w, r.login, loginData{
		shell: shell{Title: "Вход", CSRF: page.CSRF, Home: "/"},
		Error: page.Error,
		Next:  page.Next,
		Value: page.Email,
	})
}

func exec(w http.ResponseWriter, t *template.Template, data any) {
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout.tmpl", data); err != nil {
		http.Error(w, "ошибка шаблона", http.StatusInternalServerError)
		log.Printf("ошибка шаблона: %v", err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(minifyHTML(buf.String())))
}

func minifyHTML(s string) string {
	return reTags.ReplaceAllString(s, "><")
}
