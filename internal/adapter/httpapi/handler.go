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

	ViewPage(ctx context.Context, branch, slug string) (usecase.PageView, error)

	ListPages(ctx context.Context, branch string) ([]domain.Page, error)

	EditForm(ctx context.Context, branch, slug string) (usecase.EditForm, error)

	SavePage(ctx context.Context, branch, slug, content, draft string) (string, error)

	StageMedia(ctx context.Context, draft, branch, folder, name string, data []byte, overwrite bool) (usecase.MediaItem, error)

	ListMedia(ctx context.Context, draft, branch string) ([]usecase.MediaItem, error)

	DeleteMedia(ctx context.Context, draft, branch, path string) error

	ReadMedia(ctx context.Context, draft, branch, path string) ([]byte, string, error)

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

	UpdateUser(ctx context.Context, actorID, currentEmail string, account usecase.Account, blocked bool) error

	ChangeOwnPassword(ctx context.Context, userID, current, next string) error

	SetBlocked(ctx context.Context, actorID, email string, blocked bool) error
}

type View interface {
	Index(w http.ResponseWriter, view usecase.IndexView, actor usecase.Actor)

	Page(w http.ResponseWriter, view usecase.PageScreen, actor usecase.Actor)

	Edit(w http.ResponseWriter, form usecase.EditForm, branches []domain.Branch, actor usecase.Actor)

	Preview(w http.ResponseWriter, content, branch string)

	Login(w http.ResponseWriter, page usecase.LoginPage)

	Users(w http.ResponseWriter, page usecase.UsersPage, actor usecase.Actor)

	Branches(w http.ResponseWriter, page usecase.BranchesPage, actor usecase.Actor)

	History(w http.ResponseWriter, view usecase.PageScreen, actor usecase.Actor)

	NotFound(w http.ResponseWriter, actor usecase.Actor)
}

type Handler struct {
	wiki       Wiki
	auth       Auth
	view       View
	secure     bool
	home       string
	cookieName string
}

func New(wiki Wiki, auth Auth, view View, secure bool, home string) *Handler {
	if home == "" {
		home = domain.DefaultHome
	}

	return &Handler{
		wiki:       wiki,
		auth:       auth,
		view:       view,
		secure:     secure,
		home:       home,
		cookieName: cookieName(secure),
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/login", h.login)
	mux.HandleFunc("/favicon.ico", h.favicon)
	mux.HandleFunc("/robots.txt", h.robots)
	mux.HandleFunc("/sitemap.xml", h.sitemap)
	mux.Handle("/users", h.requireAuth(http.HandlerFunc(h.users)))
	mux.Handle("/account/password", h.requireAuth(http.HandlerFunc(h.changePassword)))
	mux.Handle("/branches", h.requireAuth(http.HandlerFunc(h.branches)))
	mux.Handle("/logout", h.requireAuth(http.HandlerFunc(h.logout)))
	mux.Handle("/edit/preview", h.requireAuth(http.HandlerFunc(h.previewEdit)))
	mux.Handle("/media", h.requireAuth(http.HandlerFunc(h.mediaAPI)))
	mux.HandleFunc("/css/", h.asset)
	mux.HandleFunc("/js/", h.asset)
	mux.Handle("/edit", h.requireAuth(http.HandlerFunc(h.edit)))
	mux.Handle("/history", h.optionalAuth(http.HandlerFunc(h.history)))
	mux.Handle("/", h.optionalAuth(http.HandlerFunc(h.wikiPage)))
}

func (h *Handler) wikiPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	if _, _, ok := splitMediaPath(r.URL.Path); ok {
		h.serveMedia(w, r)
		return
	}

	branchName, slug, ok := splitWikiPath(r.URL.Path)
	if !ok {
		h.notFound(w, r)
		return
	}

	if branchName == domain.DefaultBranch && strings.HasPrefix(r.URL.Path, "/b/") {
		if slug == h.home {
			slug = ""
		}
		http.Redirect(w, r, domain.PagePath(domain.DefaultBranch, slug), http.StatusMovedPermanently)
		return
	}

	branch, err := h.wiki.OpenBranch(r.Context(), branchName)
	if errors.Is(err, domain.ErrInvalidBranch) || errors.Is(err, domain.ErrNotFound) {
		h.notFound(w, r)
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

	home := slug == "" || slug == h.home
	if slug == h.home {
		target := domain.PagePath(branch.Name, "")
		if r.URL.Path != target {
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}
	}

	if slug == "" {
		slug = h.home
	}

	h.showPage(w, r, branch, slug, home)
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

	links := usecase.BranchLinks(branches, "")
	for i, branch := range branches {
		pages, err := h.pageLinks(r.Context(), branch.Name, "")
		if err != nil {
			log.Printf("список страниц: %v", err)
			http.Error(w, "не удалось загрузить страницы", http.StatusInternalServerError)
			return
		}
		links[i].Pages = pages
	}

	markIndexable(w, true)
	h.view.Index(w, usecase.IndexView{
		Title:       "Публичные страницы",
		Description: "Публичные страницы вики",
		Catalog:     true,
		Branches:    links,
		Canonical:   h.absolute(r, "/"),
		Indexable:   true,
	}, usecase.Actor{})
}

func (h *Handler) pageLinks(ctx context.Context, branch, current string) ([]usecase.PageLink, error) {
	pages, err := h.wiki.ListPages(ctx, branch)
	if err != nil {
		return nil, err
	}

	return usecase.PageLinks(branch, h.home, pages, current), nil
}

func (h *Handler) showPage(w http.ResponseWriter, r *http.Request, branch domain.Branch, slug string, home bool) {
	view, err := h.wiki.ViewPage(r.Context(), branch.Name, slug)
	if errors.Is(err, domain.ErrInvalidSlug) || errors.Is(err, domain.ErrNotFound) {
		h.notFound(w, r)
		return
	}
	if err != nil {
		log.Printf("просмотр страницы: %v", err)
		http.Error(w, "не удалось открыть страницу", http.StatusInternalServerError)
		return
	}

	actor := actorFrom(r)
	if view.Missing && actor.Email == "" {
		h.notFound(w, r)
		return
	}

	branches, err := h.wiki.VisibleBranches(r.Context(), actor.Email != "")
	if err != nil {
		log.Printf("список веток: %v", err)
		http.Error(w, "не удалось открыть страницу", http.StatusInternalServerError)
		return
	}

	if home && !view.Missing {
		if title := markdownTitle(view.Markdown); title != "" {
			view.Title = title
		}
	}

	indexable := branch.Public && !view.Missing
	markIndexable(w, indexable)
	path := domain.PagePath(branch.Name, view.Slug)
	if home {
		path = domain.PagePath(branch.Name, "")
	}

	pages, err := h.pageLinks(r.Context(), branch.Name, view.Slug)
	if err != nil {
		log.Printf("список страниц: %v", err)
		http.Error(w, "не удалось открыть страницу", http.StatusInternalServerError)
		return
	}

	screen := usecase.PageScreen{
		PageView:    view,
		EditHref:    domain.EditPath(branch.Name, view.Slug),
		ReadHref:    path,
		HistoryHref: domain.HistoryPath(branch.Name, view.Slug),
		Indexable:   indexable,
		Home:        home,
		Branches:    usecase.BranchLinks(branches, branch.Name),
		Pages:       pages,
	}
	if indexable {
		screen.Canonical = h.absolute(r, path)
	}

	h.view.Page(w, screen, actor)
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	slug := strings.TrimSpace(r.URL.Query().Get("slug"))
	if slug == "" {
		slug = h.home
	}

	branch, err := h.wiki.OpenBranch(r.Context(), requestedBranch(r))
	if errors.Is(err, domain.ErrInvalidBranch) || errors.Is(err, domain.ErrNotFound) {
		h.notFound(w, r)
		return
	}

	if err != nil {
		log.Printf("ветка: %v", err)
		http.Error(w, "не удалось открыть историю", http.StatusInternalServerError)
		return
	}

	actor := actorFrom(r)
	if !branch.Public && actor.Email == "" {
		h.denyPrivate(w, r)
		return
	}

	view, err := h.wiki.ViewPage(r.Context(), branch.Name, slug)
	if errors.Is(err, domain.ErrInvalidSlug) || errors.Is(err, domain.ErrNotFound) || view.Missing {
		h.notFound(w, r)
		return
	}
	if err != nil {
		log.Printf("история страницы: %v", err)
		http.Error(w, "не удалось открыть историю", http.StatusInternalServerError)
		return
	}

	read := domain.PagePath(branch.Name, view.Slug)
	if view.Slug == h.home {
		read = domain.PagePath(branch.Name, "")
	}

	markIndexable(w, false)
	h.view.History(w, usecase.PageScreen{
		PageView:    view,
		EditHref:    domain.EditPath(branch.Name, view.Slug),
		ReadHref:    read,
		HistoryHref: domain.HistoryPath(branch.Name, view.Slug),
	}, actor)
}

func (h *Handler) notFound(w http.ResponseWriter, r *http.Request) {
	markIndexable(w, false)
	h.view.NotFound(w, actorFrom(r))
}

func (h *Handler) denyPrivate(w http.ResponseWriter, r *http.Request) {
	markIndexable(w, false)
	http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
}

func (h *Handler) previewEdit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.view.Preview(w, r.FormValue("content"), requestedBranch(r))
}

func (h *Handler) asset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	name, ok := map[string]string{
		"/css/tailwindcss.css": "resources/css/tailwindcss.css",
		"/js/main.js":          "resources/js/main.js",
	}[r.URL.Path]
	if !ok {
		markIndexable(w, false)
		http.Error(w, "страница не найдена", http.StatusNotFound)
		return
	}

	http.ServeFile(w, r, name)
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
		h.notFound(w, r)
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
	slug, err := h.wiki.SavePage(r.Context(), branch, r.FormValue("slug"), r.FormValue("content"), h.sessionToken(r))
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
