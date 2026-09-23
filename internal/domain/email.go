package domain

import (
	"regexp"
	"strings"
)

var emailRE = regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*\.[a-z]{2,}$`)

func NormalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if len(email) > 254 || !emailRE.MatchString(email) {
		return "", ErrInvalidEmail
	}

	return email, nil
}
