package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/magomedcoder/kwiki/internal/domain"
)

var ErrIndexSync = errors.New("синхронизация индекса")

type PageView struct {
	Title     string
	Slug      string
	Markdown  string
	Revisions []domain.Revision
	Missing   bool
}

type EditForm struct {
	Slug    string
	Content string
	IsNew   bool
}

type PageUseCase struct {
	pages   domain.PageRepository
	content domain.ContentRepository
}

func New(pages domain.PageRepository, content domain.ContentRepository) *PageUseCase {
	return &PageUseCase{
		pages:   pages,
		content: content,
	}
}

func (p *PageUseCase) ListPages(ctx context.Context) ([]domain.Page, error) {
	return p.pages.List(ctx)
}

func (p *PageUseCase) ViewPage(ctx context.Context, rawSlug string) (PageView, error) {
	slug, err := domain.NormalizeSlug(rawSlug)
	if err != nil {
		return PageView{}, err
	}

	title := domain.TitleFromSlug(slug)
	data, err := p.content.Read(ctx, domain.MarkdownPath(slug))
	if errors.Is(err, domain.ErrNotFound) {
		return PageView{
			Title:   title,
			Slug:    slug,
			Missing: true,
		}, nil
	}
	if err != nil {
		return PageView{}, err
	}

	revs, err := p.pages.Revisions(ctx, slug, 20)
	if err != nil {
		return PageView{}, err
	}

	return PageView{
		Title:     title,
		Slug:      slug,
		Markdown:  string(data),
		Revisions: revs,
	}, nil
}

func (p *PageUseCase) EditForm(ctx context.Context, rawSlug string) (EditForm, error) {
	slug := strings.Trim(rawSlug, "/")
	if slug == "" {
		return EditForm{IsNew: true}, nil
	}

	slug, err := domain.NormalizeSlug(slug)
	if err != nil {
		return EditForm{}, err
	}

	data, err := p.content.Read(ctx, domain.MarkdownPath(slug))
	if errors.Is(err, domain.ErrNotFound) {
		return EditForm{Slug: slug, IsNew: true}, nil
	}
	if err != nil {
		return EditForm{}, err
	}

	return EditForm{
		Slug:    slug,
		Content: string(data),
		IsNew:   false,
	}, nil
}

func (p *PageUseCase) SavePage(ctx context.Context, rawSlug, content string) (string, error) {
	slug, err := domain.NormalizeSlug(rawSlug)
	if err != nil {
		return "", err
	}

	path := domain.MarkdownPath(slug)
	action := "edit"
	if _, err := p.content.Read(ctx, path); errors.Is(err, domain.ErrNotFound) {
		action = "create"
	} else if err != nil {
		return "", err
	}

	if err := p.content.Write(ctx, path, []byte(content), action+": "+slug); err != nil {
		return "", err
	}

	if err := p.Sync(ctx); err != nil {
		return slug, fmt.Errorf("%w: %w", ErrIndexSync, err)
	}

	return slug, nil
}

func (p *PageUseCase) Sync(ctx context.Context) error {
	files, err := p.content.ListMarkdown(ctx)
	if err != nil {
		return err
	}

	now := time.Now()
	for _, f := range files {
		slug := domain.SlugFromPath(f.Path)
		if err := p.pages.Upsert(ctx, domain.Page{
			Slug:      slug,
			Title:     domain.TitleFromSlug(slug),
			Path:      f.Path,
			Hash:      f.Hash,
			Size:      f.Size,
			UpdatedAt: now,
		}); err != nil {
			return err
		}

		if err := p.pages.AddRevisionIfMissing(ctx, domain.Revision{
			Slug:      slug,
			Hash:      f.Hash,
			Message:   "синхронизация индекса",
			Author:    "система",
			CreatedAt: now,
		}); err != nil {
			return err
		}
	}

	return nil
}
