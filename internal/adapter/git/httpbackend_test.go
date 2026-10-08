package git

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestHTTPBackendClone(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	ctx := context.Background()
	store, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Write(ctx, "main", "README.md", []byte("# clone me\n"), "seed"); err != nil {
		t.Fatal(err)
	}

	backend := &HTTPBackend{Store: store}
	mux := http.NewServeMux()
	mux.Handle("/git", backend)
	mux.Handle("/git/", backend)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	dest := filepath.Join(t.TempDir(), "clone")
	cmd := exec.Command("git", "-c", "http.followRedirects=true", "clone", srv.URL+"/git", dest)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("clone: %v\n%s", err, out)
	}

	data, err := os.ReadFile(filepath.Join(dest, "main", "README.md"))
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != "# clone me\n" {
		t.Fatalf("content %q", data)
	}
}

func TestHTTPBackendPush(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	ctx := context.Background()
	store, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Write(ctx, "main", "README.md", []byte("# base\n"), "seed"); err != nil {
		t.Fatal(err)
	}

	backend := &HTTPBackend{Store: store}
	mux := http.NewServeMux()
	mux.Handle("/git", backend)
	mux.Handle("/git/", backend)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	work := filepath.Join(t.TempDir(), "work")
	clone := exec.Command("git", "clone", srv.URL+"/git", work)
	clone.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := clone.CombinedOutput(); err != nil {
		t.Fatalf("clone: %v\n%s", err, out)
	}

	target := filepath.Join(work, "main", "pushed.md")
	if err := os.WriteFile(target, []byte("from push\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = work
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("add", "-A")
	run("commit", "-m", "push test")
	run("push", "origin", "HEAD")

	if err := store.ResetWorktree(); err != nil {
		t.Fatal(err)
	}
	
	data, err := store.Read(ctx, "main", "pushed.md")
	if err != nil || string(data) != "from push\n" {
		t.Fatalf("after push: %q %v", data, err)
	}
}
