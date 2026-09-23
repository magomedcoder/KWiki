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
		return errors.New("укажите команду: add (создать), passwd (сменить пароль), block (заблокировать), unblock (разблокировать) или delete (удалить)")
	}

	switch args[0] {
	case "add":
		return addUser(args[1:])
	case "passwd":
		return changePassword(args[1:])
	case "delete":
		return deleteUser(args[1:])
	case "block":
		return setBlocked(args[1:], true)
	case "unblock":
		return setBlocked(args[1:], false)
	default:
		return fmt.Errorf("неизвестная команда %q", args[0])
	}
}

func addUser(args []string) error {
	fs := flag.NewFlagSet("user", flag.ContinueOnError)
	data := fs.String("data", "./data", "")
	email := fs.String("email", "", "")
	firstName := fs.String("name", "", "")
	lastName := fs.String("surname", "", "")
	passwordFlag := fs.String("password", "", "")
	pepperFlag := fs.String("pepper", "", "")
	admin := fs.Bool("admin", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *email == "" || *firstName == "" || *lastName == "" {
		return errors.New("укажите -email, -name и -surname")
	}

	secret, err := readSecret(*passwordFlag)
	if err != nil {
		return err
	}

	if secret == "" {
		return errors.New("пустой пароль")
	}

	auth, err := openAuth(pathsFrom(*data).DB, *pepperFlag)
	if err != nil {
		return err
	}
	if err := auth.CreateUser(context.Background(), usecase.Account{
		FirstName: *firstName,
		LastName:  *lastName,
		Email:     *email,
		Password:  secret,
		Admin:     *admin,
	}); err != nil {
		return err
	}
	fmt.Printf("пользователь создан\n")
	return nil
}

func changePassword(args []string) error {
	fs := flag.NewFlagSet("user", flag.ContinueOnError)
	data := fs.String("data", "./data", "")
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

	auth, err := openAuth(pathsFrom(*data).DB, *pepperFlag)
	if err != nil {
		return err
	}
	if err := auth.ChangePassword(context.Background(), *email, secret); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("пользователь %s не найден", *email)
		}
		return err
	}
	fmt.Printf("пароль обновлён, сессии пользователя закрыты\n")
	return nil
}

func deleteUser(args []string) error {
	email, auth, err := openUserCommand(args)
	if err != nil {
		return err
	}
	if err := auth.DeleteUser(context.Background(), "", email); err != nil {
		return err
	}
	fmt.Printf("пользователь удалён\n")
	return nil
}

func setBlocked(args []string, blocked bool) error {
	email, auth, err := openUserCommand(args)
	if err != nil {
		return err
	}
	if err := auth.SetBlocked(context.Background(), "", email, blocked); err != nil {
		return err
	}
	if blocked {
		fmt.Printf("пользователь заблокирован\n")
		return nil
	}
	fmt.Printf("блокировка снята\n")
	return nil
}

func openUserCommand(args []string) (string, *usecase.UserUseCase, error) {
	fs := flag.NewFlagSet("user", flag.ContinueOnError)
	data := fs.String("data", "./data", "")
	email := fs.String("email", "", "")
	if err := fs.Parse(args); err != nil {
		return "", nil, err
	}
	if *email == "" {
		return "", nil, errors.New("укажите -email")
	}
	auth, err := openAuth(pathsFrom(*data).DB, "")
	if err != nil {
		return "", nil, err
	}
	return *email, auth, nil
}

func openAuth(dbPath, pepper string) (*usecase.UserUseCase, error) {
	db, err := sqlite.Open(dbPath)
	if err != nil {
		return nil, err
	}
	if pepper == "" {
		return usecase.NewAuth(db, db, db, idleHasher{}), nil
	}
	hasher, err := loadHasher(pepper)
	if err != nil {
		return nil, err
	}
	return usecase.NewAuth(db, db, db, hasher), nil
}

type idleHasher struct{}

func (idleHasher) Hash(string) (string, error) {
	return "", errors.New("хеширование пароля недоступно")
}

func (idleHasher) Verify(string, string) (bool, error) {
	return false, errors.New("хеширование пароля недоступно")
}

func (idleHasher) Dummy() string { return "" }

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
