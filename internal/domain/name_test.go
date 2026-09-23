package domain

import "testing"

func TestNormalizeName(t *testing.T) {
	got, err := NormalizeName("  Анна-Мария  ")
	if err != nil || got != "Анна-Мария" {
		t.Fatalf("получено %q ошибка %v", got, err)
	}

	for _, raw := range []string{"", "   ", "123", "Имя!", stringsTooLong()} {
		if _, err := NormalizeName(raw); err != ErrInvalidName {
			t.Fatalf("%q ошибка %v", raw, err)
		}
	}
}

func stringsTooLong() string {
	return "Аааааааааа" + "Аааааааааа" + "Аааааааааа" + "Аааааааааа" + "Аааааааааа" + "Аааааааааа" + "Аааааааааа" + "Аааааааааа" + "А"
}
