package markdown

import (
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/magomedcoder/kwiki/internal/domain"
)

var (
	reHeading  = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)
	reListItem = regexp.MustCompile(`^[-*]\s+(.+)$`)
	reOrdered  = regexp.MustCompile(`^\d{1,9}\.\s+(.+)$`)
	reQuote    = regexp.MustCompile(`^>\s?(.*)$`)
	reHR       = regexp.MustCompile(`^ {0,3}(?:-{3,}|\*{3,}|_{3,})\s*$`)
	reBold     = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	reBoldU    = regexp.MustCompile(`__([^_]+)__`)
	reItalic   = regexp.MustCompile(`\*([^*]+)\*`)
	reItalicU  = regexp.MustCompile(`_([^_]+)_`)
	reStrike   = regexp.MustCompile(`~~([^~]+)~~`)
	reCode     = regexp.MustCompile("`([^`]+)`")
	reImage    = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)(?:\s+=([0-9]*)x([0-9]*))?\)`)
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
	var quote []string
	var headings []Heading
	used := map[string]int{}
	inCode := false
	listTag := ""

	flushPara := func() {
		if len(para) > 0 {
			out.WriteString("<p>")
			out.WriteString(strings.Join(para, " "))
			out.WriteString("</p>\n")
			para = para[:0]
		}
	}
	closeList := func() {
		if listTag != "" {
			out.WriteString("</" + listTag + ">\n")
			listTag = ""
		}
	}
	openList := func(tag string) {
		if listTag != tag {
			closeList()
			out.WriteString("<" + tag + ">\n")
			listTag = tag
		}
	}
	flushQuote := func() {
		if len(quote) == 0 {
			return
		}
		out.WriteString("<blockquote><p>")
		out.WriteString(strings.Join(quote, " "))
		out.WriteString("</p></blockquote>\n")
		quote = quote[:0]
	}
	writeItem := func(tag, text string) {
		flushPara()
		flushQuote()
		openList(tag)
		out.WriteString("<li>")
		out.WriteString(inline(text, branch))
		out.WriteString("</li>\n")
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			flushPara()
			flushQuote()
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

		if i+1 < len(lines) && isTableRow(line) && isTableSep(lines[i+1]) {
			flushPara()
			flushQuote()
			closeList()
			writeTable(&out, lines[i], branch)
			i++
			for i+1 < len(lines) && isTableRow(lines[i+1]) && !isTableSep(lines[i+1]) {
				i++
				writeTableRow(&out, lines[i], "td", branch)
			}

			out.WriteString("</tbody></table>\n")
			continue
		}

		if reHR.MatchString(line) && !reListItem.MatchString(line) {
			flushPara()
			flushQuote()
			closeList()
			out.WriteString("<hr>\n")
			continue
		}

		if m := reHeading.FindStringSubmatch(line); m != nil {
			flushPara()
			flushQuote()
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
			writeItem("ul", m[1])
			continue
		}

		if m := reOrdered.FindStringSubmatch(line); m != nil {
			writeItem("ol", m[1])
			continue
		}

		if m := reQuote.FindStringSubmatch(line); m != nil {
			flushPara()
			closeList()
			quote = append(quote, inline(strings.TrimSpace(m[1]), branch))
			continue
		}

		if strings.TrimSpace(line) == "" {
			flushPara()
			flushQuote()
			closeList()
			continue
		}

		flushQuote()
		closeList()
		para = append(para, inline(strings.TrimSpace(line), branch))
	}

	flushPara()
	flushQuote()
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
	s = reBoldU.ReplaceAllString(s, "$1")
	s = reStrike.ReplaceAllString(s, "$1")
	s = reItalic.ReplaceAllString(s, "$1")
	s = reItalicU.ReplaceAllString(s, "$1")
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
	s = reBoldU.ReplaceAllString(s, "<strong>$1</strong>")
	s = reStrike.ReplaceAllString(s, "<del>$1</del>")
	s = reItalic.ReplaceAllString(s, "<em>$1</em>")
	s = reItalicU.ReplaceAllString(s, "<em>$1</em>")
	s = reImage.ReplaceAllStringFunc(s, func(m string) string {
		parts := reImage.FindStringSubmatch(m)
		if len(parts) < 3 {
			return m
		}

		alt, href := parts[1], resolveMediaHref(parts[2], branch)
		width, height := "", ""
		if len(parts) > 4 {
			width, height = parts[3], parts[4]
		}
		return mediaHTML(alt, href, width, height)
	})
	s = reLink.ReplaceAllString(s, `<a href="$2">$1</a>`)

	for i, c := range codes {
		s = strings.Replace(s, "\x00"+strconv.Itoa(i)+"\x00", "<code>"+html.EscapeString(c)+"</code>", 1)
	}

	return s
}

const maxMediaDim = 4096

func mediaHTML(alt, href, width, height string) string {
	w, h := parseMediaDim(width), parseMediaDim(height)
	size := mediaSizeAttrs(w, h)
	switch {
	case domain.IsImageMedia(href):
		return `<img src="` + href + `" alt="` + alt + `"` + size + `>`
	case domain.IsVideoMedia(href):
		return `<video src="` + href + `" controls playsinline` + size + `></video>`
	default:
		if alt == "" {
			alt = href
		}
		return `<a href="` + href + `">` + alt + `</a>`
	}
}

func parseMediaDim(raw string) int {
	if raw == "" {
		return 0
	}

	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0
	}

	if n > maxMediaDim {
		return maxMediaDim
	}

	return n
}

func mediaSizeAttrs(width, height int) string {
	if width < 1 && height < 1 {
		return ""
	}

	var b strings.Builder
	var css []string
	css = append(css, "max-width:100%")
	if width > 0 {
		b.WriteString(` width="`)
		b.WriteString(strconv.Itoa(width))
		b.WriteByte('"')
		css = append(css, "width:"+strconv.Itoa(width)+"px")
	}

	if height > 0 {
		b.WriteString(` height="`)
		b.WriteString(strconv.Itoa(height))
		b.WriteByte('"')
		css = append(css, "height:"+strconv.Itoa(height)+"px")
	} else {
		css = append(css, "height:auto")
	}

	b.WriteString(` style="`)
	b.WriteString(strings.Join(css, ";"))
	b.WriteByte('"')

	return b.String()
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

func splitTableRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}

	return parts
}

func isTableRow(line string) bool {
	line = strings.TrimSpace(line)
	return strings.Contains(line, "|") && !strings.HasPrefix(line, "```")
}

func isTableSep(line string) bool {
	if !isTableRow(line) {
		return false
	}

	for _, cell := range splitTableRow(line) {
		cell = strings.Trim(cell, " :")
		if cell == "" || strings.Trim(cell, "-") != "" {
			return false
		}
	}

	return true
}

func writeTable(out *strings.Builder, header, branch string) {
	out.WriteString("<table><thead>")
	writeTableRow(out, header, "th", branch)
	out.WriteString("</thead><tbody>")
}

func writeTableRow(out *strings.Builder, line, tag, branch string) {
	out.WriteString("<tr>")
	for _, cell := range splitTableRow(line) {
		out.WriteString("<" + tag + ">")
		out.WriteString(inline(cell, branch))
		out.WriteString("</" + tag + ">")
	}

	out.WriteString("</tr>")
}
