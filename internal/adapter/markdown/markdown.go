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
	reLink     = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
)

func HTML(src []byte) string {
	lines := strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")

	var out strings.Builder
	var para []string
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
			tag := "h" + strconv.Itoa(level)
			out.WriteString("<" + tag + ">")
			out.WriteString(inline(m[2]))
			out.WriteString("</" + tag + ">\n")
			continue
		}

		if m := reListItem.FindStringSubmatch(line); m != nil {
			flushPara()
			if !inList {
				out.WriteString("<ul>\n")
				inList = true
			}
			out.WriteString("<li>")
			out.WriteString(inline(m[1]))
			out.WriteString("</li>\n")
			continue
		}

		if strings.TrimSpace(line) == "" {
			flushPara()
			closeList()
			continue
		}

		para = append(para, inline(strings.TrimSpace(line)))
	}

	flushPara()
	closeList()
	if inCode {
		out.WriteString("</code></pre>\n")
	}

	return out.String()
}

func inline(s string) string {
	var codes []string
	s = reCode.ReplaceAllStringFunc(s, func(m string) string {
		codes = append(codes, m[1:len(m)-1])
		return "\x00" + strconv.Itoa(len(codes)-1) + "\x00"
	})

	s = html.EscapeString(s)
	s = reBold.ReplaceAllString(s, "<strong>$1</strong>")
	s = reItalic.ReplaceAllString(s, "<em>$1</em>")
	s = reLink.ReplaceAllString(s, `<a href="$2">$1</a>`)

	for i, c := range codes {
		s = strings.Replace(s, "\x00"+strconv.Itoa(i)+"\x00", "<code>"+html.EscapeString(c)+"</code>", 1)
	}

	return s
}
