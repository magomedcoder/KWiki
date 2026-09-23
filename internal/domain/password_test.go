package domain

import "testing"

func TestValidatePassword(t *testing.T) {
	if err := ValidatePassword("admin@example.com", "S3cure-Wiki-Pass"); err != nil {
		t.Fatal(err)
	}

	if err := ValidatePassword("admin@example.com", "Пароль12345!"); err != nil {
		t.Fatal(err)
	}

	weak := []string{
		"short1A!",
		"alllowercase1",
		"S3cure-Wiki-Pass-admin",
		"admin@example.com",
		"Password12345",
	}
	for _, password := range weak {
		if err := ValidatePassword("admin@example.com", password); err != ErrWeakPassword {
			t.Fatalf("%q err %v", password, err)
		}
	}
}
