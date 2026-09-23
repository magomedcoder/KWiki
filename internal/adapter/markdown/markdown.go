package markdown

import (
	"html"
	"regexp"
	"strconv"
	"strings"
)

var (
	reHeading  = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)
	reListItem = regexp.MustCompile(`^[-*]\s+(.+)$`)
	reBold     = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	reItalic   = regexp.MustCompile(`\*([^*]+)\*`)
	reCode     = regexp.MustCompile("`([^`]+)`")
	reImage    = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)\)`)
	reLink     = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
)

type Heading struct {
	Level int
	ID    string
	Text  string
}

func HTML(src []byte, branch string) string {
	body, _ := Document(src, branch)
	return body
}

func Document(src []byte, branch string) (string, []Heading) {
	lines := strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")

	var out strings.Builder
	var para []string
	var headings []Heading
	used := map[string]int{}
	inCode := false
	inList := false

	flushPara := func() {
		if len(para) > 0 {
			out.WriteString("<p>")
			out.WriteString(strings.Join(para, " "))
			out.WriteString("</p>\n")
			para = para[:0]
		}
	}
	closeList := func() {
		if inList {
			out.WriteString("</ul>\n")
			inList = false
		}
	}

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			flushPara()
			closeList()
			if inCode {
				out.WriteString("</code></pre>\n")
				inCode = false
			} else {
				out.WriteString("<pre><code>")
				inCode = true
			}
			continue
		}
		if inCode {
			out.WriteString(html.EscapeString(line))
			out.WriteByte('\n')
			continue
		}

		if m := reHeading.FindStringSubmatch(line); m != nil {
			flushPara()
			closeList()
			level := len(m[1])
			text := plainText(m[2])
			id := anchorID(text, used)
			headings = append(headings, Heading{Level: level, ID: id, Text: text})
			out.WriteString("<h" + strconv.Itoa(level) + ` id="` + html.EscapeString(id) + `">`)
			out.WriteString(inline(m[2], branch))
			out.WriteString("</h" + strconv.Itoa(level) + ">\n")
			continue
		}

		if m := reListItem.FindStringSubmatch(line); m != nil {
			flushPara()
			if !inList {
				out.WriteString("<ul>\n")
				inList = true
			}
			out.WriteString("<li>")
			out.WriteString(inline(m[1], branch))
			out.WriteString("</li>\n")
			continue
		}

		if strings.TrimSpace(line) == "" {
			flushPara()
			closeList()
			continue
		}

		para = append(para, inline(strings.TrimSpace(line), branch))
	}

	flushPara()
	closeList()
	if inCode {
		out.WriteString("</code></pre>\n")
	}

	return out.String(), headings
}

var reSlug = regexp.MustCompile(`[^\p{L}\p{N}]+`)

func plainText(s string) string {
	s = reCode.ReplaceAllString(s, "$1")
	s = reBold.ReplaceAllString(s, "$1")
	s = reItalic.ReplaceAllString(s, "$1")
	s = reImage.ReplaceAllString(s, "$1")
	s = reLink.ReplaceAllString(s, "$1")

	return strings.TrimSpace(s)
}

func anchorID(text string, used map[string]int) string {
	id := strings.ToLower(strings.Trim(reSlug.ReplaceAllString(text, "-"), "-"))
	if id == "" {
		id = "section"
	}

	used[id]++
	if used[id] == 1 {
		return id
	}

	return id + "-" + strconv.Itoa(used[id])
}

func inline(s, branch string) string {
	var codes []string
	s = reCode.ReplaceAllStringFunc(s, func(m string) string {
		codes = append(codes, m[1:len(m)-1])
		return "\x00" + strconv.Itoa(len(codes)-1) + "\x00"
	})

	s = html.EscapeString(s)
	s = reBold.ReplaceAllString(s, "<strong>$1</strong>")
	s = reItalic.ReplaceAllString(s, "<em>$1</em>")
	s = reImage.ReplaceAllStringFunc(s, func(m string) string {
		parts := reImage.FindStringSubmatch(m)
		if len(parts) != 3 {
			return m
		}

		alt, href := parts[1], resolveMediaHref(parts[2], branch)
		lower := strings.ToLower(href)
		switch {
		case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"),
			strings.HasSuffix(lower, ".png"), strings.HasSuffix(lower, ".gif"),
			strings.HasSuffix(lower, ".webp"), strings.HasSuffix(lower, ".svg"),
			strings.HasSuffix(lower, ".avif"), strings.HasSuffix(lower, ".bmp"),
			strings.HasSuffix(lower, ".ico"):
			return `<img src="` + href + `" alt="` + alt + `">`
		default:
			if alt == "" {
				alt = href
			}

			return `<a href="` + href + `">` + alt + `</a>`
		}
	})
	s = reLink.ReplaceAllString(s, `<a href="$2">$1</a>`)

	for i, c := range codes {
		s = strings.Replace(s, "\x00"+strconv.Itoa(i)+"\x00", "<code>"+html.EscapeString(c)+"</code>", 1)
	}

	return s
}

func resolveMediaHref(href, branch string) string {
	if branch == "" || href == "" {
		return href
	}

	lower := strings.ToLower(href)
	if strings.HasPrefix(href, "/") || strings.HasPrefix(href, "#") || strings.Contains(href, "://") || strings.HasPrefix(lower, "mailto:") {
		return href
	}

	if strings.Contains(href, "..") {
		return href
	}

	rel := strings.Trim(href, "/")
	rel = strings.TrimPrefix(rel, "media/")
	if rel == "" {
		return href
	}

	return "/b/" + branch + "/media/" + rel
}
