package domain

import "errors"

var (
	ErrInvalidSlug = errors.New("некорректный слаг")
	ErrNotFound    = errors.New("не найдено")
)
