package usecase

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/magomedcoder/kwiki/internal/domain"
)

func TestSaveAndViewPage(t *testing.T) {
	ctx := context.Background()
	svc, _, content, _ := newWiki(t)

	slug, err := svc.SavePage(ctx, domain.DefaultBranch, "/guides/intro/", "# Привет\n\nТекст страницы")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if slug != "guides/intro" {
		t.Fatalf("slug = %q", slug)
	}
	if content.messages[0] != "создание: main/guides/intro" {
		t.Fatalf("message = %q", content.messages[0])
	}

	view, err := svc.ViewPage(ctx, domain.DefaultBranch, "guides/intro")
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	if view.Missing || view.Title != "intro" || !strings.Contains(view.Markdown, "Текст страницы") {
		t.Fatalf("view = %+v", view)
	}
	if view.Description != "Текст страницы" {
		t.Fatalf("description = %q", view.Description)
	}
	if len(view.Revisions) != 1 || view.Revisions[0].Author != "система" {
		t.Fatalf("revisions = %+v", view.Revisions)
	}

	if _, err := svc.SavePage(ctx, domain.DefaultBranch, "guides/intro", "# Ещё"); err != nil {
		t.Fatalf("second save: %v", err)
	}
	if content.messages[1] != "правка: main/guides/intro" {
		t.Fatalf("message = %q", content.messages[1])
	}
}

func TestBranchesIsolatePages(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _ := newWiki(t)

	if _, err := svc.CreateBranch(ctx, "Docs", true); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.SavePage(ctx, "docs", "home", "публично"); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.SavePage(ctx, domain.DefaultBranch, "home", "скрыто"); err != nil {
		t.Fatal(err)
	}

	public, err := svc.ViewPage(ctx, "docs", "home")
	if err != nil || public.Markdown != "публично" || !public.Public {
		t.Fatalf("public = %+v, err = %v", public, err)
	}

	main, err := svc.ListPages(ctx, domain.DefaultBranch)
	if err != nil || len(main) != 1 || main[0].Slug != "home" {
		t.Fatalf("main = %+v, err = %v", main, err)
	}

	visible, err := svc.VisibleBranches(ctx, false)
	if err != nil || len(visible) != 1 || visible[0].Name != "docs" {
		t.Fatalf("visible = %+v, err = %v", visible, err)
	}

	entries, err := svc.Sitemap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Path == "/home" {
			t.Fatalf("private page in sitemap: %+v", entries)
		}
	}

	if len(entries) != 3 || entries[0].Path != "/" || entries[2].Path != "/b/docs/home" {
		t.Fatalf("sitemap = %+v", entries)
	}
}

func TestDeleteDefaultBranch(t *testing.T) {
	svc, _, _, _ := newWiki(t)
	if err := svc.DeleteBranch(context.Background(), domain.DefaultBranch); !errors.Is(err, domain.ErrDefaultBranch) {
		t.Fatalf("err = %v", err)
	}
}

func TestDeleteBranchRemovesPages(t *testing.T) {
	ctx := context.Background()
	svc, pages, content, branches := newWiki(t)
	if _, err := svc.CreateBranch(ctx, "docs", false); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.SavePage(ctx, "docs", "home", "текст"); err != nil {
		t.Fatal(err)
	}

	if err := svc.DeleteBranch(ctx, "docs"); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.OpenBranch(ctx, "docs"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}

	if len(pages.pages) != 0 || len(content.files) != 0 || len(branches.items) != 1 {
		t.Fatalf("pages %d files %d branches %d", len(pages.pages), len(content.files), len(branches.items))
	}
}

func TestViewMissingAndInvalidSlug(t *testing.T) {
	svc, _, _, _ := newWiki(t)

	view, err := svc.ViewPage(context.Background(), domain.DefaultBranch, "нет-такой")
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	if !view.Missing || view.Slug != "нет-такой" {
		t.Fatalf("view = %+v", view)
	}

	if _, err := svc.ViewPage(context.Background(), domain.DefaultBranch, "../secret"); !errors.Is(err, domain.ErrInvalidSlug) {
		t.Fatalf("err = %v", err)
	}
}

func TestEditForm(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _ := newWiki(t)

	form, err := svc.EditForm(ctx, domain.DefaultBranch, "")
	if err != nil || !form.IsNew || form.Slug != "" || form.Branch != domain.DefaultBranch {
		t.Fatalf("new form = %+v, err = %v", form, err)
	}

	if _, err := svc.SavePage(ctx, domain.DefaultBranch, "home", "текст"); err != nil {
		t.Fatalf("save: %v", err)
	}

	form, err = svc.EditForm(ctx, domain.DefaultBranch, "home")
	if err != nil || form.IsNew || form.Content != "текст" {
		t.Fatalf("edit form = %+v, err = %v", form, err)
	}
}

func TestSaveRejectsEmptySlug(t *testing.T) {
	svc, _, _, _ := newWiki(t)
	_, err := svc.SavePage(context.Background(), domain.DefaultBranch, "///", "x")
	if !errors.Is(err, domain.ErrInvalidSlug) {
		t.Fatalf("err = %v", err)
	}
}

func newWiki(t *testing.T) (*PageUseCase, *memPages, *memContent, *memBranches) {
	t.Helper()
	pages := newMemPages()
	content := newMemContent()
	branches := &memBranches{}
	svc := New(pages, branches, content)
	if err := svc.EnsureDefault(context.Background()); err != nil {
		t.Fatal(err)
	}

	return svc, pages, content, branches
}

type memPages struct {
	pages []domain.Page
	revs  []domain.Revision
}

func newMemPages() *memPages { return &memPages{} }

func (m *memPages) List(_ context.Context, branch string) ([]domain.Page, error) {
	var out []domain.Page
	for _, page := range m.pages {
		if page.Branch == branch {
			out = append(out, page)
		}
	}

	return out, nil
}

func (m *memPages) Revisions(_ context.Context, branch, slug string, limit int) ([]domain.Revision, error) {
	var out []domain.Revision
	for _, rev := range slices.Backward(m.revs) {
		if rev.Branch != branch || rev.Slug != slug {
			continue
		}

		out = append(out, rev)
		if limit > 0 && len(out) >= limit {
			break
		}
	}

	return out, nil
}

func (m *memPages) Upsert(_ context.Context, page domain.Page) error {
	for i := range m.pages {
		if m.pages[i].Branch == page.Branch && m.pages[i].Slug == page.Slug {
			m.pages[i] = page
			return nil
		}
	}

	m.pages = append(m.pages, page)
	return nil
}

func (m *memPages) AddRevisionIfMissing(_ context.Context, rev domain.Revision) error {
	for _, existing := range m.revs {
		if existing.Branch == rev.Branch && existing.Slug == rev.Slug && existing.Hash == rev.Hash {
			return nil
		}
	}
	m.revs = append(m.revs, rev)
	return nil
}

func (m *memPages) Prune(_ context.Context, branch string, slugs []string) error {
	keep := map[string]bool{}
	for _, slug := range slugs {
		keep[slug] = true
	}

	filtered := m.pages[:0]
	for _, page := range m.pages {
		if page.Branch == branch && !keep[page.Slug] {
			continue
		}
		filtered = append(filtered, page)
	}

	m.pages = filtered
	return nil
}

func (m *memPages) DeleteBranchPages(_ context.Context, branch string) error {
	filtered := m.pages[:0]
	for _, page := range m.pages {
		if page.Branch != branch {
			filtered = append(filtered, page)
		}
	}

	m.pages = filtered
	revs := m.revs[:0]
	for _, rev := range m.revs {
		if rev.Branch != branch {
			revs = append(revs, rev)
		}
	}

	m.revs = revs
	return nil
}

type memContent struct {
	files    map[string][]byte
	messages []string
}

func newMemContent() *memContent {
	return &memContent{files: map[string][]byte{}}
}

func contentKey(branch, path string) string {
	return branch + "\x00" + path
}

func (m *memContent) ListMarkdown(_ context.Context, branch string) ([]domain.ContentFile, error) {
	prefix := branch + "\x00"
	var out []domain.ContentFile
	for key, data := range m.files {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		out = append(out, domain.ContentFile{
			Path: strings.TrimPrefix(key, prefix),
			Hash: hashOf(data),
			Size: int64(len(data)),
		})
	}

	return out, nil
}

func (m *memContent) Read(_ context.Context, branch, path string) ([]byte, error) {
	data, ok := m.files[contentKey(branch, path)]
	if !ok {
		return nil, domain.ErrNotFound
	}

	return append([]byte(nil), data...), nil
}

func (m *memContent) Write(_ context.Context, branch, path string, content []byte, message string) error {
	m.files[contentKey(branch, path)] = append([]byte(nil), content...)
	m.messages = append(m.messages, message)
	return nil
}

func (m *memContent) RemoveBranch(_ context.Context, branch string) error {
	prefix := branch + "\x00"
	for key := range m.files {
		if strings.HasPrefix(key, prefix) {
			delete(m.files, key)
		}
	}

	return nil
}

func (m *memContent) RelocateLoose(context.Context, string, []string) error {
	return nil
}

func hashOf(data []byte) string {
	return string(data)
}

type memBranches struct {
	items []domain.Branch
}

func (m *memBranches) CreateBranch(_ context.Context, branch domain.Branch) error {
	for _, existing := range m.items {
		if existing.Name == branch.Name {
			return domain.ErrBranchTaken
		}
	}

	m.items = append(m.items, branch)
	return nil
}

func (m *memBranches) FindBranch(_ context.Context, name string) (domain.Branch, error) {
	for _, branch := range m.items {
		if branch.Name == name {
			return branch, nil
		}
	}

	return domain.Branch{}, domain.ErrNotFound
}

func (m *memBranches) ListBranches(context.Context) ([]domain.Branch, error) {
	return append([]domain.Branch(nil), m.items...), nil
}

func (m *memBranches) SetBranchPublic(_ context.Context, name string, public bool) error {
	for i := range m.items {
		if m.items[i].Name == name {
			m.items[i].Public = public
			return nil
		}
	}

	return domain.ErrNotFound
}

func (m *memBranches) DeleteBranch(_ context.Context, name string) error {
	for i := range m.items {
		if m.items[i].Name == name {
			m.items = append(m.items[:i], m.items[i+1:]...)
			return nil
		}
	}

	return domain.ErrNotFound
}
