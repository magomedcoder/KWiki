package httpapi

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/magomedcoder/kwiki/internal/domain"
	"github.com/magomedcoder/kwiki/internal/usecase"
)

type Wiki interface {
	OpenBranch(ctx context.Context, name string) (domain.Branch, error)

	VisibleBranches(ctx context.Context, authenticated bool) ([]domain.Branch, error)

	ListBranches(ctx context.Context) ([]domain.Branch, error)

	ListPages(ctx context.Context, branch string) ([]domain.Page, error)

	ViewPage(ctx context.Context, branch, slug string) (usecase.PageView, error)

	EditForm(ctx context.Context, branch, slug string) (usecase.EditForm, error)

	SavePage(ctx context.Context, branch, slug, content string) (string, error)

	CreateBranch(ctx context.Context, name string, public bool) (domain.Branch, error)

	SetBranchPublic(ctx context.Context, name string, public bool) error

	DeleteBranch(ctx context.Context, name string) error

	Sitemap(ctx context.Context) ([]usecase.SitemapEntry, error)
}

type Auth interface {
	BeginLogin(ctx context.Context, previousToken, client string) (usecase.IssuedSession, error)

	Login(ctx context.Context, challengeToken, csrf, email, password, ip, client string) (usecase.IssuedSession, error)

	Resume(ctx context.Context, token, client string) (usecase.Actor, error)

	Logout(ctx context.Context, token string) error

	CreateUser(ctx context.Context, account usecase.Account) error

	ListUsers(ctx context.Context, actorID string) ([]usecase.ManagedUser, error)

	DeleteUser(ctx context.Context, actorID, email string) error

	SetBlocked(ctx context.Context, actorID, email string, blocked bool) error
}

type View interface {
	Index(w http.ResponseWriter, view usecase.IndexView, actor usecase.Actor)

	Page(w http.ResponseWriter, view usecase.PageScreen, actor usecase.Actor)

	Edit(w http.ResponseWriter, form usecase.EditForm, branches []domain.Branch, actor usecase.Actor)

	Login(w http.ResponseWriter, page usecase.LoginPage)

	Users(w http.ResponseWriter, page usecase.UsersPage, actor usecase.Actor)

	Branches(w http.ResponseWriter, page usecase.BranchesPage, actor usecase.Actor)
}

type Handler struct {
	wiki       Wiki
	auth       Auth
	view       View
	secure     bool
	cookieName string
}

func New(wiki Wiki, auth Auth, view View, secure bool) *Handler {
	return &Handler{
		wiki:       wiki,
		auth:       auth,
		view:       view,
		secure:     secure,
		cookieName: cookieName(secure),
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/login", h.login)
	mux.HandleFunc("/robots.txt", h.robots)
	mux.HandleFunc("/sitemap.xml", h.sitemap)
	mux.Handle("/users", h.requireAuth(http.HandlerFunc(h.users)))
	mux.Handle("/branches", h.requireAuth(http.HandlerFunc(h.branches)))
	mux.Handle("/logout", h.requireAuth(http.HandlerFunc(h.logout)))
	mux.Handle("/edit", h.requireAuth(http.HandlerFunc(h.edit)))
	mux.Handle("/", h.optionalAuth(http.HandlerFunc(h.wikiPage)))
}

func (h *Handler) wikiPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	branchName, slug, ok := splitWikiPath(r.URL.Path)
	if !ok {
		markIndexable(w, false)
		http.Error(w, "страница не найдена", http.StatusNotFound)
		return
	}

	if branchName == domain.DefaultBranch && strings.HasPrefix(r.URL.Path, "/b/") {
		http.Redirect(w, r, domain.PagePath(domain.DefaultBranch, slug), http.StatusMovedPermanently)
		return
	}

	branch, err := h.wiki.OpenBranch(r.Context(), branchName)
	if errors.Is(err, domain.ErrInvalidBranch) || errors.Is(err, domain.ErrNotFound) {
		markIndexable(w, false)
		http.Error(w, "ветка не найдена", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("ветка: %v", err)
		http.Error(w, "не удалось открыть ветку", http.StatusInternalServerError)
		return
	}

	actor := actorFrom(r)
	if !branch.Public && actor.Email == "" {
		if r.URL.Path == "/" {
			h.publicCatalog(w, r)
			return
		}
		h.denyPrivate(w, r)
		return
	}

	if slug == "" {
		h.listBranch(w, r, branch)
		return
	}
	h.showPage(w, r, branch, slug)
}

func (h *Handler) publicCatalog(w http.ResponseWriter, r *http.Request) {
	branches, err := h.wiki.VisibleBranches(r.Context(), false)
	if err != nil {
		log.Printf("список веток: %v", err)
		http.Error(w, "не удалось загрузить ветки", http.StatusInternalServerError)
		return
	}
	if len(branches) == 0 {
		h.denyPrivate(w, r)
		return
	}

	markIndexable(w, true)
	h.view.Index(w, usecase.IndexView{
		Title:       "Публичные ветки",
		Description: "Публичные ветки вики",
		Catalog:     true,
		Branches:    usecase.BranchLinks(branches, ""),
		Canonical:   h.absolute(r, "/"),
		Indexable:   true,
	}, usecase.Actor{})
}

func (h *Handler) listBranch(w http.ResponseWriter, r *http.Request, branch domain.Branch) {
	pages, err := h.wiki.ListPages(r.Context(), branch.Name)
	if err != nil {
		log.Printf("список страниц: %v", err)
		http.Error(w, "не удалось загрузить страницы", http.StatusInternalServerError)
		return
	}

	actor := actorFrom(r)
	branches, err := h.wiki.VisibleBranches(r.Context(), actor.Email != "")
	if err != nil {
		log.Printf("список веток: %v", err)
		http.Error(w, "не удалось загрузить ветки", http.StatusInternalServerError)
		return
	}

	items := make([]usecase.PageItem, 0, len(pages))
	for _, page := range pages {
		items = append(items, usecase.PageItem{
			Title: page.Title,
			Href:  domain.PagePath(branch.Name, page.Slug),
			Hash:  page.Hash,
		})
	}

	title := branch.Name
	if branch.Name == domain.DefaultBranch {
		title = "Страницы"
	}
	indexable := branch.Public
	markIndexable(w, indexable)
	view := usecase.IndexView{
		Title:     title,
		Branch:    branch.Name,
		Pages:     items,
		Branches:  usecase.BranchLinks(branches, branch.Name),
		Indexable: indexable,
	}
	if indexable {
		view.Description = usecase.BranchDescription(branch)
		view.Canonical = h.absolute(r, domain.PagePath(branch.Name, ""))
	}
	h.view.Index(w, view, actor)
}

func (h *Handler) showPage(w http.ResponseWriter, r *http.Request, branch domain.Branch, slug string) {
	view, err := h.wiki.ViewPage(r.Context(), branch.Name, slug)
	if errors.Is(err, domain.ErrInvalidSlug) || errors.Is(err, domain.ErrNotFound) {
		markIndexable(w, false)
		http.Error(w, "страница не найдена", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("просмотр страницы: %v", err)
		http.Error(w, "не удалось открыть страницу", http.StatusInternalServerError)
		return
	}

	actor := actorFrom(r)
	if view.Missing && actor.Email == "" {
		markIndexable(w, false)
		http.Error(w, "страница не найдена", http.StatusNotFound)
		return
	}

	branches, err := h.wiki.VisibleBranches(r.Context(), actor.Email != "")
	if err != nil {
		log.Printf("список веток: %v", err)
		http.Error(w, "не удалось открыть страницу", http.StatusInternalServerError)
		return
	}

	indexable := branch.Public && !view.Missing
	markIndexable(w, indexable)
	screen := usecase.PageScreen{
		PageView:  view,
		EditHref:  domain.EditPath(branch.Name, view.Slug),
		Indexable: indexable,
		Branches:  usecase.BranchLinks(branches, branch.Name),
	}
	if indexable {
		screen.Canonical = h.absolute(r, domain.PagePath(branch.Name, view.Slug))
	}
	h.view.Page(w, screen, actor)
}

func (h *Handler) denyPrivate(w http.ResponseWriter, r *http.Request) {
	markIndexable(w, false)
	http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
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
	form, err := h.wiki.EditForm(r.Context(), requestedBranch(r), r.URL.Query().Get("slug"))
	if errors.Is(err, domain.ErrInvalidSlug) {
		http.Error(w, "некорректный адрес страницы", http.StatusBadRequest)
		return
	}
	if errors.Is(err, domain.ErrInvalidBranch) || errors.Is(err, domain.ErrNotFound) {
		http.Error(w, "ветка не найдена", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("форма правки: %v", err)
		http.Error(w, "не удалось открыть форму", http.StatusInternalServerError)
		return
	}

	branches, err := h.wiki.ListBranches(r.Context())
	if err != nil {
		log.Printf("список веток: %v", err)
		http.Error(w, "не удалось открыть форму", http.StatusInternalServerError)
		return
	}

	h.view.Edit(w, form, branches, actorFrom(r))
}

func (h *Handler) savePage(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "некорректный запрос", http.StatusBadRequest)
		return
	}

	branch := requestedBranch(r)
	slug, err := h.wiki.SavePage(r.Context(), branch, r.FormValue("slug"), r.FormValue("content"))
	if err != nil && !errors.Is(err, usecase.ErrIndexSync) {
		if errors.Is(err, domain.ErrInvalidSlug) {
			http.Error(w, "некорректный адрес страницы", http.StatusBadRequest)
			return
		}
		if errors.Is(err, domain.ErrInvalidBranch) || errors.Is(err, domain.ErrNotFound) {
			http.Error(w, "ветка не найдена", http.StatusNotFound)
			return
		}

		log.Printf("сохранение страницы: %v", err)
		http.Error(w, "не удалось сохранить страницу", http.StatusInternalServerError)
		return
	}

	if err != nil {
		log.Printf("синхронизация индекса: %v", err)
	}

	name, normErr := domain.NormalizeBranch(branch)
	if normErr != nil {
		name = domain.DefaultBranch
	}
	http.Redirect(w, r, domain.PagePath(name, slug), http.StatusSeeOther)
}

func requestedBranch(r *http.Request) string {
	name := strings.TrimSpace(r.URL.Query().Get("branch"))
	if r.Method == http.MethodPost {
		if form := strings.TrimSpace(r.FormValue("branch")); form != "" {
			name = form
		}
	}
	if name == "" {
		return domain.DefaultBranch
	}

	return name
}
