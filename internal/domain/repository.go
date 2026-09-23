package domain

import "context"

type PageRepository interface {
	List(ctx context.Context) ([]Page, error)

	Revisions(ctx context.Context, slug string, limit int) ([]Revision, error)

	Upsert(ctx context.Context, page Page) error

	AddRevisionIfMissing(ctx context.Context, rev Revision) error
}

type ContentRepository interface {
	ListMarkdown(ctx context.Context) ([]ContentFile, error)

	Read(ctx context.Context, path string) ([]byte, error)

	Write(ctx context.Context, path string, content []byte, message string) error
}
