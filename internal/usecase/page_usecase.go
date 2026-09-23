package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/magomedcoder/kwiki/internal/domain"
)

var ErrIndexSync = errors.New("синхронизация индекса")

type PageView struct {
	Title       string
	Slug        string
	Branch      string
	Markdown    string
	Description string
	Revisions   []domain.Revision
	Missing     bool
	Public      bool
	UpdatedAt   time.Time
}

type EditForm struct {
	Branch  string
	Slug    string
	Content string
	IsNew   bool
}

type BranchView struct {
	Name    string
	Public  bool
	Current bool
	Href    string
}

type IndexView struct {
	Title       string
	Description string
	Branch      string
	Catalog     bool
	Branches    []BranchView
	Canonical   string
	Indexable   bool
}

type PageScreen struct {
	PageView
	EditHref  string
	Canonical string
	Indexable bool
	Home      bool
	Branches  []BranchView
}

type BranchesPage struct {
	Branches []domain.Branch
	Error    string
	Notice   string
}

type SitemapEntry struct {
	Path    string
	Updated time.Time
}

type PageUseCase struct {
	pages    domain.PageRepository
	branches domain.BranchRepository
	content  domain.ContentRepository
}

func New(pages domain.PageRepository, branches domain.BranchRepository, content domain.ContentRepository) *PageUseCase {
	return &PageUseCase{
		pages:    pages,
		branches: branches,
		content:  content,
	}
}

func (p *PageUseCase) EnsureDefault(ctx context.Context) error {
	_, err := p.CreateBranch(ctx, domain.DefaultBranch, false)
	if err != nil && !errors.Is(err, domain.ErrBranchTaken) {
		return err
	}

	list, err := p.branches.ListBranches(ctx)
	if err != nil {
		return err
	}

	names := make([]string, 0, len(list))
	for _, branch := range list {
		names = append(names, branch.Name)
	}

	return p.content.RelocateLoose(ctx, domain.DefaultBranch, names)
}

func (p *PageUseCase) CreateBranch(ctx context.Context, raw string, public bool) (domain.Branch, error) {
	name, err := domain.NormalizeBranch(raw)
	if err != nil {
		return domain.Branch{}, err
	}

	if _, err := p.branches.FindBranch(ctx, name); err == nil {
		return domain.Branch{}, domain.ErrBranchTaken
	} else if !errors.Is(err, domain.ErrNotFound) {
		return domain.Branch{}, err
	}

	branch := domain.Branch{
		Name:      name,
		Public:    public,
		CreatedAt: time.Now(),
	}
	if err := p.branches.CreateBranch(ctx, branch); err != nil {
		return domain.Branch{}, err
	}

	return branch, nil
}

func (p *PageUseCase) SetBranchPublic(ctx context.Context, raw string, public bool) error {
	name, err := domain.NormalizeBranch(raw)
	if err != nil {
		return err
	}

	return p.branches.SetBranchPublic(ctx, name, public)
}

func (p *PageUseCase) DeleteBranch(ctx context.Context, raw string) error {
	name, err := domain.NormalizeBranch(raw)
	if err != nil {
		return err
	}

	if name == domain.DefaultBranch {
		return domain.ErrDefaultBranch
	}

	if _, err := p.branches.FindBranch(ctx, name); err != nil {
		return err
	}

	if err := p.content.RemoveBranch(ctx, name); err != nil {
		return err
	}

	if err := p.pages.DeleteBranchPages(ctx, name); err != nil {
		return err
	}

	return p.branches.DeleteBranch(ctx, name)
}

func (p *PageUseCase) OpenBranch(ctx context.Context, raw string) (domain.Branch, error) {
	name, err := domain.NormalizeBranch(raw)
	if err != nil {
		return domain.Branch{}, err
	}

	return p.branches.FindBranch(ctx, name)
}

func (p *PageUseCase) ListBranches(ctx context.Context) ([]domain.Branch, error) {
	return p.branches.ListBranches(ctx)
}

func (p *PageUseCase) VisibleBranches(ctx context.Context, authenticated bool) ([]domain.Branch, error) {
	all, err := p.branches.ListBranches(ctx)
	if err != nil || authenticated {
		return all, err
	}

	out := make([]domain.Branch, 0, len(all))
	for _, branch := range all {
		if branch.Public {
			out = append(out, branch)
		}
	}

	return out, nil
}

func BranchLinks(branches []domain.Branch, current string) []BranchView {
	out := make([]BranchView, 0, len(branches))
	for _, branch := range branches {
		out = append(out, BranchView{
			Name:    branch.Name,
			Public:  branch.Public,
			Current: branch.Name == current,
			Href:    domain.PagePath(branch.Name, ""),
		})
	}

	return out
}

func BranchDescription(branch domain.Branch) string {
	if branch.Name == domain.DefaultBranch {
		return "Публичные страницы вики"
	}

	return "Публичные страницы ветки " + branch.Name
}

func (p *PageUseCase) ListPages(ctx context.Context, branch string) ([]domain.Page, error) {
	if _, err := p.OpenBranch(ctx, branch); err != nil {
		return nil, err
	}

	return p.pages.List(ctx, branch)
}

func (p *PageUseCase) ViewPage(ctx context.Context, rawBranch, rawSlug string) (PageView, error) {
	branch, err := p.OpenBranch(ctx, rawBranch)
	if err != nil {
		return PageView{}, err
	}

	slug, err := domain.NormalizeSlug(rawSlug)
	if err != nil {
		return PageView{}, err
	}

	title := domain.TitleFromSlug(slug)
	data, err := p.content.Read(ctx, branch.Name, domain.MarkdownPath(slug))
	if errors.Is(err, domain.ErrNotFound) {
		return PageView{
			Title:   title,
			Slug:    slug,
			Branch:  branch.Name,
			Public:  branch.Public,
			Missing: true,
		}, nil
	}
	if err != nil {
		return PageView{}, err
	}

	revs, err := p.pages.Revisions(ctx, branch.Name, slug, 20)
	if err != nil {
		return PageView{}, err
	}

	view := PageView{
		Title:       title,
		Slug:        slug,
		Branch:      branch.Name,
		Markdown:    string(data),
		Description: describe(string(data)),
		Revisions:   revs,
		Public:      branch.Public,
	}
	if len(revs) > 0 {
		view.UpdatedAt = revs[0].CreatedAt
	}

	return view, nil
}

func (p *PageUseCase) EditForm(ctx context.Context, rawBranch, rawSlug string) (EditForm, error) {
	branch, err := p.OpenBranch(ctx, rawBranch)
	if err != nil {
		return EditForm{}, err
	}

	slug := strings.Trim(rawSlug, "/")
	if slug == "" {
		return EditForm{Branch: branch.Name, IsNew: true}, nil
	}

	slug, err = domain.NormalizeSlug(slug)
	if err != nil {
		return EditForm{}, err
	}

	data, err := p.content.Read(ctx, branch.Name, domain.MarkdownPath(slug))
	if errors.Is(err, domain.ErrNotFound) {
		return EditForm{Branch: branch.Name, Slug: slug, IsNew: true}, nil
	}
	if err != nil {
		return EditForm{}, err
	}

	return EditForm{
		Branch:  branch.Name,
		Slug:    slug,
		Content: string(data),
		IsNew:   false,
	}, nil
}

func (p *PageUseCase) SavePage(ctx context.Context, rawBranch, rawSlug, content string) (string, error) {
	branch, err := p.OpenBranch(ctx, rawBranch)
	if err != nil {
		return "", err
	}

	slug, err := domain.NormalizeSlug(rawSlug)
	if err != nil {
		return "", err
	}

	path := domain.MarkdownPath(slug)
	action := "правка"
	if _, err := p.content.Read(ctx, branch.Name, path); errors.Is(err, domain.ErrNotFound) {
		action = "создание"
	} else if err != nil {
		return "", err
	}

	message := action + ": " + branch.Name + "/" + slug
	if err := p.content.Write(ctx, branch.Name, path, []byte(content), message); err != nil {
		return "", err
	}

	if err := p.syncBranch(ctx, branch.Name); err != nil {
		return slug, fmt.Errorf("%w: %w", ErrIndexSync, err)
	}

	return slug, nil
}

func (p *PageUseCase) Sync(ctx context.Context) error {
	branches, err := p.branches.ListBranches(ctx)
	if err != nil {
		return err
	}

	for _, branch := range branches {
		if err := p.syncBranch(ctx, branch.Name); err != nil {
			return err
		}
	}

	return nil
}

func (p *PageUseCase) Sitemap(ctx context.Context) ([]SitemapEntry, error) {
	branches, err := p.branches.ListBranches(ctx)
	if err != nil {
		return nil, err
	}

	var out []SitemapEntry
	mainPublic := false
	hasPublic := false
	for _, branch := range branches {
		if branch.Name == domain.DefaultBranch {
			mainPublic = branch.Public
		}

		if branch.Public {
			hasPublic = true
		}
	}
	if hasPublic && !mainPublic {
		out = append(out, SitemapEntry{Path: "/"})
	}

	for _, branch := range branches {
		if !branch.Public {
			continue
		}

		out = append(out, SitemapEntry{Path: domain.PagePath(branch.Name, "")})
		pages, err := p.pages.List(ctx, branch.Name)
		if err != nil {
			return nil, err
		}

		for _, page := range pages {
			out = append(out, SitemapEntry{
				Path:    domain.PagePath(branch.Name, page.Slug),
				Updated: page.UpdatedAt,
			})
		}
	}

	return out, nil
}

func (p *PageUseCase) syncBranch(ctx context.Context, branch string) error {
	files, err := p.content.ListMarkdown(ctx, branch)
	if err != nil {
		return err
	}

	now := time.Now()
	slugs := make([]string, 0, len(files))
	for _, file := range files {
		slug := domain.SlugFromPath(file.Path)
		slugs = append(slugs, slug)
		if err := p.pages.Upsert(ctx, domain.Page{
			Branch:    branch,
			Slug:      slug,
			Title:     domain.TitleFromSlug(slug),
			Path:      branch + "/" + file.Path,
			Hash:      file.Hash,
			Size:      file.Size,
			UpdatedAt: now,
		}); err != nil {
			return err
		}

		if err := p.pages.AddRevisionIfMissing(ctx, domain.Revision{
			Branch:    branch,
			Slug:      slug,
			Hash:      file.Hash,
			Message:   "синхронизация индекса",
			Author:    "система",
			CreatedAt: now,
		}); err != nil {
			return err
		}
	}

	return p.pages.Prune(ctx, branch, slugs)
}

func describe(markdown string) string {
	var b strings.Builder
	for line := range strings.SplitSeq(markdown, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "```") {
			continue
		}

		line = strings.Trim(line, "#>*-_`")
		if line == "" {
			continue
		}

		if b.Len() > 0 {
			b.WriteByte(' ')
		}

		b.WriteString(line)
		if b.Len() > 240 {
			break
		}
	}

	text := strings.Join(strings.Fields(b.String()), " ")
	runes := []rune(text)
	if len(runes) > 160 {
		runes = runes[:160]
		text = strings.TrimRightFunc(string(runes), unicode.IsSpace)
	}

	return text
}
