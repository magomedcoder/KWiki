package html

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/magomedcoder/kwiki/internal/domain"
	"github.com/magomedcoder/kwiki/internal/usecase"
)

func TestComponentsKeepMarkup(t *testing.T) {
	view, err := Load(nil)
	if err != nil {
		t.Fatal(err)
	}
	actor := usecase.Actor{
		Email: "a@example.com",
		Name:  "Юзер",
		Admin: true,
		CSRF:  "token",
	}

	edit := httptest.NewRecorder()
	view.Edit(edit, usecase.EditForm{
		Branch:  "main",
		Slug:    "sun",
		Content: "# Hi\n",
		IsNew:   false,
	}, nil, actor, "ru")
	users := httptest.NewRecorder()
	view.Users(users, usecase.UsersPage{
		Users: []usecase.ManagedUser{
			{
				Email:     "a@example.com",
				FirstName: "Юзер",
				LastName:  "Редактор",
				Self:      true,
				Admin:     true,
			},
		},
	}, actor, "ru")
	login := httptest.NewRecorder()
	view.Login(login, usecase.LoginPage{
		CSRF: "token",
	}, "ru")
	page := httptest.NewRecorder()
	view.Page(page, usecase.PageScreen{
		Title:    "Тема",
		Markdown: "Текст\n\n- пункт\n",
		EditHref: "/edit",
	}, actor, "ru")
	fresh := httptest.NewRecorder()
	view.Edit(fresh, usecase.EditForm{
		Branch: "main",
		IsNew:  true,
	}, []domain.Branch{
		{
			Name:   "main",
			Public: true,
		},
	}, actor, "ru")

	for name, body := range map[string]string{
		"edit":  edit.Body.String(),
		"users": users.Body.String(),
		"login": login.Body.String(),
		"page":  page.Body.String(),
		"fresh": fresh.Body.String(),
	} {
		if strings.Contains(body, "no value") {
			t.Errorf("%s содержит пустое значение шаблона", name)
		}

		if !strings.Contains(body, `href="/css/tailwindcss.css"`) || !strings.Contains(body, "max-md:") {
			t.Errorf("%s без стилей или мобильных классов", name)
		}
	}

	if !strings.Contains(edit.Body.String(), `src="/js/main.js"`) || !strings.Contains(edit.Body.String(), "data-before=\"**\"") || !strings.Contains(edit.Body.String(), `id="media-folder-pick"`) || !strings.Contains(edit.Body.String(), `data-media-open="upload"`) || !strings.Contains(edit.Body.String(), `data-media-open="pick"`) {
		t.Fatal("редактор потерял скрипт или кнопки")
	}

	usersBody := users.Body.String()
	if !strings.Contains(usersBody, "Новая страница") || !strings.Contains(usersBody, "Ветки") || !strings.Contains(usersBody, "Сменить пароль") || !strings.Contains(usersBody, `id="user-create-dialog"`) || !strings.Contains(usersBody, `id="user-edit-dialog"`) || !strings.Contains(usersBody, `id="user-delete-dialog"`) || !strings.Contains(usersBody, `id="password-dialog"`) || !strings.Contains(login.Body.String(), "Войти") || !strings.Contains(login.Body.String(), "login-card") {
		t.Fatal("шапка или форма входа собраны не так")
	}

	if strings.Contains(usersBody, `aria-label="Содержание"`) {
		t.Fatal("служебная страница показывает оглавление")
	}

	branches := httptest.NewRecorder()
	view.Branches(branches, usecase.BranchesPage{
		Branches: []domain.Branch{
			{
				Name:   "main",
				Public: true,
			},
		},
	}, actor, "ru")
	if !strings.Contains(branches.Body.String(), `id="branch-create-dialog"`) || !strings.Contains(branches.Body.String(), "Новая ветка") || strings.Contains(branches.Body.String(), "no value") {
		t.Fatal("окно новой ветки собрано не так")
	}

	if strings.Contains(page.Body.String(), "max-w-[46rem]") || strings.Contains(page.Body.String(), "max-w-5xl") {
		t.Fatal("страница снова собрана узкой колонкой")
	}

	guest := httptest.NewRecorder()
	view.Page(guest, usecase.PageScreen{
		Title:    "Введение",
		Slug:     "intro",
		Branch:   "docs",
		Markdown: "Текст\n\n## Установка\n\n### Шаг\n",
		Public:   true,
		Revisions: []domain.Revision{{
			Hash:      "deadbeef",
			Message:   "правка",
			CreatedAt: time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC),
		}},
		HistoryHref: "/history?branch=docs&slug=intro",
		Indexable:   true,
	}, usecase.Actor{}, "ru")
	body := guest.Body.String()
	if strings.Contains(body, "no value") || strings.Contains(body, "Новая страница") || strings.Contains(body, "Выйти") || strings.Contains(body, "Сменить пароль") || strings.Contains(body, "Править") || strings.Contains(body, "deadbeef") {
		t.Fatal("гостевая страница показывает правку, выход или историю")
	}

	if !strings.Contains(body, `justify-end`) || !strings.Contains(body, `href="/history?branch=docs&amp;slug=intro"`) {
		t.Fatal("кнопка истории не стоит справа внизу")
	}

	if !strings.Contains(body, `href="#page-title"`) || !strings.Contains(body, ">Установка<") || !strings.Contains(body, ">Шаг<") || !strings.Contains(body, ">Меню<") {
		t.Fatal("гостевая страница без оглавления")
	}

	if !strings.Contains(page.Body.String(), "Новая страница") || !strings.Contains(page.Body.String(), `href="#page-title"`) {
		t.Fatal("шапка или оглавление страницы собраны не так")
	}

	home := httptest.NewRecorder()
	view.Page(home, usecase.PageScreen{
		Title:    "Обзор",
		Slug:     "README",
		Branch:   "main",
		Markdown: "Текст",
		Public:   true,
		Home:     true,
	}, actor, "ru")
	homeBody := home.Body.String()
	if strings.Contains(homeBody, `href="true"`) || strings.Contains(homeBody, `href="false"`) || !strings.Contains(homeBody, `href="/"`) {
		t.Fatal("ссылка на главную указывает на true или false")
	}

	history := httptest.NewRecorder()
	view.History(history, usecase.PageScreen{
		Title: "Введение", Slug: "intro", Branch: "docs",
		Revisions: []domain.Revision{{
			Hash:      "deadbeef",
			Message:   "правка",
			CreatedAt: time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC),
		}},
		ReadHref: "/b/docs/intro",
		EditHref: "/edit?branch=docs&slug=intro",
	}, usecase.Actor{}, "ru")
	historyBody := history.Body.String()
	if !strings.Contains(historyBody, "deadbeef") || !strings.Contains(historyBody, "История: Введение") || !strings.Contains(historyBody, `href="/b/docs/intro"`) || strings.Contains(historyBody, "Править") {
		t.Fatal("страница истории собрана не так")
	}

	missing := httptest.NewRecorder()
	view.NotFound(missing, usecase.Actor{}, "ru")
	if missing.Code != http.StatusNotFound || !strings.Contains(missing.Body.String(), "Страница не найдена") || !strings.Contains(missing.Body.String(), `href="/"`) {
		t.Fatalf("страница 404: код %d", missing.Code)
	}

	en := httptest.NewRecorder()
	view.Login(en, usecase.LoginPage{CSRF: "token"}, "en")
	if !strings.Contains(en.Body.String(), "Sign in") || !strings.Contains(en.Body.String(), `lang="en"`) {
		t.Fatal("английская локаль не применилась")
	}

	help := httptest.NewRecorder()
	view.Help(help, usecase.Actor{}, "ru")
	helpBody := help.Body.String()
	needles := []string{
		"Как писать в Markdown",
		"# Большой заголовок",
		"###### Шестой уровень",
		"**важно**",
		"~~устарело~~",
		"1. сначала это",
		"&gt; Коротко о главном.",
		"| Колонка A |",
		"![alt](photo.png =400x300)",
		"clip.mp4",
		`href="/help"`,
	}

	for _, n := range needles {
		if !strings.Contains(helpBody, n) {
			t.Fatalf("справка без %q", n)
		}
	}

	if strings.Contains(helpBody, "no value") {
		t.Fatal("страница справки собрана не так")
	}
}
