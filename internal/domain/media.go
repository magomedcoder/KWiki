package domain

import (
	"path"
	"strings"
)

const (
	MediaDir       = "media"
	MaxMediaBytes  = 2 << 20 // 2 MiB
	MediaURLPrefix = "/b/"
)

type ContentChange struct {
	Path   string
	Data   []byte
	Delete bool
}

func MediaURL(branch, rel string) string {
	rel = strings.TrimPrefix(rel, "/")
	return MediaURLPrefix + branch + "/" + rel
}

func MediaRef(rel string) string {
	rel = strings.TrimPrefix(strings.TrimPrefix(rel, "/"), MediaDir+"/")
	return strings.Trim(rel, "/")
}

func NormalizeMediaRel(folder, name string) (string, error) {
	folder = strings.Trim(strings.ReplaceAll(folder, `\`, "/"), "/")
	name = strings.Trim(strings.ReplaceAll(name, `\`, "/"), "/")
	if name == "" || strings.Contains(name, "/") {
		return "", ErrInvalidMediaPath
	}

	rel := name
	if folder != "" {
		rel = folder + "/" + name
	}

	return NormalizeMediaPath(MediaDir + "/" + rel)
}

func NormalizeMediaPath(raw string) (string, error) {
	raw = strings.Trim(strings.ReplaceAll(raw, `\`, "/"), "/")
	if raw == "" || strings.Contains(raw, "..") {
		return "", ErrInvalidMediaPath
	}

	clean := path.Clean("/" + raw)
	clean = strings.TrimPrefix(clean, "/")
	if clean == "." || clean == "" || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "", ErrInvalidMediaPath
	}

	if !strings.HasPrefix(clean, MediaDir+"/") && clean != MediaDir {
		return "", ErrInvalidMediaPath
	}

	if clean == MediaDir {
		return "", ErrInvalidMediaPath
	}

	base := path.Base(clean)
	if base == "" || base == "." || base == ".." {
		return "", ErrInvalidMediaPath
	}

	return clean, nil
}

func IsImageMedia(rel string) bool {
	switch strings.ToLower(path.Ext(rel)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".avif", ".bmp", ".ico":
		return true
	default:
		return false
	}
}

func MediaContentType(rel string) string {
	switch strings.ToLower(path.Ext(rel)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".avif":
		return "image/avif"
	case ".bmp":
		return "image/bmp"
	case ".ico":
		return "image/x-icon"
	case ".pdf":
		return "application/pdf"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	default:
		return "application/octet-stream"
	}
}
