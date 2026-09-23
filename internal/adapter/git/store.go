package git

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/magomedcoder/kwiki/internal/domain"
)

type Store struct {
	repo *git.Repository
	root string
}

var _ domain.ContentRepository = (*Store)(nil)

func NewLocal(path string) (*Store, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}

	repo, err := git.PlainOpen(abs)
	if errors.Is(err, git.ErrRepositoryNotExists) {
		repo, err = git.PlainInit(abs, false)
		if err != nil {
			return nil, err
		}
		if err := ensureInitialCommit(repo, abs); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	return &Store{repo: repo, root: abs}, nil
}

func ensureInitialCommit(repo *git.Repository, root string) error {
	if _, err := repo.Head(); err == nil {
		return nil
	} else if !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	keep := filepath.Join(root, ".gitkeep")
	if _, err := os.Stat(keep); os.IsNotExist(err) {
		if err := os.WriteFile(keep, []byte{}, 0o644); err != nil {
			return err
		}
		if _, err := wt.Add(".gitkeep"); err != nil {
			return err
		}
	}

	_, err = wt.Commit("начальный коммит", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "kwiki",
			Email: "kwiki@localhost",
			When:  time.Now(),
		},
	})
	return err
}

func (s *Store) ListMarkdown(ctx context.Context) ([]domain.ContentFile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	tree, err := s.headTree()
	if err != nil {
		return nil, err
	}

	var files []domain.ContentFile
	err = tree.Files().ForEach(func(f *object.File) error {
		if strings.HasSuffix(f.Name, ".md") {
			files = append(files, domain.ContentFile{
				Path: f.Name,
				Hash: f.Hash.String(),
				Size: f.Size,
			})
		}
		return nil
	})
	return files, err
}

func (s *Store) Read(ctx context.Context, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	tree, err := s.headTree()
	if err != nil {
		return nil, err
	}

	f, err := tree.File(path)
	if errors.Is(err, object.ErrFileNotFound) {
		return nil, domain.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	r, err := f.Reader()
	if err != nil {
		return nil, err
	}
	defer r.Close()

	return io.ReadAll(r)
}

func (s *Store) Write(ctx context.Context, relPath string, content []byte, message string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	wt, err := s.repo.Worktree()
	if err != nil {
		return err
	}

	full := filepath.Join(s.root, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(full, content, 0o644); err != nil {
		return err
	}

	if _, err := wt.Add(relPath); err != nil {
		return err
	}

	_, err = wt.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "kwiki",
			Email: "kwiki@localhost",
			When:  time.Now(),
		},
	})
	return err
}

func (s *Store) headTree() (*object.Tree, error) {
	ref, err := s.repo.Head()
	if err != nil {
		return nil, err
	}

	commit, err := s.repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, err
	}

	return commit.Tree()
}
