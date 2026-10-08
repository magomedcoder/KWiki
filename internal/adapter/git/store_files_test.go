package git

import (
	"context"
	"testing"

	"github.com/magomedcoder/kwiki/internal/domain"
)

func TestListDirCreateDeleteMove(t *testing.T) {
	ctx := context.Background()
	store, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Write(ctx, "main", "README.md", []byte("# Hi"), "создание"); err != nil {
		t.Fatal(err)
	}
	if err := store.Write(ctx, "main", "media/a.png", []byte("png"), "медиа"); err != nil {
		t.Fatal(err)
	}
	if err := store.Write(ctx, "main", "docs/.gitkeep", []byte{}, "папка"); err != nil {
		t.Fatal(err)
	}

	entries, err := store.ListDir(ctx, "main", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 3 {
		t.Fatalf("entries %+v", entries)
	}
	var sawDir, sawMD bool
	for _, e := range entries {
		if e.Name == "docs" && e.Kind == domain.EntryDir {
			sawDir = true
		}
		if e.Name == "README.md" && e.Kind == domain.EntryFile {
			sawMD = true
		}
	}
	if !sawDir || !sawMD {
		t.Fatalf("entries %+v", entries)
	}

	if err := store.MovePath(ctx, "main", "README.md", "docs/README.md", "перемещение"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Stat(ctx, "main", "README.md"); err == nil {
		t.Fatal("старый путь остался")
	}
	st, err := store.Stat(ctx, "main", "docs/README.md")
	if err != nil || st.Kind != domain.EntryFile {
		t.Fatalf("stat %+v %v", st, err)
	}

	if err := store.DeletePath(ctx, "main", "docs", "удаление"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Stat(ctx, "main", "docs/README.md"); err == nil {
		t.Fatal("файл не удалён")
	}
}

func TestNormalizeRejectsTraversal(t *testing.T) {
	if _, err := domain.NormalizeRepoPath("../secret"); err == nil {
		t.Fatal("ожидалась ошибка")
	}
	if _, err := domain.NormalizeRepoPath(".git/config"); err == nil {
		t.Fatal("ожидалась ошибка")
	}
}
