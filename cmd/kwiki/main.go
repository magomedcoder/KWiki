package main

import (
	"flag"
	"github.com/magomedcoder/kwiki/internal/db"
	"github.com/magomedcoder/kwiki/internal/gitstore"
	"github.com/magomedcoder/kwiki/internal/handlers"
	"github.com/magomedcoder/kwiki/internal/render"
	"log"
	"net/http"
)

func main() {
	repoPath := flag.String("repo", "./data/wiki-content", "")
	dbPath := flag.String("db", "./data/wiki.db", "")
	addr := flag.String("addr", ":8000", "")
	flag.Parse()

	store, err := gitstore.NewLocal(*repoPath)
	if err != nil {
		log.Fatalf("git: %v", err)
	}

	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}

	if err := db.SyncIndex(database, store); err != nil {
		log.Fatalf("sync: %v", err)
	}

	render.LoadTemplates("resources/templates")

	h := handlers.New(store, database)
	http.HandleFunc("/", h.Index)
	http.HandleFunc("/edit", h.Edit)

	log.Printf("KWiki запущен на %s (repo=%s, db=%s)", *addr, *repoPath, *dbPath)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
