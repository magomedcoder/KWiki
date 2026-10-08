package usecase

import (
	"context"
	"testing"

	"github.com/magomedcoder/kwiki/internal/domain"
)

func TestFilesBrowseUploadMoveDelete(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _ := newWiki(t)
	actor := Actor{
		Email: "a@example.com",
		Name:  "Editor",
	}

	if err := svc.CreateFile(ctx, "main", "notes.md", []byte("hi"), false, actor); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.UploadFile(ctx, "main", "media", "a.png", []byte("png"), false, actor); err != nil {
		t.Fatal(err)
	}

	if err := svc.CreateFile(ctx, "main", "drafts", nil, true, actor); err != nil {
		t.Fatal(err)
	}

	view, err := svc.BrowseFiles(ctx, "main", "", true)
	if err != nil || view.Empty {
		t.Fatalf("%+v %v", view, err)
	}

	if err := svc.MoveFilePath(ctx, "main", "notes.md", "drafts/notes.md", actor); err != nil {
		t.Fatal(err)
	}

	page, err := svc.ViewPage(ctx, "main", "drafts/notes")
	if err != nil || page.Missing {
		t.Fatalf("index after move: %+v %v", page, err)
	}

	if err := svc.DeleteFilePath(ctx, "main", "drafts/notes.md", actor); err != nil {
		t.Fatal(err)
	}

	page, err = svc.ViewPage(ctx, "main", "drafts/notes")
	if err != nil || !page.Missing {
		t.Fatalf("expected missing after delete: %+v %v", page, err)
	}

	if _, err := svc.UploadFile(ctx, "main", "", "big.bin", make([]byte, domain.MaxFileBytes+1), false, actor); err != domain.ErrFileTooLarge {
		t.Fatalf("size err %v", err)
	}
}
