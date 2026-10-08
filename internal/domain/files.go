package domain

import (
	"path"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxFileBytes   = MaxMediaBytes // 2 MiB
	MaxPathDepth   = 16
	MaxPathRunes   = 240
	GitKeepName    = ".gitkeep"
	FilesURLPrefix = "/files"
)

type EntryKind string

const (
	EntryFile EntryKind = "file"
	EntryDir  EntryKind = "dir"
)

type DirEntry struct {
	Name    string
	Path    string
	Kind    EntryKind
	Size    int64
	Hash    string
	Updated time.Time
}

func NormalizeRepoPath(raw string) (string, error) {
	raw = strings.Trim(strings.ReplaceAll(raw, `\`, "/"), "/")
	if raw == "" {
		return "", nil
	}

	if strings.Contains(raw, "..") || strings.HasPrefix(raw, ".git/") || raw == ".git" {
		return "", ErrInvalidPath
	}

	clean := path.Clean("/" + raw)
	clean = strings.TrimPrefix(clean, "/")
	if clean == "." || clean == "" {
		return "", nil
	}

	if strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "", ErrInvalidPath
	}

	parts := strings.Split(clean, "/")
	if len(parts) > MaxPathDepth {
		return "", ErrInvalidPath
	}

	if utf8.RuneCountInString(clean) > MaxPathRunes {
		return "", ErrInvalidPath
	}

	for _, part := range parts {
		if part == "" || part == "." || part == ".." || part == ".git" {
			return "", ErrInvalidPath
		}
	}

	return clean, nil
}

func PathDepth(rel string) int {
	rel = strings.Trim(rel, "/")
	if rel == "" {
		return 0
	}

	return strings.Count(rel, "/") + 1
}

func ParentPath(rel string) string {
	rel = strings.Trim(rel, "/")
	if rel == "" {
		return ""
	}

	dir := path.Dir(rel)
	if dir == "." {
		return ""
	}

	return dir
}

func BaseName(rel string) string {
	return path.Base(strings.Trim(rel, "/"))
}

func JoinRepoPath(parts ...string) (string, error) {
	var b strings.Builder
	for _, part := range parts {
		part = strings.Trim(strings.ReplaceAll(part, `\`, "/"), "/")
		if part == "" {
			continue
		}

		if b.Len() > 0 {
			b.WriteByte('/')
		}
		b.WriteString(part)
	}

	return NormalizeRepoPath(b.String())
}

func IsMarkdownPath(rel string) bool {
	return strings.HasSuffix(strings.ToLower(rel), ".md")
}

func IsHiddenRepoName(name string) bool {
	return name == ".git" || strings.HasPrefix(name, ".git")
}

func FileContentType(rel string) string {
	if ct := MediaContentType(rel); ct != "application/octet-stream" {
		return ct
	}

	switch strings.ToLower(path.Ext(rel)) {
	case ".md", ".markdown", ".txt", ".csv", ".log":
		return "text/plain; charset=utf-8"
	case ".json":
		return "application/json"
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "text/javascript; charset=utf-8"
	case ".xml":
		return "application/xml"
	case ".yaml", ".yml":
		return "text/yaml; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

func FilesBrowsePath(branch, rel string) string {
	rel = strings.Trim(rel, "/")
	if branch == "" || branch == DefaultBranch {
		if rel == "" {
			return FilesURLPrefix
		}

		return FilesURLPrefix + "/" + rel
	}

	base := "/b/" + branch + FilesURLPrefix
	if rel == "" {
		return base
	}

	return base + "/" + rel
}

func IsTextPreview(rel string) bool {
	switch strings.ToLower(path.Ext(rel)) {
	case ".md", ".markdown", ".txt", ".csv", ".log", ".json", ".yaml", ".yml", ".xml", ".css", ".js", ".html", ".htm", ".svg", ".go", ".py", ".rs", ".ts", ".tsx", ".sh", ".env", ".toml", ".ini", ".cfg", ".conf":
		return true
	default:
		return false
	}
}
