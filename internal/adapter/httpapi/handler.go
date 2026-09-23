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
	Index(w http.ResponseWriter, pages []domain.Page, actor usecase.Actor)

	Page(w http.ResponseWriter, view usecase.PageView, actor usecase.Actor)

	Edit(w http.ResponseWriter, form usecase.EditForm, actor usecase.Actor)

	Login(w http.ResponseWriter, page usecase.LoginPage)

	Users(w http.ResponseWriter, page usecase.UsersPage, actor usecase.Actor)
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
	mux.Handle("/users", h.requireAuth(http.HandlerFunc(h.users)))
	mux.Handle("/logout", h.requireAuth(http.HandlerFunc(h.logout)))
	mux.Handle("/edit", h.requireAuth(http.HandlerFunc(h.edit)))
	mux.Handle("/", h.requireAuth(http.HandlerFunc(h.index)))
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		h.viewPage(w, r)
		return
	}

	pages, err := h.wiki.ListPages(r.Context())
	if err != nil {
		log.Printf("список страниц: %v", err)
		http.Error(w, "не удалось загрузить страницы", http.StatusInternalServerError)
		return
	}
	h.view.Index(w, pages, actorFrom(r))
}

func (h *Handler) viewPage(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/")
	view, err := h.wiki.ViewPage(r.Context(), slug)
	if errors.Is(err, domain.ErrInvalidSlug) {
		http.Error(w, "страница не найдена", http.StatusNotFound)
		return
	}

	if err != nil {
		log.Printf("просмотр страницы: %v", err)
		http.Error(w, "не удалось открыть страницу", http.StatusInternalServerError)
		return
	}

	h.view.Page(w, view, actorFrom(r))
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
		http.Error(w, "некорректный адрес страницы", http.StatusBadRequest)
		return
	}

	if err != nil {
		log.Printf("форма правки: %v", err)
		http.Error(w, "не удалось открыть форму", http.StatusInternalServerError)
		return
	}

	h.view.Edit(w, form, actorFrom(r))
}

func (h *Handler) savePage(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "некорректный запрос", http.StatusBadRequest)
		return
	}

	slug, err := h.wiki.SavePage(r.Context(), r.FormValue("slug"), r.FormValue("content"))
	if err != nil && !errors.Is(err, usecase.ErrIndexSync) {
		if errors.Is(err, domain.ErrInvalidSlug) {
			http.Error(w, "некорректный адрес страницы", http.StatusBadRequest)
			return
		}

		log.Printf("сохранение страницы: %v", err)
		http.Error(w, "не удалось сохранить страницу", http.StatusInternalServerError)
		return
	}

	if err != nil {
		log.Printf("синхронизация индекса: %v", err)
	}

	http.Redirect(w, r, "/"+slug, http.StatusSeeOther)
}
