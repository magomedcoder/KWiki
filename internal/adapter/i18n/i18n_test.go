package i18n

import (
	"testing"
	"testing/fstest"

	"github.com/magomedcoder/kwiki/resources"
)

func TestLoadAndFallback(t *testing.T) {
	bundle, err := Load(resources.FS)
	if err != nil {
		t.Fatal(err)
	}

	if !bundle.Supported("ru") || !bundle.Supported("en") {
		t.Fatalf("supported %+v", bundle.Languages())
	}

	if got := bundle.T("en", "nav.pages"); got != "Pages" {
		t.Fatalf("en = %q", got)
	}

	if got := bundle.T("ru", "nav.pages"); got != "Страницы" {
		t.Fatalf("ru = %q", got)
	}

	if got := bundle.T("de", "nav.pages"); got != "Страницы" {
		t.Fatalf("fallback = %q", got)
	}

	if got := bundle.T("en", "page.history_title", "Intro"); got != "History: Intro" {
		t.Fatalf("format = %q", got)
	}

	if got := bundle.T("en", "missing.key"); got != "missing.key" {
		t.Fatalf("missing = %q", got)
	}

	if got := bundle.Locale("en"); got != "en_US" {
		t.Fatalf("locale = %q", got)
	}

	client := bundle.ClientMessages("en")
	if client["js.media.no_file"] != "No file selected" {
		t.Fatalf("client %+v", client)
	}
}

func TestResolveOrder(t *testing.T) {
	bundle, err := Load(resources.FS)
	if err != nil {
		t.Fatal(err)
	}

	if got := bundle.Resolve("en", "ru"); got != "en" {
		t.Fatalf("cookie = %q", got)
	}

	if got := bundle.Resolve("de", "en-US,en;q=0.8"); got != "en" {
		t.Fatalf("accept = %q", got)
	}

	if got := bundle.Resolve("", "fr"); got != "ru" {
		t.Fatalf("default = %q", got)
	}
}

func TestParseAcceptLanguage(t *testing.T) {
	got := ParseAcceptLanguage("fr-CH, fr;q=0.9, en;q=0.8, de;q=0.7, *;q=0.5", []string{"ru", "en"})
	if got != "en" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalize(t *testing.T) {
	if got := Normalize("en-US"); got != "en" {
		t.Fatalf("got %q", got)
	}

	if got := Normalize(" RU_ru "); got != "ru" {
		t.Fatalf("got %q", got)
	}
}

func TestMissingLocales(t *testing.T) {
	_, err := Load(fstest.MapFS{})
	if err == nil {
		t.Fatal("expected error")
	}
}
