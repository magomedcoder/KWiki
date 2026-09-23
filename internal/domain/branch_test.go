package domain

import "testing"

func TestNormalizeBranch(t *testing.T) {
	name, err := NormalizeBranch(" Docs ")
	if err != nil || name != "docs" {
		t.Fatalf("name = %q, err = %v", name, err)
	}

	if _, err := NormalizeBranch("main"); err != nil {
		t.Fatal(err)
	}

	for _, raw := range []string{"", "-docs", "docs-", "do--cs", "a/b", "ветка", "docs.name"} {
		if _, err := NormalizeBranch(raw); err != ErrInvalidBranch {
			t.Fatalf("%q err = %v", raw, err)
		}
	}
}

func TestPagePath(t *testing.T) {
	if got := PagePath(DefaultBranch, ""); got != "/" {
		t.Fatalf("root = %q", got)
	}

	if got := PagePath(DefaultBranch, "guides/intro"); got != "/guides/intro" {
		t.Fatalf("page = %q", got)
	}

	if got := PagePath("docs", ""); got != "/b/docs" {
		t.Fatalf("branch = %q", got)
	}

	if got := PagePath("docs", "a/b"); got != "/b/docs/a/b" {
		t.Fatalf("nested = %q", got)
	}

	if got := EditPath("docs", "a"); got != "/edit?branch=docs&slug=a" {
		t.Fatalf("edit = %q", got)
	}
}
