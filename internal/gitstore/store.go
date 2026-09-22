package gitstore

import (
	"github.com/go-git/go-git/v6"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v6/plumbing/object"
)

type Store struct {
	repo *git.Repository
	root string
}

type File struct {
	Path string
	Hash string
	Size int64
}

func NewLocal(path string) (*Store, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}

	abs, _ := filepath.Abs(path)
	return &Store{
		repo: repo,
		root: abs,
	}, nil
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

func (s *Store) ListMarkdown() ([]File, error) {
	tree, err := s.headTree()
	if err != nil {
		return nil, err
	}

	var files []File
	err = tree.Files().ForEach(func(f *object.File) error {
		if strings.HasSuffix(f.Name, ".md") {
			files = append(files, File{
				Path: f.Name,
				Hash: f.Hash.String(),
				Size: f.Size,
			})
		}
		return nil
	})

	return files, err
}

func (s *Store) ReadFile(path string) ([]byte, error) {
	tree, err := s.headTree()
	if err != nil {
		return nil, err
	}

	f, err := tree.File(path)
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

func (s *Store) History(path string, limit int) ([]*object.Commit, error) {
	ref, err := s.repo.Head()
	if err != nil {
		return nil, err
	}

	iter, err := s.repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return nil, err
	}

	var out []*object.Commit
	err = iter.ForEach(func(c *object.Commit) error {
		tree, err := c.Tree()
		if err != nil {
			return nil
		}

		if _, err := tree.File(path); err == nil {
			out = append(out, c)
		}

		if limit > 0 && len(out) >= limit {
			return io.EOF
		}

		return nil
	})
	if err == io.EOF {
		err = nil
	}

	return out, err
}

func (s *Store) WriteFile(relPath string, content []byte, message string) error {
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
