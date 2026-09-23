package domain

import "context"

type PageRepository interface {
	List(ctx context.Context, branch string) ([]Page, error)

	Revisions(ctx context.Context, branch, slug string, limit int) ([]Revision, error)

	Upsert(ctx context.Context, page Page) error

	AddRevisionIfMissing(ctx context.Context, rev Revision) error

	Prune(ctx context.Context, branch string, slugs []string) error

	DeleteBranchPages(ctx context.Context, branch string) error
}

type BranchRepository interface {
	CreateBranch(ctx context.Context, branch Branch) error

	FindBranch(ctx context.Context, name string) (Branch, error)

	ListBranches(ctx context.Context) ([]Branch, error)

	SetBranchPublic(ctx context.Context, name string, public bool) error

	DeleteBranch(ctx context.Context, name string) error
}

type ContentRepository interface {
	ListMarkdown(ctx context.Context, branch string) ([]ContentFile, error)

	Read(ctx context.Context, branch, path string) ([]byte, error)

	Write(ctx context.Context, branch, path string, content []byte, message string) error

	RemoveBranch(ctx context.Context, branch string) error

	RelocateLoose(ctx context.Context, dest string, branches []string) error
}
