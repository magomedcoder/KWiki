package domain

import "testing"

func TestNormalizeBranch(t *testing.T) {
	name, err := NormalizeBranch(" Docs ")
	if err != nil || name != "docs" {
		t.Fatalf("имя = %q, ошибка = %v", name, err)
	}

	if _, err := NormalizeBranch("main"); err != nil {
		t.Fatal(err)
	}

	for _, raw := range []string{"", "-docs", "docs-", "do--cs", "a/b", "ветка", "docs.name"} {
		if _, err := NormalizeBranch(raw); err != ErrInvalidBranch {
			t.Fatalf("%q ошибка = %v", raw, err)
		}
	}
}

func TestPagePath(t *testing.T) {
	if got := PagePath(DefaultBranch, ""); got != "/" {
		t.Fatalf("корень = %q", got)
	}

	if got := PagePath(DefaultBranch, "guides/intro"); got != "/guides/intro" {
		t.Fatalf("страница = %q", got)
	}

	if got := PagePath("docs", ""); got != "/b/docs" {
		t.Fatalf("ветка = %q", got)
	}

	if got := PagePath("docs", "a/b"); got != "/b/docs/a/b" {
		t.Fatalf("вложенный = %q", got)
	}

	if got := EditPath("docs", "a"); got != "/edit?branch=docs&slug=a" {
		t.Fatalf("правка = %q", got)
	}
}
