package domain

import "testing"

func TestHomeSlug(t *testing.T) {
	slug, err := HomeSlug(" README.md ")
	if err != nil || slug != "README" {
		t.Fatalf("адрес = %q, ошибка = %v", slug, err)
	}

	if _, err := HomeSlug(""); err != ErrInvalidSlug {
		t.Fatalf("ошибка = %v", err)
	}
}

func TestNormalizeSlug(t *testing.T) {
	slug, err := NormalizeSlug("/guides/intro/")
	if err != nil || slug != "guides/intro" {
		t.Fatalf("адрес = %q, ошибка = %v", slug, err)
	}

	for _, raw := range []string{"", "/", "../secret", `a\b`} {
		if _, err := NormalizeSlug(raw); err != ErrInvalidSlug {
			t.Fatalf("исходный %q: ошибка = %v", raw, err)
		}
	}
}

func TestPathAndTitle(t *testing.T) {
	if got := MarkdownPath("guides/intro"); got != "guides/intro.md" {
		t.Fatalf("путь = %q", got)
	}

	if got := MarkdownPath("readme.md"); got != "readme.md" {
		t.Fatalf("путь = %q", got)
	}

	if got := SlugFromPath("./guides/intro.md"); got != "guides/intro" {
		t.Fatalf("адрес = %q", got)
	}

	if got := TitleFromSlug("guides/quick-start"); got != "quick start" {
		t.Fatalf("заголовок = %q", got)
	}
}
