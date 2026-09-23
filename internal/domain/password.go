package domain

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

var commonPasswords = map[string]struct{}{
	"password1234":  {},
	"password12345": {},
	"qwerty12345":   {},
	"qwerty123456":  {},
	"admin12345":    {},
	"admin123456":   {},
	"welcome12345":  {},
	"changeme123":   {},
	"letmein1234":   {},
	"iloveyou123":   {},
}

func ValidatePassword(email, password string) error {
	n := utf8.RuneCountInString(password)
	if n < 12 || n > 128 {
		return ErrWeakPassword
	}

	lowerEmail := strings.ToLower(email)
	lowerPassword := strings.ToLower(password)
	if lowerPassword == lowerEmail {
		return ErrWeakPassword
	}

	local, _, _ := strings.Cut(lowerEmail, "@")
	if len(local) >= 4 && strings.Contains(lowerPassword, local) {
		return ErrWeakPassword
	}

	if _, ok := commonPasswords[lowerPassword]; ok {
		return ErrWeakPassword
	}

	var lower, upper, digit, symbol bool
	for _, r := range password {
		switch {
		case unicode.IsControl(r):
			return ErrWeakPassword
		case unicode.IsLower(r):
			lower = true
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsDigit(r):
			digit = true
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			symbol = true
		default:
			if !unicode.IsSpace(r) {
				symbol = true
			}
		}
	}

	classes := 0
	for _, ok := range []bool{lower, upper, digit, symbol} {
		if ok {
			classes++
		}
	}

	if classes < 3 {
		return ErrWeakPassword
	}

	return nil
}
