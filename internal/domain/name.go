package domain

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

func NormalizeName(raw string) (string, error) {
	name := strings.Join(strings.Fields(raw), " ")
	n := utf8.RuneCountInString(name)
	if n < 1 || n > 80 {
		return "", ErrInvalidName
	}

	letters := 0
	for _, r := range name {
		switch {
		case unicode.IsLetter(r):
			letters++
		case r == ' ' || r == '-' || r == '\'' || r == '’':
		default:
			return "", ErrInvalidName
		}
	}
	if letters == 0 {
		return "", ErrInvalidName
	}

	return name, nil
}
