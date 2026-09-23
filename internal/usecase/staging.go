package usecase

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/magomedcoder/kwiki/internal/domain"
)

type mediaStaging struct {
	mu   sync.Mutex
	root string
	mem  map[string]map[string][]byte
}

func newMediaStaging(root string) *mediaStaging {
	return &mediaStaging{
		root: root,
		mem:  map[string]map[string][]byte{},
	}
}

func draftKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:16])
}

func (s *mediaStaging) put(draft, branch, path string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.root != "" {
		full, err := s.filePathLocked(draft, branch, path)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		return os.WriteFile(full, data, 0o644)
	}

	key := draft + "\x00" + branch
	bucket := s.mem[key]
	if bucket == nil {
		bucket = map[string][]byte{}
		s.mem[key] = bucket
	}

	bucket[path] = append([]byte(nil), data...)
	return nil
}

func (s *mediaStaging) get(draft, branch, path string) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getLocked(draft, branch, path)
}

func (s *mediaStaging) getLocked(draft, branch, path string) ([]byte, bool, error) {
	if s.root != "" {
		full, err := s.filePathLocked(draft, branch, path)
		if err != nil {
			return nil, false, err
		}

		data, err := os.ReadFile(full)
		if os.IsNotExist(err) {
			return nil, false, nil
		}

		if err != nil {
			return nil, false, err
		}

		return data, true, nil
	}

	bucket := s.mem[draft+"\x00"+branch]
	data, ok := bucket[path]
	if !ok {
		return nil, false, nil
	}

	return append([]byte(nil), data...), true, nil
}

func (s *mediaStaging) remove(draft, branch, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.root != "" {
		full, err := s.filePathLocked(draft, branch, path)
		if err != nil {
			return err
		}

		err = os.Remove(full)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	key := draft + "\x00" + branch
	if bucket := s.mem[key]; bucket != nil {
		delete(bucket, path)
	}

	return nil
}

func (s *mediaStaging) list(draft, branch string) ([]domain.ContentFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listLocked(draft, branch)
}

func (s *mediaStaging) listLocked(draft, branch string) ([]domain.ContentFile, error) {
	var out []domain.ContentFile
	if s.root != "" {
		base, err := s.branchDirLocked(draft, branch)
		if err != nil {
			return nil, err
		}

		_ = filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}

			rel, err := filepath.Rel(base, path)
			if err != nil {
				return nil
			}

			rel = filepath.ToSlash(rel)
			if !strings.HasPrefix(rel, domain.MediaDir+"/") {
				return nil
			}

			out = append(out, domain.ContentFile{
				Path: rel,
				Size: info.Size(),
			})
			return nil
		})

		return out, nil
	}

	bucket := s.mem[draft+"\x00"+branch]
	for path, data := range bucket {
		out = append(out, domain.ContentFile{Path: path, Size: int64(len(data))})
	}

	return out, nil
}

func (s *mediaStaging) consume(draft, branch string) ([]domain.ContentChange, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	files, err := s.listLocked(draft, branch)
	if err != nil {
		return nil, err
	}

	changes := make([]domain.ContentChange, 0, len(files))
	for _, file := range files {
		data, ok, err := s.getLocked(draft, branch, file.Path)
		if err != nil {
			return nil, err
		}

		if !ok {
			continue
		}
		changes = append(changes, domain.ContentChange{Path: file.Path, Data: data})
	}

	if s.root != "" {
		base, err := s.branchDirLocked(draft, branch)
		if err != nil {
			return nil, err
		}

		_ = os.RemoveAll(base)
	} else {
		delete(s.mem, draft+"\x00"+branch)
	}

	return changes, nil
}

func (s *mediaStaging) filePathLocked(draft, branch, path string) (string, error) {
	base, err := s.branchDirLocked(draft, branch)
	if err != nil {
		return "", err
	}

	full := filepath.Join(base, filepath.FromSlash(path))
	if !insidePath(base, full) {
		return "", domain.ErrInvalidMediaPath
	}

	return full, nil
}

func (s *mediaStaging) branchDirLocked(draft, branch string) (string, error) {
	if draft == "" || branch == "" {
		return "", domain.ErrInvalidMediaPath
	}

	base := filepath.Join(s.root, draftKey(draft), branch)
	if !insidePath(s.root, base) {
		return "", domain.ErrInvalidMediaPath
	}

	return base, nil
}

func insidePath(root, full string) bool {
	rel, err := filepath.Rel(root, full)
	if err != nil {
		return false
	}

	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
