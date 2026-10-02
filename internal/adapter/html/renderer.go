package html

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/magomedcoder/kwiki/internal/adapter/i18n"
	"github.com/magomedcoder/kwiki/internal/adapter/markdown"
	"github.com/magomedcoder/kwiki/internal/domain"
	"github.com/magomedcoder/kwiki/internal/usecase"
	"github.com/magomedcoder/kwiki/resources"
)

var reTags = regexp.MustCompile(`>\s+<`)

type Renderer struct {
	bundle   *i18n.Bundle
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
	Lang        string
	Locale      string
	Languages   []i18n.Language
	ClientI18n  template.JS
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

func dict(values ...any) (map[string]any, error) {
	if len(values)%2 != 0 {
		return nil, errors.New("dict: odd argument count")
	}

	out := make(map[string]any, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, errors.New("dict: key is not a string")
		}
		out[key] = values[i+1]
	}

	return out, nil
}

func list(values ...any) []any {
	return values
}

func (r *Renderer) funcs() template.FuncMap {
	return template.FuncMap{
		"dict": dict,
		"list": list,
		"t": func(lang, key string, args ...any) string {
			return r.bundle.T(lang, key, args...)
		},
	}
}

func (r *Renderer) parsePage(fsys fs.FS, page string) (*template.Template, error) {
	components, err := fs.Glob(fsys, "templates/components/*.tmpl")
	if err != nil {
		return nil, err
	}

	files := append([]string{
		"templates/layout.tmpl",
		"templates/pages/" + page,
	}, components...)
	return template.New("layout.tmpl").Funcs(r.funcs()).Option("missingkey=zero").ParseFS(fsys, files...)
}

func Load(bundle *i18n.Bundle) (*Renderer, error) {
	if bundle == nil {
		var err error
		bundle, err = i18n.Load(resources.FS)
		if err != nil {
			return nil, err
		}
	}

	return loadFS(resources.FS, bundle)
}

func loadFS(fsys fs.FS, bundle *i18n.Bundle) (*Renderer, error) {
	r := &Renderer{bundle: bundle}
	var err error
	if r.index, err = r.parsePage(fsys, "index.tmpl"); err != nil {
		return nil, err
	}

	if r.page, err = r.parsePage(fsys, "page.tmpl"); err != nil {
		return nil, err
	}

	if r.edit, err = r.parsePage(fsys, "edit.tmpl"); err != nil {
		return nil, err
	}

	if r.login, err = r.parsePage(fsys, "login.tmpl"); err != nil {
		return nil, err
	}

	if r.users, err = r.parsePage(fsys, "users.tmpl"); err != nil {
		return nil, err
	}

	if r.branches, err = r.parsePage(fsys, "branches.tmpl"); err != nil {
		return nil, err
	}

	if r.notFound, err = r.parsePage(fsys, "notfound.tmpl"); err != nil {
		return nil, err
	}

	if r.history, err = r.parsePage(fsys, "history.tmpl"); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Renderer) actorShell(title string, actor usecase.Actor, lang string) shell {
	lang = r.bundle.Resolve(lang, "")
	raw, _ := json.Marshal(r.bundle.ClientMessages(lang))
	return shell{
		Title:      title,
		Email:      actor.Email,
		Name:       actor.Name,
		Admin:      actor.Admin,
		CSRF:       actor.CSRF,
		Home:       "/",
		NewHref:    "/edit",
		Lang:       lang,
		Locale:     r.bundle.Locale(lang),
		Languages:  r.bundle.Languages(),
		ClientI18n: template.JS(raw),
	}
}

func (r *Renderer) Index(w http.ResponseWriter, view usecase.IndexView, actor usecase.Actor, lang string) {
	frame := r.actorShell(view.Title, actor, lang)
	frame.Indexable = view.Indexable
	frame.Canonical = view.Canonical
	frame.Description = view.Description
	if view.Branch != "" {
		frame.Home = domain.PagePath(view.Branch, "")
		frame.NewHref = domain.EditPath(view.Branch, "")
	}
	r.exec(w, r.index, indexData{
		shell:    frame,
		Catalog:  view.Catalog,
		Branches: view.Branches,
	}, lang)
}

func (r *Renderer) Page(w http.ResponseWriter, view usecase.PageScreen, actor usecase.Actor, lang string) {
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

	frame := r.actorShell(view.Title, actor, lang)
	frame.Indexable = view.Indexable
	frame.Canonical = view.Canonical
	frame.Description = view.Description
	frame.Home = domain.PagePath(view.Branch, "")
	frame.NewHref = domain.EditPath(view.Branch, "")
	frame.Headings = headings
	if !view.UpdatedAt.IsZero() {
		frame.Modified = view.UpdatedAt.UTC().Format(time.RFC3339)
	}

	r.exec(w, r.page, pageData{
		shell:       frame,
		Slug:        view.Slug,
		Body:        body,
		Revisions:   view.Revisions,
		Missing:     view.Missing,
		EditHref:    view.EditHref,
		HistoryHref: view.HistoryHref,
		IsHome:      view.Home,
		Branches:    view.Branches,
	}, lang)
}

func (r *Renderer) History(w http.ResponseWriter, view usecase.PageScreen, actor usecase.Actor, lang string) {
	frame := r.actorShell(r.bundle.T(lang, "page.history_title", view.Title), actor, lang)
	frame.Home = domain.PagePath(view.Branch, "")
	frame.NewHref = domain.EditPath(view.Branch, "")
	r.exec(w, r.history, historyData{
		shell:     frame,
		Revisions: view.Revisions,
		EditHref:  view.EditHref,
		ReadHref:  view.ReadHref,
	}, lang)
}

func (r *Renderer) NotFound(w http.ResponseWriter, actor usecase.Actor, lang string) {
	frame := r.actorShell(r.bundle.T(lang, "page.not_found"), actor, lang)
	var buf bytes.Buffer
	if err := r.notFound.ExecuteTemplate(&buf, "layout.tmpl", frame); err != nil {
		http.Error(w, r.bundle.T(lang, "errors.template"), http.StatusInternalServerError)
		log.Printf("ошибка шаблона: %v", err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(minifyHTML(buf.String())))
}

func (r *Renderer) Edit(w http.ResponseWriter, form usecase.EditForm, branches []domain.Branch, actor usecase.Actor, lang string) {
	title := r.bundle.T(lang, "page.new_title")
	if !form.IsNew {
		title = r.bundle.T(lang, "page.edit_title", form.Slug)
	}

	frame := r.actorShell(title, actor, lang)
	frame.Home = domain.PagePath(form.Branch, "")
	frame.NewHref = domain.EditPath(form.Branch, "")
	cancel := domain.PagePath(form.Branch, "")
	if form.Slug != "" {
		cancel = domain.PagePath(form.Branch, form.Slug)
	}
	r.exec(w, r.edit, editData{
		shell:    frame,
		Branch:   form.Branch,
		Slug:     form.Slug,
		Content:  form.Content,
		IsNew:    form.IsNew,
		Branches: branches,
		Preview:  r.previewHTML(form.Content, form.Branch, lang),
		Cancel:   cancel,
	}, lang)
}

func (r *Renderer) Preview(w http.ResponseWriter, content, branch, lang string) {
	_, _ = w.Write([]byte(r.previewHTML(content, branch, lang)))
}

func (r *Renderer) previewHTML(content, branch, lang string) template.HTML {
	if strings.TrimSpace(content) == "" {
		return template.HTML(`<p class="m-0 text-wiki-faint">` + template.HTMLEscapeString(r.bundle.T(lang, "common.preview_empty")) + `</p>`)
	}

	return template.HTML(markdown.HTML([]byte(content), branch))
}

func (r *Renderer) Users(w http.ResponseWriter, page usecase.UsersPage, actor usecase.Actor, lang string) {
	r.exec(w, r.users, struct {
		shell
		usecase.UsersPage
	}{
		shell:     r.actorShell(r.bundle.T(lang, "users.title"), actor, lang),
		UsersPage: page,
	}, lang)
}

func (r *Renderer) Branches(w http.ResponseWriter, page usecase.BranchesPage, actor usecase.Actor, lang string) {
	r.exec(w, r.branches, struct {
		shell
		usecase.BranchesPage
	}{
		shell:        r.actorShell(r.bundle.T(lang, "branches.title"), actor, lang),
		BranchesPage: page,
	}, lang)
}

func (r *Renderer) Login(w http.ResponseWriter, page usecase.LoginPage, lang string) {
	frame := r.actorShell(r.bundle.T(lang, "auth.login"), usecase.Actor{CSRF: page.CSRF}, lang)
	r.exec(w, r.login, loginData{
		shell: frame,
		Error: page.Error,
		Next:  page.Next,
		Value: page.Email,
	}, lang)
}

func (r *Renderer) exec(w http.ResponseWriter, t *template.Template, data any, lang string) {
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout.tmpl", data); err != nil {
		http.Error(w, r.bundle.T(lang, "errors.template"), http.StatusInternalServerError)
		log.Printf("ошибка шаблона: %v", err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(minifyHTML(buf.String())))
}

func minifyHTML(s string) string {
	return reTags.ReplaceAllString(s, "><")
}
