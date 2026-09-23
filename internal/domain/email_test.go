package domain

import "testing"

func TestNormalizeEmail(t *testing.T) {
	got, err := NormalizeEmail("  Admin@Example.com ")
	if err != nil || got != "admin@example.com" {
		t.Fatalf("получено %q ошибка %v", got, err)
	}

	for _, raw := range []string{"", "admin", "a@b", "a@b.c", "not an email@x.com", "a@@b.com"} {
		if _, err := NormalizeEmail(raw); err != ErrInvalidEmail {
			t.Fatalf("исходный %q ошибка %v", raw, err)
		}
	}
}
