package domain

import "testing"

func TestNormalizeMediaRel(t *testing.T) {
	rel, err := NormalizeMediaRel("shots", "a.png")
	if err != nil || rel != "media/shots/a.png" {
		t.Fatalf("got %q %v", rel, err)
	}

	rel, err = NormalizeMediaRel("", "a.png")
	if err != nil || rel != "media/a.png" {
		t.Fatalf("got %q %v", rel, err)
	}

	if _, err := NormalizeMediaRel("../x", "a.png"); err == nil {
		t.Fatal("ожидали ошибку")
	}

	if _, err := NormalizeMediaRel("", "../a.png"); err == nil {
		t.Fatal("ожидали ошибку")
	}
}

func TestMediaURL(t *testing.T) {
	if got := MediaURL("docs", "media/a.png"); got != "/b/docs/media/a.png" {
		t.Fatalf("%s", got)
	}
}

func TestMediaRef(t *testing.T) {
	if got := MediaRef("media/shots/a.png"); got != "shots/a.png" {
		t.Fatalf("%s", got)
	}
	if got := MediaRef("media/a.png"); got != "a.png" {
		t.Fatalf("%s", got)
	}
}
