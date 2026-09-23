package domain

import "errors"

var (
	ErrInvalidSlug        = errors.New("некорректный слаг")
	ErrNotFound           = errors.New("не найдено")
	ErrInvalidEmail       = errors.New("некорректная почта")
	ErrWeakPassword       = errors.New("пароль должен быть 12-128 символов и содержать минимум три класса: строчные, прописные, цифры, знаки")
	ErrEmailTaken         = errors.New("пользователь уже есть")
	ErrInvalidCredentials = errors.New("неверная почта или пароль")
	ErrTooManyAttempts    = errors.New("слишком много попыток")
	ErrUnauthenticated    = errors.New("требуется вход")
	ErrCSRF               = errors.New("некорректный запрос")
)
