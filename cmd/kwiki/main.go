package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	"github.com/magomedcoder/kwiki/internal/adapter/git"
	"github.com/magomedcoder/kwiki/internal/adapter/html"
	"github.com/magomedcoder/kwiki/internal/adapter/httpapi"
	"github.com/magomedcoder/kwiki/internal/adapter/sqlite"
	"github.com/magomedcoder/kwiki/internal/usecase"
)

func main() {
	repoPath := flag.String("repo", "./data/wiki-content", "")
	dbPath := flag.String("db", "./data/wiki.db", "")
	addr := flag.String("addr", ":8000", "")
	flag.Parse()

	content, err := git.NewLocal(*repoPath)
	if err != nil {
		log.Fatalf("git: %v", err)
	}

	pages, err := sqlite.Open(*dbPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}

	wiki := usecase.New(pages, content)
	if err := wiki.Sync(context.Background()); err != nil {
		log.Fatalf("sync: %v", err)
	}

	views, err := html.Load("resources/templates")
	if err != nil {
		log.Fatalf("templates: %v", err)
	}

	handler := httpapi.New(wiki, views)
	mux := http.NewServeMux()
	handler.Register(mux)

	log.Printf("KWiki запущен на %s (repo=%s, db=%s)", *addr, *repoPath, *dbPath)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
