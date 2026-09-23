package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/magomedcoder/kwiki/internal/adapter/sqlite"
	"github.com/magomedcoder/kwiki/internal/domain"
	"github.com/magomedcoder/kwiki/internal/usecase"
	"golang.org/x/term"
)

func runUser(args []string) error {
	if len(args) == 0 {
		return errors.New("укажите команду: add или passwd")
	}

	switch args[0] {
	case "add":
		return changeUser(args[1:], false)
	case "passwd":
		return changeUser(args[1:], true)
	default:
		return fmt.Errorf("неизвестная команда %q", args[0])
	}
}

func changeUser(args []string, update bool) error {
	fs := flag.NewFlagSet("user", flag.ContinueOnError)
	dbPath := fs.String("db", "./data/wiki.db", "")
	email := fs.String("email", "", "")
	passwordFlag := fs.String("password", "", "")
	pepperFlag := fs.String("pepper", "", "")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *email == "" {
		return errors.New("укажите -email")
	}

	secret, err := readSecret(*passwordFlag)
	if err != nil {
		return err
	}

	if secret == "" {
		return errors.New("пустой пароль")
	}

	db, err := sqlite.Open(*dbPath)
	if err != nil {
		return err
	}

	hasher, err := loadHasher(*pepperFlag)
	if err != nil {
		return err
	}

	auth := usecase.NewAuth(db, db, db, hasher)
	ctx := context.Background()
	if update {
		if err := auth.ChangePassword(ctx, *email, secret); err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return fmt.Errorf("пользователь %s не найден", *email)
			}
			return err
		}

		fmt.Printf("пароль обновлён, сессии пользователя закрыты\n")
		return nil
	}

	if err := auth.CreateUser(ctx, *email, secret); err != nil {
		return err
	}
	fmt.Printf("пользователь создан\n")

	return nil
}

func readSecret(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}

	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		fmt.Fprint(os.Stderr, "Пароль: ")
		buf, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		defer clearBytes(buf)
		return string(buf), nil
	}

	buf, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	defer clearBytes(buf)

	return strings.TrimRight(string(buf), "\r\n"), nil
}

func clearBytes(buf []byte) {
	for i := range buf {
		buf[i] = 0
	}
}
