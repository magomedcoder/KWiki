package git

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/magomedcoder/kwiki/internal/domain"
)

type Store struct {
	mu   sync.Mutex
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

func (s *Store) ListMarkdown(ctx context.Context, branch string) ([]domain.ContentFile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	prefix, err := branchPrefix(branch)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tree, err := s.headTree()
	if err != nil {
		return nil, err
	}

	var files []domain.ContentFile
	err = tree.Files().ForEach(func(f *object.File) error {
		if !strings.HasPrefix(f.Name, prefix) || !strings.HasSuffix(f.Name, ".md") {
			return nil
		}
		rel := strings.TrimPrefix(f.Name, prefix)
		if rel == "" || strings.Contains(rel, "..") {
			return nil
		}
		files = append(files, domain.ContentFile{
			Path: rel,
			Hash: f.Hash.String(),
			Size: f.Size,
		})
		return nil
	})
	return files, err
}

func (s *Store) Read(ctx context.Context, branch, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	full, err := joinBranch(branch, path)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tree, err := s.headTree()
	if err != nil {
		return nil, err
	}

	return readTreeFile(tree, full)
}

func (s *Store) Write(ctx context.Context, branch, relPath string, content []byte, message string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	full, err := joinBranch(branch, relPath)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	wt, err := s.repo.Worktree()
	if err != nil {
		return err
	}
	if err := s.writeFile(wt, full, content); err != nil {
		return err
	}

	return s.commit(wt, message)
}

func (s *Store) RemoveBranch(ctx context.Context, branch string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	prefix, err := branchPrefix(branch)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tree, err := s.headTree()
	if err != nil {
		return err
	}

	var paths []string
	err = tree.Files().ForEach(func(f *object.File) error {
		if strings.HasPrefix(f.Name, prefix) {
			paths = append(paths, f.Name)
		}
		return nil
	})
	if err != nil || len(paths) == 0 {
		return err
	}

	wt, err := s.repo.Worktree()
	if err != nil {
		return err
	}

	for _, path := range paths {
		if _, err := wt.Remove(path); err != nil {
			return err
		}
	}

	if err := os.RemoveAll(filepath.Join(s.root, branch)); err != nil {
		return err
	}

	return s.commit(wt, "удаление ветки: "+branch)
}

func (s *Store) RelocateLoose(ctx context.Context, dest string, branches []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := branchPrefix(dest); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	known := make(map[string]bool, len(branches))
	for _, branch := range branches {
		known[branch] = true
	}

	tree, err := s.headTree()
	if err != nil {
		return err
	}

	var loose []string
	err = tree.Files().ForEach(func(f *object.File) error {
		if !strings.HasSuffix(f.Name, ".md") {
			return nil
		}

		top, _, _ := strings.Cut(f.Name, "/")
		if known[top] {
			return nil
		}

		loose = append(loose, f.Name)
		return nil
	})
	if err != nil || len(loose) == 0 {
		return err
	}

	wt, err := s.repo.Worktree()
	if err != nil {
		return err
	}

	for _, path := range loose {
		data, err := readTreeFile(tree, path)
		if err != nil {
			return err
		}

		if err := s.writeFile(wt, dest+"/"+path, data); err != nil {
			return err
		}

		if _, err := wt.Remove(path); err != nil {
			return err
		}
	}

	return s.commit(wt, "перенос страниц в ветку "+dest)
}

func (s *Store) writeFile(wt *git.Worktree, rel string, content []byte) error {
	rel = filepath.ToSlash(rel)
	full := filepath.Join(s.root, filepath.FromSlash(rel))
	if !inside(s.root, full) {
		return domain.ErrInvalidSlug
	}

	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(full, content, 0o644); err != nil {
		return err
	}

	_, err := wt.Add(rel)
	return err
}

func (s *Store) commit(wt *git.Worktree, message string) error {
	_, err := wt.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "kwiki",
			Email: "kwiki@localhost",
			When:  time.Now(),
		},
	})
	return err
}

func readTreeFile(tree *object.Tree, path string) ([]byte, error) {
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

func branchPrefix(branch string) (string, error) {
	if branch == "" || strings.Contains(branch, "/") || strings.Contains(branch, "..") {
		return "", domain.ErrInvalidBranch
	}

	return branch + "/", nil
}

func joinBranch(branch, rel string) (string, error) {
	prefix, err := branchPrefix(branch)
	if err != nil {
		return "", err
	}
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "/")
	if rel == "" || strings.Contains(rel, "..") || strings.Contains(rel, `\`) {
		return "", domain.ErrInvalidSlug
	}
	return prefix + rel, nil
}

func inside(root, full string) bool {
	rel, err := filepath.Rel(root, full)
	if err != nil {
		return false
	}

	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
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
