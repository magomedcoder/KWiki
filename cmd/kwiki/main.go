package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/magomedcoder/kwiki/internal/adapter/git"
	"github.com/magomedcoder/kwiki/internal/adapter/html"
	"github.com/magomedcoder/kwiki/internal/adapter/httpapi"
	"github.com/magomedcoder/kwiki/internal/adapter/password"
	"github.com/magomedcoder/kwiki/internal/adapter/sqlite"
	"github.com/magomedcoder/kwiki/internal/domain"
	"github.com/magomedcoder/kwiki/internal/usecase"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "user" {
		if err := runUser(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		return
	}
	data := flag.String("data", "./data", "")
	addr := flag.String("addr", ":8000", "")
	secure := flag.Bool("secure", false, "")
	pepper := flag.String("pepper", "", "")
	homeFlag := flag.String("home", "README.md", "")
	flag.Parse()

	home, err := domain.HomeSlug(*homeFlag)
	if err != nil {
		log.Fatalf("главная страница: %v", err)
	}

	paths := pathsFrom(*data)
	content, err := git.NewLocal(paths.Repo)
	if err != nil {
		log.Fatalf("репозиторий: %v", err)
	}

	db, err := sqlite.Open(paths.DB)
	if err != nil {
		log.Fatalf("база: %v", err)
	}

	hasher, err := loadHasher(*pepper)
	if err != nil {
		log.Fatalf("пароль: %v", err)
	}

	wiki := usecase.New(db, db, content)
	if err := wiki.EnsureDefault(context.Background()); err != nil {
		log.Fatalf("ветки: %v", err)
	}
	if err := wiki.Sync(context.Background()); err != nil {
		log.Fatalf("синхронизация: %v", err)
	}

	auth := usecase.NewAuth(db, db, db, hasher)
	if err := auth.EnsureAdmin(context.Background()); err != nil {
		log.Fatalf("пользователи: %v", err)
	}

	if n, err := auth.UserCount(context.Background()); err != nil {
		log.Fatalf("пользователи: %v", err)
	} else if n == 0 {
		log.Printf("нет пользователей: kwiki user add -data %s -email kwiki@example.com -name Имя -surname Фамилия", paths.Root)
	}

	views, err := html.Load("resources/templates")
	if err != nil {
		log.Fatalf("шаблоны: %v", err)
	}

	handler := httpapi.New(wiki, auth, views, *secure, home)
	mux := http.NewServeMux()
	handler.Register(mux)

	if !*secure {
		log.Printf("куки сессии без постоянной защиты: для защищённого соединения укажите -secure")
	}
	log.Printf("KWiki запущен на %s (каталог %s)", *addr, paths.Root)
	log.Fatal(http.ListenAndServe(*addr, handler.Protect(mux)))
}

func loadHasher(raw string) (*password.Hasher, error) {
	if len(raw) < 16 {
		log.Printf("предупреждение: задайте -pepper длиной от 16 символов, один и тот же для команд user и сервера")
	}

	var pepper []byte
	if raw != "" {
		pepper = []byte(raw)
	}

	return password.Recommended(pepper)
}
