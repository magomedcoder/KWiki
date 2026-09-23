package html

import (
	"bytes"
	"errors"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/magomedcoder/kwiki/internal/adapter/markdown"
	"github.com/magomedcoder/kwiki/internal/domain"
	"github.com/magomedcoder/kwiki/internal/usecase"
	"github.com/magomedcoder/kwiki/resources"
)

var reTags = regexp.MustCompile(`>\s+<`)

type Renderer struct {
	index    *template.Template
	page     *template.Template
	edit     *template.Template
	login    *template.Template
	users    *template.Template
	branches *template.Template
	notFound *template.Template
	history  *template.Template
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
	Headings    []markdown.Heading
}

type indexData struct {
	shell
	Catalog  bool
	Branches []usecase.BranchView
}

type pageData struct {
	shell
	Slug        string
	Body        template.HTML
	Revisions   []domain.Revision
	Missing     bool
	EditHref    string
	HistoryHref string
	IsHome      bool
	Branches    []usecase.BranchView
}

type editData struct {
	shell
	Branch   string
	Slug     string
	Content  string
	IsNew    bool
	Branches []domain.Branch
	Preview  template.HTML
	Cancel   string
}

type historyData struct {
	shell
	Revisions []domain.Revision
	EditHref  string
	ReadHref  string
}

type loginData struct {
	shell
	Error string
	Next  string
	Value string
}

var templateFuncs = template.FuncMap{
	"dict": dict,
	"list": list,
}

func dict(values ...any) (map[string]any, error) {
	if len(values)%2 != 0 {
		return nil, errors.New("dict: нечётное число аргументов")
	}

	out := make(map[string]any, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, errors.New("dict: ключ не строка")
		}
		out[key] = values[i+1]
	}

	return out, nil
}

func list(values ...any) []any {
	return values
}

func parsePage(fsys fs.FS, page string) (*template.Template, error) {
	components, err := fs.Glob(fsys, "templates/components/*.tmpl")
	if err != nil {
		return nil, err
	}

	files := append([]string{
		"templates/layout.tmpl",
		"templates/pages/" + page,
	}, components...)
	return template.New("layout.tmpl").Funcs(templateFuncs).Option("missingkey=zero").ParseFS(fsys, files...)
}

func Load() (*Renderer, error) {
	return loadFS(resources.FS)
}

func loadFS(fsys fs.FS) (*Renderer, error) {
	index, err := parsePage(fsys, "index.tmpl")
	if err != nil {
		return nil, err
	}

	page, err := parsePage(fsys, "page.tmpl")
	if err != nil {
		return nil, err
	}

	edit, err := parsePage(fsys, "edit.tmpl")
	if err != nil {
		return nil, err
	}

	login, err := parsePage(fsys, "login.tmpl")
	if err != nil {
		return nil, err
	}

	users, err := parsePage(fsys, "users.tmpl")
	if err != nil {
		return nil, err
	}

	branches, err := parsePage(fsys, "branches.tmpl")
	if err != nil {
		return nil, err
	}

	notFound, err := parsePage(fsys, "notfound.tmpl")
	if err != nil {
		return nil, err
	}

	history, err := parsePage(fsys, "history.tmpl")
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
		notFound: notFound,
		history:  history,
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
		Branches: view.Branches,
	})
}

func (r *Renderer) Page(w http.ResponseWriter, view usecase.PageScreen, actor usecase.Actor) {
	var body template.HTML
	var headings []markdown.Heading
	if !view.Missing {
		rendered, found := markdown.Document([]byte(view.Markdown), view.Branch)
		body = template.HTML(rendered)
		headings = found
		if !view.Home && strings.TrimSpace(view.Title) != "" {
			headings = append([]markdown.Heading{{
				Level: 1,
				ID:    "page-title",
				Text:  view.Title,
			}}, headings...)
		}
	}

	frame := actorShell(view.Title, actor)
	frame.Indexable = view.Indexable
	frame.Canonical = view.Canonical
	frame.Description = view.Description
	frame.Home = domain.PagePath(view.Branch, "")
	frame.NewHref = domain.EditPath(view.Branch, "")
	frame.Headings = headings
	if !view.UpdatedAt.IsZero() {
		frame.Modified = view.UpdatedAt.UTC().Format(time.RFC3339)
	}

	exec(w, r.page, pageData{
		shell:       frame,
		Slug:        view.Slug,
		Body:        body,
		Revisions:   view.Revisions,
		Missing:     view.Missing,
		EditHref:    view.EditHref,
		HistoryHref: view.HistoryHref,
		IsHome:      view.Home,
		Branches:    view.Branches,
	})
}

func (r *Renderer) History(w http.ResponseWriter, view usecase.PageScreen, actor usecase.Actor) {
	frame := actorShell("История: "+view.Title, actor)
	frame.Home = domain.PagePath(view.Branch, "")
	frame.NewHref = domain.EditPath(view.Branch, "")
	exec(w, r.history, historyData{
		shell:     frame,
		Revisions: view.Revisions,
		EditHref:  view.EditHref,
		ReadHref:  view.ReadHref,
	})
}

func (r *Renderer) NotFound(w http.ResponseWriter, actor usecase.Actor) {
	frame := actorShell("Страница не найдена", actor)
	var buf bytes.Buffer
	if err := r.notFound.ExecuteTemplate(&buf, "layout.tmpl", frame); err != nil {
		http.Error(w, "ошибка шаблона", http.StatusInternalServerError)
		log.Printf("ошибка шаблона: %v", err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(minifyHTML(buf.String())))
}

func (r *Renderer) Edit(w http.ResponseWriter, form usecase.EditForm, branches []domain.Branch, actor usecase.Actor) {
	title := "Новая страница"
	if !form.IsNew {
		title = "Редактирование: " + form.Slug
	}

	frame := actorShell(title, actor)
	frame.Home = domain.PagePath(form.Branch, "")
	frame.NewHref = domain.EditPath(form.Branch, "")
	cancel := domain.PagePath(form.Branch, "")
	if form.Slug != "" {
		cancel = domain.PagePath(form.Branch, form.Slug)
	}
	exec(w, r.edit, editData{
		shell:    frame,
		Branch:   form.Branch,
		Slug:     form.Slug,
		Content:  form.Content,
		IsNew:    form.IsNew,
		Branches: branches,
		Preview:  previewHTML(form.Content, form.Branch),
		Cancel:   cancel,
	})
}

func (r *Renderer) Preview(w http.ResponseWriter, content, branch string) {
	_, _ = w.Write([]byte(previewHTML(content, branch)))
}

func previewHTML(content, branch string) template.HTML {
	if strings.TrimSpace(content) == "" {
		return `<p class="m-0 text-wiki-faint">Просмотр</p>`
	}

	return template.HTML(markdown.HTML([]byte(content), branch))
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
		Title: "Вход", CSRF: page.CSRF, Home: "/",
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
