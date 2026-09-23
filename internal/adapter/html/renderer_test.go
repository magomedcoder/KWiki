package html

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/magomedcoder/kwiki/internal/domain"
	"github.com/magomedcoder/kwiki/internal/usecase"
)

func TestEditShowsLivePreview(t *testing.T) {
	view, err := Load("../../../resources/templates")
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	view.Edit(rec, usecase.EditForm{
		Branch:  domain.DefaultBranch,
		Content: "# Привет\n\n**жирный**",
		IsNew:   true,
	}, nil, usecase.Actor{CSRF: "token"})

	body := rec.Body.String()
	if !strings.Contains(body, `id="привет"`) || !strings.Contains(body, ">Привет</h1>") || !strings.Contains(body, "<strong>жирный</strong>") {
		t.Fatalf("нет просмотра: %s", body)
	}

	if !strings.Contains(body, `src="/js/main.js"`) || !strings.Contains(body, `href="/css/tailwindcss.css"`) || !strings.Contains(body, "Просмотр") {
		t.Fatalf("нет элементов редактора")
	}
}

func TestMinifyKeepsMarkdownNewlines(t *testing.T) {
	in := "<textarea>\n# Hello\n\n**bold**\n</textarea>"
	out := minifyHTML(in)
	if out != in {
		t.Fatalf("результат = %q", out)
	}
}
