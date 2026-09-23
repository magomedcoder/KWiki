package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/magomedcoder/kwiki/internal/domain"
)

func TestSaveAndViewPage(t *testing.T) {
	ctx := context.Background()
	pages := newMemPages()
	content := newMemContent()
	svc := New(pages, content)

	slug, err := svc.SavePage(ctx, "/guides/intro/", "# Привет")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if slug != "guides/intro" {
		t.Fatalf("slug = %q", slug)
	}
	if content.messages[0] != "create: guides/intro" {
		t.Fatalf("message = %q", content.messages[0])
	}

	view, err := svc.ViewPage(ctx, "guides/intro")
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	if view.Missing || view.Title != "intro" || view.Markdown != "# Привет" {
		t.Fatalf("view = %+v", view)
	}
	if len(view.Revisions) != 1 || view.Revisions[0].Author != "система" {
		t.Fatalf("revisions = %+v", view.Revisions)
	}

	if _, err := svc.SavePage(ctx, "guides/intro", "# Ещё"); err != nil {
		t.Fatalf("second save: %v", err)
	}
	if content.messages[1] != "edit: guides/intro" {
		t.Fatalf("message = %q", content.messages[1])
	}
}

func TestViewMissingAndInvalidSlug(t *testing.T) {
	svc := New(newMemPages(), newMemContent())

	view, err := svc.ViewPage(context.Background(), "нет-такой")
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	if !view.Missing || view.Slug != "нет-такой" {
		t.Fatalf("view = %+v", view)
	}

	if _, err := svc.ViewPage(context.Background(), "../secret"); !errors.Is(err, domain.ErrInvalidSlug) {
		t.Fatalf("err = %v", err)
	}
}

func TestEditForm(t *testing.T) {
	ctx := context.Background()
	content := newMemContent()
	svc := New(newMemPages(), content)

	form, err := svc.EditForm(ctx, "")
	if err != nil || !form.IsNew || form.Slug != "" {
		t.Fatalf("new form = %+v, err = %v", form, err)
	}

	if _, err := svc.SavePage(ctx, "home", "текст"); err != nil {
		t.Fatalf("save: %v", err)
	}

	form, err = svc.EditForm(ctx, "home")
	if err != nil || form.IsNew || form.Content != "текст" {
		t.Fatalf("edit form = %+v, err = %v", form, err)
	}
}

func TestSaveRejectsEmptySlug(t *testing.T) {
	_, err := New(newMemPages(), newMemContent()).SavePage(context.Background(), "///", "x")
	if !errors.Is(err, domain.ErrInvalidSlug) {
		t.Fatalf("err = %v", err)
	}
}

type memPages struct {
	pages []domain.Page
	revs  []domain.Revision
}

func newMemPages() *memPages { return &memPages{} }

func (m *memPages) List(context.Context) ([]domain.Page, error) {
	out := append([]domain.Page(nil), m.pages...)
	return out, nil
}

func (m *memPages) Revisions(_ context.Context, slug string, limit int) ([]domain.Revision, error) {
	var out []domain.Revision
	for i := len(m.revs) - 1; i >= 0; i-- {
		if m.revs[i].Slug != slug {
			continue
		}
		out = append(out, m.revs[i])
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (m *memPages) Upsert(_ context.Context, page domain.Page) error {
	for i := range m.pages {
		if m.pages[i].Slug == page.Slug {
			m.pages[i] = page
			return nil
		}
	}
	m.pages = append(m.pages, page)
	return nil
}

func (m *memPages) AddRevisionIfMissing(_ context.Context, rev domain.Revision) error {
	for _, existing := range m.revs {
		if existing.Slug == rev.Slug && existing.Hash == rev.Hash {
			return nil
		}
	}
	m.revs = append(m.revs, rev)
	return nil
}

type memContent struct {
	files    map[string][]byte
	messages []string
}

func newMemContent() *memContent {
	return &memContent{files: map[string][]byte{}}
}

func (m *memContent) ListMarkdown(context.Context) ([]domain.ContentFile, error) {
	var out []domain.ContentFile
	for path, data := range m.files {
		out = append(out, domain.ContentFile{
			Path: path,
			Hash: hashOf(data),
			Size: int64(len(data)),
		})
	}
	return out, nil
}

func (m *memContent) Read(_ context.Context, path string) ([]byte, error) {
	data, ok := m.files[path]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return append([]byte(nil), data...), nil
}

func (m *memContent) Write(_ context.Context, path string, content []byte, message string) error {
	m.files[path] = append([]byte(nil), content...)
	m.messages = append(m.messages, message)
	return nil
}

func hashOf(data []byte) string {
	return string(data)
}
