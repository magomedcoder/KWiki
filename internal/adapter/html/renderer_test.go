package html

import "testing"

func TestMinifyKeepsMarkdownNewlines(t *testing.T) {
	in := "<textarea>\n# Hello\n\n**bold**\n</textarea>"
	out := minifyHTML(in)
	if out != in {
		t.Fatalf("out = %q", out)
	}
}
