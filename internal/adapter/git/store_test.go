package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBranchFilesAndRelocate(t *testing.T) {
	ctx := context.Background()
	store, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	wt, err := store.repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}

	loose := filepath.Join(store.root, "guides", "intro.md")
	if err := os.MkdirAll(filepath.Dir(loose), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(loose, []byte("# Привет\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := wt.Add("guides/intro.md"); err != nil {
		t.Fatal(err)
	}

	if err := store.commit(wt, "черновик"); err != nil {
		t.Fatal(err)
	}

	if err := store.RelocateLoose(ctx, "main", []string{"main"}); err != nil {
		t.Fatal(err)
	}

	files, err := store.ListMarkdown(ctx, "main")
	if err != nil || len(files) != 1 || files[0].Path != "guides/intro.md" {
		t.Fatalf("файлы = %+v, ошибка = %v", files, err)
	}

	data, err := store.Read(ctx, "main", "guides/intro.md")
	if err != nil || string(data) != "# Привет\n" {
		t.Fatalf("данные = %q, ошибка = %v", data, err)
	}

	if _, err := os.Stat(loose); !os.IsNotExist(err) {
		t.Fatalf("свободный файл всё ещё на месте: %v", err)
	}

	if err := store.Write(ctx, "docs", "a.md", []byte("doc"), "создание: docs/a"); err != nil {
		t.Fatal(err)
	}

	docs, err := store.ListMarkdown(ctx, "docs")
	if err != nil || len(docs) != 1 || docs[0].Path != "a.md" {
		t.Fatalf("документы = %+v, ошибка = %v", docs, err)
	}

	mainFiles, err := store.ListMarkdown(ctx, "main")
	if err != nil || len(mainFiles) != 1 {
		t.Fatalf("основная = %+v, ошибка = %v", mainFiles, err)
	}

	if err := store.RemoveBranch(ctx, "docs"); err != nil {
		t.Fatal(err)
	}

	docs, err = store.ListMarkdown(ctx, "docs")
	if err != nil || len(docs) != 0 {
		t.Fatalf("документы после удаления = %+v, ошибка = %v", docs, err)
	}

	mainFiles, err = store.ListMarkdown(ctx, "main")
	if err != nil || len(mainFiles) != 1 {
		t.Fatalf("основная после удаления = %+v, ошибка = %v", mainFiles, err)
	}
}
