package git

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/plumbing/storer"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/storage"
	"github.com/magomedcoder/kwiki/internal/domain"
)

type HTTPBackend struct {
	Store *Store
}

func (b *HTTPBackend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if b == nil || b.Store == nil {
		http.Error(w, "git unavailable", http.StatusServiceUnavailable)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/git")
	service := r.URL.Query().Get("service")
	proto := r.Header.Get("Git-Protocol")

	b.Store.mu.Lock()
	defer b.Store.mu.Unlock()

	st := &ffStorer{Storer: b.Store.repo.Storer}
	wc := nopCloser{Writer: w}
	body := r.Body
	if body == nil {
		body = io.NopCloser(strings.NewReader(""))
	}

	switch {
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/info/refs"):
		switch service {
		case "git-upload-pack":
			w.Header().Set("Content-Type", "application/x-git-upload-pack-advertisement")
			w.Header().Set("Cache-Control", "no-cache")
			w.WriteHeader(http.StatusOK)
			err := transport.UploadPack(r.Context(), st, nil, wc, &transport.UploadPackOptions{
				GitProtocol:   proto,
				AdvertiseRefs: true,
				StatelessRPC:  true,
			})
			if err != nil {
				return
			}
		case "git-receive-pack":
			w.Header().Set("Content-Type", "application/x-git-receive-pack-advertisement")
			w.Header().Set("Cache-Control", "no-cache")
			w.WriteHeader(http.StatusOK)
			err := transport.ReceivePack(r.Context(), st, nil, wc, &transport.ReceivePackOptions{
				GitProtocol:   proto,
				AdvertiseRefs: true,
				StatelessRPC:  true,
			})
			if err != nil {
				return
			}
		default:
			http.Error(w, "unsupported service", http.StatusForbidden)
		}

	case r.Method == http.MethodPost && strings.HasSuffix(path, "/git-upload-pack"):
		w.Header().Set("Content-Type", "application/x-git-upload-pack-result")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		_ = transport.UploadPack(r.Context(), st, body, wc, &transport.UploadPackOptions{
			GitProtocol:  proto,
			StatelessRPC: true,
		})

	case r.Method == http.MethodPost && strings.HasSuffix(path, "/git-receive-pack"):
		w.Header().Set("Content-Type", "application/x-git-receive-pack-result")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		_ = transport.ReceivePack(r.Context(), st, body, wc, &transport.ReceivePackOptions{
			GitProtocol:  proto,
			StatelessRPC: true,
		})

	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func (b *HTTPBackend) CloneURL(r *http.Request, secure bool) string {
	host := r.Host
	if host == "" {
		return "/git"
	}
	scheme := "http"
	if secure || r.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/git", scheme, host)
}

type nopCloser struct{ 
	io.Writer 
}

func (nopCloser) Close() error {
	 return nil 
	}

type ffStorer struct {
	storage.Storer
}

func (s *ffStorer) SetReference(ref *plumbing.Reference) error {
	if ref.Type() == plumbing.HashReference {
		old, err := s.Storer.Reference(ref.Name())
		if err == nil && old.Type() == plumbing.HashReference && old.Hash() != ref.Hash() {
			ok, ferr := isFastForward(s.Storer, old.Hash(), ref.Hash())
			if ferr != nil {
				return ferr
			}
			if !ok {
				return domain.ErrForcePush
			}
		} else if err != nil && err != plumbing.ErrReferenceNotFound {
			return err
		}
	}
	return s.Storer.SetReference(ref)
}

func isFastForward(s storer.EncodedObjectStorer, oldHash, newHash plumbing.Hash) (bool, error) {
	if oldHash == plumbing.ZeroHash || oldHash == newHash {
		return true, nil
	}
	newer, err := object.GetCommit(s, newHash)
	if err != nil {
		return false, err
	}
	found := false
	iter := object.NewCommitPreorderIter(newer, nil, nil)
	err = iter.ForEach(func(c *object.Commit) error {
		if c.Hash == oldHash {
			found = true
			return storer.ErrStop
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return found, nil
}
