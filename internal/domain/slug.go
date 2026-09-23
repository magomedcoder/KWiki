package domain

import "strings"

func NormalizeSlug(raw string) (string, error) {
	slug := strings.Trim(raw, "/")
	if slug == "" || strings.Contains(slug, "..") || strings.Contains(slug, `\`) {
		return "", ErrInvalidSlug
	}

	return slug, nil
}

func MarkdownPath(slug string) string {
	if strings.HasSuffix(slug, ".md") {
		return slug
	}

	return slug + ".md"
}

func SlugFromPath(path string) string {
	slug := strings.TrimSuffix(path, ".md")
	return strings.TrimPrefix(slug, "./")
}

func TitleFromSlug(slug string) string {
	base := slug
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}

	return strings.ReplaceAll(base, "-", " ")
}
