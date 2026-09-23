package usecase

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/magomedcoder/kwiki/internal/domain"
)

func TestMediaStageSaveAndDelete(t *testing.T) {
	svc, _, content, _ := newWiki(t)
	ctx := context.Background()
	draft := "session-1"

	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	item, err := svc.StageMedia(ctx, draft, domain.DefaultBranch, "shots", "a.png", png, false)
	if err != nil {
		t.Fatal(err)
	}
	if item.URL != "/b/main/media/shots/a.png" || item.Ref != "shots/a.png" || !item.Staged {
		t.Fatalf("%+v", item)
	}

	if _, err := svc.StageMedia(ctx, draft, domain.DefaultBranch, "shots", "a.png", png, false); !errors.Is(err, domain.ErrMediaExists) {
		t.Fatalf("ошибка = %v", err)
	}

	big := bytes.Repeat([]byte("x"), domain.MaxMediaBytes+1)
	if _, err := svc.StageMedia(ctx, draft, domain.DefaultBranch, "", "big.bin", big, false); !errors.Is(err, domain.ErrMediaTooLarge) {
		t.Fatalf("ошибка = %v", err)
	}

	slug, err := svc.SavePage(ctx, domain.DefaultBranch, "home", "см. ![a](/b/main/media/shots/a.png)", draft)
	if err != nil || slug != "home" {
		t.Fatalf("slug=%s err=%v", slug, err)
	}

	if _, ok := content.files[contentKey(domain.DefaultBranch, "media/shots/a.png")]; !ok {
		t.Fatal("медиа не записано в git")
	}

	if !strings.Contains(string(content.files[contentKey(domain.DefaultBranch, "home.md")]), "/b/main/media/shots/a.png") {
		t.Fatal("страница без ссылки")
	}

	if len(content.messages) == 0 || !strings.Contains(content.messages[len(content.messages)-1], "создание: main/home") {
		t.Fatalf("сообщения %+v", content.messages)
	}

	list, err := svc.ListMedia(ctx, "", domain.DefaultBranch)
	if err != nil || len(list) != 1 || list[0].Staged {
		t.Fatalf("%+v %v", list, err)
	}

	if err := svc.DeleteMedia(ctx, "", domain.DefaultBranch, "media/shots/a.png"); err != nil {
		t.Fatal(err)
	}

	if _, ok := content.files[contentKey(domain.DefaultBranch, "media/shots/a.png")]; ok {
		t.Fatal("медиа осталось после удаления")
	}
}

func TestCopyMediaOnSaveToOtherBranch(t *testing.T) {
	svc, _, content, _ := newWiki(t)
	ctx := context.Background()

	if err := content.Write(ctx, domain.DefaultBranch, "media/pic.png", []byte("img"), "медиа"); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.CreateBranch(ctx, "docs", true); err != nil {
		t.Fatal(err)
	}

	md := "рисунок ![x](/b/main/media/pic.png)"
	if _, err := svc.SavePage(ctx, "docs", "guide", md, ""); err != nil {
		t.Fatal(err)
	}

	data, err := content.Read(ctx, "docs", "media/pic.png")
	if err != nil || string(data) != "img" {
		t.Fatalf("копия = %q %v", data, err)
	}

	page, err := content.Read(ctx, "docs", "guide.md")
	if err != nil || !strings.Contains(string(page), "](pic.png)") || strings.Contains(string(page), "/b/main/media/pic.png") {
		t.Fatalf("страница = %q", page)
	}
}
