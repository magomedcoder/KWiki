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

	slug, err := svc.SavePage(ctx, domain.DefaultBranch, "/guides/intro/", "# Привет\n\nТекст страницы", "")
	if err != nil {
		t.Fatalf("сохранение: %v", err)
	}
	if slug != "guides/intro" {
		t.Fatalf("адрес = %q", slug)
	}
	if content.messages[0] != "создание: main/guides/intro" {
		t.Fatalf("сообщение = %q", content.messages[0])
	}

	view, err := svc.ViewPage(ctx, domain.DefaultBranch, "guides/intro")
	if err != nil {
		t.Fatalf("просмотр: %v", err)
	}
	if view.Missing || view.Title != "intro" || !strings.Contains(view.Markdown, "Текст страницы") {
		t.Fatalf("вид = %+v", view)
	}
	if view.Description != "Текст страницы" {
		t.Fatalf("описание = %q", view.Description)
	}
	if len(view.Revisions) != 1 || view.Revisions[0].Author != "система" {
		t.Fatalf("правки = %+v", view.Revisions)
	}

	if _, err := svc.SavePage(ctx, domain.DefaultBranch, "guides/intro", "# Ещё", ""); err != nil {
		t.Fatalf("второе сохранение: %v", err)
	}
	if content.messages[1] != "правка: main/guides/intro" {
		t.Fatalf("сообщение = %q", content.messages[1])
	}
}

func TestBranchesIsolatePages(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _ := newWiki(t)

	if _, err := svc.CreateBranch(ctx, "Docs", true); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.SavePage(ctx, "docs", "home", "публично", ""); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.SavePage(ctx, domain.DefaultBranch, "home", "скрыто", ""); err != nil {
		t.Fatal(err)
	}

	public, err := svc.ViewPage(ctx, "docs", "home")
	if err != nil || public.Markdown != "публично" || !public.Public {
		t.Fatalf("публичные = %+v, ошибка = %v", public, err)
	}

	main, err := svc.ListPages(ctx, domain.DefaultBranch)
	if err != nil || len(main) != 1 || main[0].Slug != "home" {
		t.Fatalf("основная = %+v, ошибка = %v", main, err)
	}

	visible, err := svc.VisibleBranches(ctx, false)
	if err != nil || len(visible) != 1 || visible[0].Name != "docs" {
		t.Fatalf("видимые = %+v, ошибка = %v", visible, err)
	}

	entries, err := svc.Sitemap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Path == "/home" {
			t.Fatalf("приватная страница в карте сайта: %+v", entries)
		}
	}

	if len(entries) != 3 || entries[0].Path != "/" || entries[2].Path != "/b/docs/home" {
		t.Fatalf("карта сайта = %+v", entries)
	}
}

func TestDeleteDefaultBranch(t *testing.T) {
	svc, _, _, _ := newWiki(t)
	if err := svc.DeleteBranch(context.Background(), domain.DefaultBranch); !errors.Is(err, domain.ErrDefaultBranch) {
		t.Fatalf("ошибка = %v", err)
	}
}

func TestDeleteBranchRemovesPages(t *testing.T) {
	ctx := context.Background()
	svc, pages, content, branches := newWiki(t)
	if _, err := svc.CreateBranch(ctx, "docs", false); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.SavePage(ctx, "docs", "home", "текст", ""); err != nil {
		t.Fatal(err)
	}

	if err := svc.DeleteBranch(ctx, "docs"); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.OpenBranch(ctx, "docs"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("ошибка = %v", err)
	}

	if len(pages.pages) != 0 || len(content.files) != 0 || len(branches.items) != 1 {
		t.Fatalf("страниц %d файлов %d веток %d", len(pages.pages), len(content.files), len(branches.items))
	}
}

func TestViewMissingAndInvalidSlug(t *testing.T) {
	svc, _, _, _ := newWiki(t)

	view, err := svc.ViewPage(context.Background(), domain.DefaultBranch, "нет-такой")
	if err != nil {
		t.Fatalf("просмотр: %v", err)
	}
	if !view.Missing || view.Slug != "нет-такой" {
		t.Fatalf("вид = %+v", view)
	}

	if _, err := svc.ViewPage(context.Background(), domain.DefaultBranch, "../secret"); !errors.Is(err, domain.ErrInvalidSlug) {
		t.Fatalf("ошибка = %v", err)
	}
}

func TestEditForm(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _ := newWiki(t)

	form, err := svc.EditForm(ctx, domain.DefaultBranch, "")
	if err != nil || !form.IsNew || form.Slug != "" || form.Branch != domain.DefaultBranch {
		t.Fatalf("новая форма = %+v, ошибка = %v", form, err)
	}

	if _, err := svc.SavePage(ctx, domain.DefaultBranch, "home", "текст", ""); err != nil {
		t.Fatalf("сохранение: %v", err)
	}

	form, err = svc.EditForm(ctx, domain.DefaultBranch, "home")
	if err != nil || form.IsNew || form.Content != "текст" {
		t.Fatalf("форма правки = %+v, ошибка = %v", form, err)
	}
}

func TestSaveRejectsEmptySlug(t *testing.T) {
	svc, _, _, _ := newWiki(t)
	_, err := svc.SavePage(context.Background(), domain.DefaultBranch, "///", "x", "")
	if !errors.Is(err, domain.ErrInvalidSlug) {
		t.Fatalf("ошибка = %v", err)
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
	files, err := m.ListPrefix(context.Background(), branch, "")
	if err != nil {
		return nil, err
	}

	out := files[:0]
	for _, file := range files {
		if strings.HasSuffix(file.Path, ".md") {
			out = append(out, file)
		}
	}

	return out, nil
}

func (m *memContent) ListDir(ctx context.Context, branch, prefix string) ([]domain.DirEntry, error) {
	files, err := m.ListPrefix(ctx, branch, prefix)
	if err != nil {
		return nil, err
	}

	prefix, err = domain.NormalizeRepoPath(prefix)
	if err != nil {
		return nil, err
	}

	seen := map[string]domain.DirEntry{}
	prefixSlash := ""
	if prefix != "" {
		prefixSlash = prefix + "/"
	}

	for _, file := range files {
		rel := file.Path
		if prefixSlash != "" {
			if !strings.HasPrefix(rel, prefixSlash) {
				continue
			}

			rel = strings.TrimPrefix(rel, prefixSlash)
		}

		name, rest, _ := strings.Cut(rel, "/")
		if name == "" {
			continue
		}

		child := name
		if prefix != "" {
			child = prefix + "/" + name
		}

		if rest != "" {
			seen[name] = domain.DirEntry{
				Name: name,
				Path: child,
				Kind: domain.EntryDir,
			}
			continue
		}

		seen[name] = domain.DirEntry{
			Name: name,
			Path: child,
			Kind: domain.EntryFile,
			Size: file.Size,
			Hash: file.Hash,
		}
	}
	out := make([]domain.DirEntry, 0, len(seen))
	for _, e := range seen {
		out = append(out, e)
	}
	return out, nil
}

func (m *memContent) Stat(ctx context.Context, branch, path string) (domain.DirEntry, error) {
	path, err := domain.NormalizeRepoPath(path)
	if err != nil {
		return domain.DirEntry{}, err
	}

	if path == "" {
		return domain.DirEntry{
			Kind: domain.EntryDir,
			Name: branch,
		}, nil
	}

	if data, ok := m.files[contentKey(branch, path)]; ok {
		return domain.DirEntry{
			Name: domain.BaseName(path),
			Path: path,
			Kind: domain.EntryFile,
			Size: int64(len(data)),
		}, nil
	}

	entries, err := m.ListDir(ctx, branch, path)
	if err != nil {
		return domain.DirEntry{}, err
	}

	if len(entries) == 0 {
		prefixFiles, err := m.ListPrefix(ctx, branch, path+"/")
		if err != nil {
			return domain.DirEntry{}, err
		}

		if len(prefixFiles) == 0 {
			return domain.DirEntry{}, domain.ErrNotFound
		}
	}

	return domain.DirEntry{
		Name: domain.BaseName(path),
		Path: path,
		Kind: domain.EntryDir,
	}, nil
}

func (m *memContent) DeletePath(ctx context.Context, branch, path, message string) error {
	entry, err := m.Stat(ctx, branch, path)
	if err != nil {
		return err
	}

	var changes []domain.ContentChange
	if entry.Kind == domain.EntryFile {
		changes = []domain.ContentChange{
			{
				Path:   path,
				Delete: true,
			},
		}
	} else {
		files, err := m.ListPrefix(ctx, branch, path+"/")
		if err != nil {
			return err
		}

		for _, f := range files {
			changes = append(changes, domain.ContentChange{
				Path:   f.Path,
				Delete: true,
			})
		}
	}

	return m.WriteBatch(ctx, branch, changes, message)
}

func (m *memContent) MovePath(ctx context.Context, branch, from, to, message string) error {
	entry, err := m.Stat(ctx, branch, from)
	if err != nil {
		return err
	}

	if _, err := m.Stat(ctx, branch, to); err == nil {
		return domain.ErrPathExists
	} else if !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	var changes []domain.ContentChange
	if entry.Kind == domain.EntryFile {
		data, err := m.Read(ctx, branch, from)
		if err != nil {
			return err
		}

		changes = []domain.ContentChange{
			{
				Path: to,
				Data: data,
			},
			{
				Path:   from,
				Delete: true,
			},
		}
	} else {
		files, err := m.ListPrefix(ctx, branch, from+"/")
		if err != nil {
			return err
		}

		for _, f := range files {
			data, err := m.Read(ctx, branch, f.Path)
			if err != nil {
				return err
			}
			dest := to + strings.TrimPrefix(f.Path, from)
			changes = append(changes, domain.ContentChange{
				Path: dest,
				Data: data,
			}, domain.ContentChange{
				Path:   f.Path,
				Delete: true,
			})
		}
	}

	return m.WriteBatch(ctx, branch, changes, message)
}

func (m *memContent) ListPrefix(_ context.Context, branch, prefix string) ([]domain.ContentFile, error) {
	prefixKey := branch + "\x00"
	var out []domain.ContentFile
	for key, data := range m.files {
		if !strings.HasPrefix(key, prefixKey) {
			continue
		}

		path := strings.TrimPrefix(key, prefixKey)
		if prefix != "" && !strings.HasPrefix(path, prefix) {
			continue
		}
		out = append(out, domain.ContentFile{
			Path: path,
			Hash: hashOf(data),
			Size: int64(len(data)),
		})
	}

	return out, nil
}

func (m *memContent) Exists(_ context.Context, branch, path string) (bool, error) {
	_, ok := m.files[contentKey(branch, path)]
	return ok, nil
}

func (m *memContent) Read(_ context.Context, branch, path string) ([]byte, error) {
	data, ok := m.files[contentKey(branch, path)]
	if !ok {
		return nil, domain.ErrNotFound
	}

	return append([]byte(nil), data...), nil
}

func (m *memContent) Write(_ context.Context, branch, path string, content []byte, message string) error {
	return m.WriteBatch(context.Background(), branch, []domain.ContentChange{{
		Path: path,
		Data: content,
	}}, message)
}

func (m *memContent) WriteBatch(_ context.Context, branch string, changes []domain.ContentChange, message string) error {
	for _, change := range changes {
		key := contentKey(branch, change.Path)
		if change.Delete {
			delete(m.files, key)
			continue
		}
		m.files[key] = append([]byte(nil), change.Data...)
	}

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
