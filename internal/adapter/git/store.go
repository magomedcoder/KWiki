package git

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
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

	if err := configureReceive(repo); err != nil {
		return nil, err
	}

	return &Store{repo: repo, root: abs}, nil
}

func configureReceive(repo *git.Repository) error {
	cfg, err := repo.Config()
	if err != nil {
		return err
	}

	changed := false
	if cfg.Raw.Section("http").Option("receivepack") != "true" {
		cfg.Raw.Section("http").SetOption("receivepack", "true")
		changed = true
	}

	if cfg.Raw.Section("receive").Option("denyCurrentBranch") != "updateInstead" {
		cfg.Raw.Section("receive").SetOption("denyCurrentBranch", "updateInstead")
		changed = true
	}

	if cfg.Raw.Section("receive").Option("denyNonFastForwards") != "true" {
		cfg.Raw.Section("receive").SetOption("denyNonFastForwards", "true")
		changed = true
	}

	if !changed {
		return nil
	}

	return repo.Storer.SetConfig(cfg)
}

func (s *Store) Root() string {
	return s.root
}

func (s *Store) Repository() *git.Repository {
	return s.repo
}

func (s *Store) ResetWorktree() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	wt, err := s.repo.Worktree()
	if err != nil {
		return err
	}

	return wt.Reset(&git.ResetOptions{Mode: git.HardReset})
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
			Name:  "система",
			Email: "kwiki@localhost",
			When:  time.Now(),
		},
	})
	return err
}

func (s *Store) ListMarkdown(ctx context.Context, branch string) ([]domain.ContentFile, error) {
	files, err := s.ListPrefix(ctx, branch, "")
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

func (s *Store) ListPrefix(ctx context.Context, branch, prefix string) ([]domain.ContentFile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	branchPref, err := branchPrefix(branch)
	if err != nil {
		return nil, err
	}

	prefix = strings.TrimPrefix(filepath.ToSlash(prefix), "/")
	if strings.Contains(prefix, "..") || strings.Contains(prefix, `\`) {
		return nil, domain.ErrInvalidSlug
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tree, err := s.headTree()
	if err != nil {
		return nil, err
	}

	var files []domain.ContentFile
	err = tree.Files().ForEach(func(f *object.File) error {
		if !strings.HasPrefix(f.Name, branchPref) {
			return nil
		}

		rel := strings.TrimPrefix(f.Name, branchPref)
		if rel == "" || strings.Contains(rel, "..") {
			return nil
		}

		if prefix != "" && !strings.HasPrefix(rel, prefix) {
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

func (s *Store) ListDir(ctx context.Context, branch, prefix string) ([]domain.DirEntry, error) {
	prefix, err := domain.NormalizeRepoPath(prefix)
	if err != nil {
		return nil, err
	}

	files, err := s.ListPrefix(ctx, branch, prefix)
	if err != nil {
		return nil, err
	}

	type agg struct {
		entry domain.DirEntry
		seen  bool
	}
	children := map[string]*agg{}
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
		if rel == "" {
			continue
		}

		name, _, hasRest := strings.Cut(rel, "/")
		if name == "" || domain.IsHiddenRepoName(name) {
			continue
		}

		childPath := name
		if prefix != "" {
			childPath = prefix + "/" + name
		}

		if hasRest {
			item, ok := children[name]
			if !ok {
				children[name] = &agg{entry: domain.DirEntry{
					Name: name,
					Path: childPath,
					Kind: domain.EntryDir,
				}}
				continue
			}
			if item.entry.Kind != domain.EntryDir {
				item.entry.Kind = domain.EntryDir
				item.entry.Size = 0
				item.entry.Hash = ""
			}
			continue
		}

		children[name] = &agg{entry: domain.DirEntry{
			Name: name,
			Path: childPath,
			Kind: domain.EntryFile,
			Size: file.Size,
			Hash: file.Hash,
		}, seen: true}
	}

	out := make([]domain.DirEntry, 0, len(children))
	for _, item := range children {
		out = append(out, item.entry)
	}
	sortDirEntries(out)
	return out, nil
}

func sortDirEntries(entries []domain.DirEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Kind != entries[j].Kind {
			return entries[i].Kind == domain.EntryDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
}

func (s *Store) Stat(ctx context.Context, branch, path string) (domain.DirEntry, error) {
	path, err := domain.NormalizeRepoPath(path)
	if err != nil || path == "" {
		if err != nil {
			return domain.DirEntry{}, err
		}
		return domain.DirEntry{
			Path: "",
			Kind: domain.EntryDir,
			Name: branch,
		}, nil
	}

	data, err := s.Read(ctx, branch, path)
	if err == nil {
		return domain.DirEntry{
			Name: domain.BaseName(path),
			Path: path,
			Kind: domain.EntryFile,
			Size: int64(len(data)),
		}, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return domain.DirEntry{}, err
	}

	children, err := s.ListDir(ctx, branch, path)
	if err != nil {
		return domain.DirEntry{}, err
	}

	if len(children) == 0 {
		prefixFiles, err := s.ListPrefix(ctx, branch, path+"/")
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

func (s *Store) Exists(ctx context.Context, branch, path string) (bool, error) {
	_, err := s.Read(ctx, branch, path)
	if errors.Is(err, domain.ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return true, nil
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
	return s.WriteBatch(ctx, branch, []domain.ContentChange{{
		Path: relPath,
		Data: content,
	}}, message)
}

func (s *Store) WriteBatch(ctx context.Context, branch string, changes []domain.ContentChange, message string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(changes) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	wt, err := s.repo.Worktree()
	if err != nil {
		return err
	}

	for _, change := range changes {
		full, err := joinBranch(branch, change.Path)
		if err != nil {
			return err
		}

		if change.Delete {
			if _, err := wt.Remove(full); err != nil {
				return err
			}

			_ = os.Remove(filepath.Join(s.root, filepath.FromSlash(full)))
			continue
		}

		if err := s.writeFile(wt, full, change.Data); err != nil {
			return err
		}
	}

	return s.commit(wt, message)
}

func (s *Store) DeletePath(ctx context.Context, branch, path, message string) error {
	path, err := domain.NormalizeRepoPath(path)
	if err != nil || path == "" {
		if err != nil {
			return err
		}
		return domain.ErrInvalidPath
	}

	entry, err := s.Stat(ctx, branch, path)
	if err != nil {
		return err
	}

	var changes []domain.ContentChange
	if entry.Kind == domain.EntryFile {
		changes = []domain.ContentChange{{Path: path, Delete: true}}
	} else {
		files, err := s.ListPrefix(ctx, branch, path+"/")
		if err != nil {
			return err
		}
		for _, file := range files {
			changes = append(changes, domain.ContentChange{Path: file.Path, Delete: true})
		}
		if len(changes) == 0 {
			return domain.ErrNotFound
		}
	}
	return s.WriteBatch(ctx, branch, changes, message)
}

func (s *Store) MovePath(ctx context.Context, branch, from, to, message string) error {
	from, err := domain.NormalizeRepoPath(from)
	if err != nil || from == "" {
		if err != nil {
			return err
		}
		return domain.ErrInvalidPath
	}
	to, err = domain.NormalizeRepoPath(to)
	if err != nil || to == "" {
		if err != nil {
			return err
		}
		return domain.ErrInvalidPath
	}
	if from == to {
		return nil
	}
	if strings.HasPrefix(to+"/", from+"/") {
		return domain.ErrInvalidPath
	}

	entry, err := s.Stat(ctx, branch, from)
	if err != nil {
		return err
	}

	if _, err := s.Stat(ctx, branch, to); err == nil {
		return domain.ErrPathExists
	} else if !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	var changes []domain.ContentChange
	if entry.Kind == domain.EntryFile {
		data, err := s.Read(ctx, branch, from)
		if err != nil {
			return err
		}
		changes = []domain.ContentChange{
			{Path: to, Data: data},
			{Path: from, Delete: true},
		}
	} else {
		files, err := s.ListPrefix(ctx, branch, from+"/")
		if err != nil {
			return err
		}
		if len(files) == 0 {
			return domain.ErrNotFound
		}
		for _, file := range files {
			data, err := s.Read(ctx, branch, file.Path)
			if err != nil {
				return err
			}
			dest := to + strings.TrimPrefix(file.Path, from)
			changes = append(changes,
				domain.ContentChange{Path: dest, Data: data},
				domain.ContentChange{Path: file.Path, Delete: true},
			)
		}
	}
	return s.WriteBatch(ctx, branch, changes, message)
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
			Name:  "система",
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
