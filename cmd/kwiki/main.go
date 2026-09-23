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
	"github.com/magomedcoder/kwiki/internal/usecase"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "user" {
		if err := runUser(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		return
	}

	repoPath := flag.String("repo", "./data/wiki-content", "")
	dbPath := flag.String("db", "./data/wiki.db", "")
	addr := flag.String("addr", ":8000", "")
	secure := flag.Bool("secure", false, "")
	pepper := flag.String("pepper", "", "")
	flag.Parse()

	content, err := git.NewLocal(*repoPath)
	if err != nil {
		log.Fatalf("git: %v", err)
	}

	db, err := sqlite.Open(*dbPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}

	hasher, err := loadHasher(*pepper)
	if err != nil {
		log.Fatalf("password: %v", err)
	}

	wiki := usecase.New(db, content)
	if err := wiki.Sync(context.Background()); err != nil {
		log.Fatalf("sync: %v", err)
	}

	auth := usecase.NewAuth(db, db, db, hasher)
	if n, err := auth.UserCount(context.Background()); err != nil {
		log.Fatalf("users: %v", err)
	} else if n == 0 {
		log.Printf("нет пользователей: kwiki user add -db %s -email you@example.com", *dbPath)
	}

	views, err := html.Load("resources/templates")
	if err != nil {
		log.Fatalf("templates: %v", err)
	}

	handler := httpapi.New(wiki, auth, views, *secure)
	mux := http.NewServeMux()
	handler.Register(mux)

	if !*secure {
		log.Printf("куки сессии без постоянного Secure: для HTTPS укажите -secure")
	}
	log.Printf("KWiki запущен на %s (repo=%s, db=%s)", *addr, *repoPath, *dbPath)
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
