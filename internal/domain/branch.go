package domain

import (
	"net/url"
	"strings"
	"time"
)

const DefaultBranch = "main"

type Branch struct {
	Name      string
	Public    bool
	CreatedAt time.Time
}

func NormalizeBranch(raw string) (string, error) {
	name := strings.ToLower(strings.TrimSpace(raw))
	if name == "" || len(name) > 32 {
		return "", ErrInvalidBranch
	}

	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9' && i > 0:
		case c == '-' && i > 0 && i < len(name)-1 && name[i-1] != '-':
		default:
			return "", ErrInvalidBranch
		}
	}

	return name, nil
}

func PagePath(branch, slug string) string {
	slug = strings.Trim(slug, "/")
	if branch == "" || branch == DefaultBranch {
		if slug == "" {
			return "/"
		}
		return "/" + slug
	}

	if slug == "" {
		return "/b/" + branch
	}

	return "/b/" + branch + "/" + slug
}

func HistoryPath(branch, slug string) string {
	return queryPath("/history", branch, slug)
}

func EditPath(branch, slug string) string {
	return queryPath("/edit", branch, slug)
}

func queryPath(base, branch, slug string) string {
	values := url.Values{}
	if branch != "" && branch != DefaultBranch {
		values.Set("branch", branch)
	}

	if slug != "" {
		values.Set("slug", slug)
	}

	if len(values) == 0 {
		return base
	}

	return base + "?" + values.Encode()
}
