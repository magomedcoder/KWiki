package domain

import "errors"

var (
	ErrInvalidSlug        = errors.New("некорректный адрес страницы")
	ErrNotFound           = errors.New("не найдено")
	ErrInvalidEmail       = errors.New("некорректная почта")
	ErrWeakPassword       = errors.New("пароль должен быть 12-128 символов и содержать минимум три класса: строчные, прописные, цифры, знаки")
	ErrEmailTaken         = errors.New("пользователь уже есть")
	ErrInvalidCredentials = errors.New("неверная почта или пароль")
	ErrTooManyAttempts    = errors.New("слишком много попыток")
	ErrUnauthenticated    = errors.New("требуется вход")
	ErrCSRF               = errors.New("некорректный запрос")
	ErrInvalidName        = errors.New("укажите имя и фамилию")
	ErrBlocked            = errors.New("учётная запись заблокирована")
	ErrLastAdmin          = errors.New("нельзя удалить или заблокировать последнего администратора")
	ErrSelfAction         = errors.New("нельзя изменить свою учётную запись")
	ErrInvalidBranch      = errors.New("некорректное имя ветки")
	ErrBranchTaken        = errors.New("ветка уже есть")
	ErrDefaultBranch      = errors.New("нельзя удалить основную ветку")
	ErrInvalidMediaPath   = errors.New("некорректный путь медиа")
	ErrMediaExists        = errors.New("файл уже есть")
	ErrMediaTooLarge      = errors.New("файл больше 2 МБ")
)
