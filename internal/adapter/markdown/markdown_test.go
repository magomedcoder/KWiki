package markdown

import (
	"strings"
	"testing"
)

func TestDocumentHeadings(t *testing.T) {
	body, headings := Document([]byte("# Введение\n\nТекст\n\n## Установка\n\n## Установка\n\n```\n# не заголовок\n```\n"))
	if len(headings) != 3 {
		t.Fatalf("заголовки %+v", headings)
	}

	if headings[0].ID != "введение" || headings[1].ID != "установка" || headings[2].ID != "установка-2" {
		t.Fatalf("адреса %+v", headings)
	}

	if headings[2].Level != 2 {
		t.Fatalf("уровень %+v", headings[2])
	}

	for _, id := range []string{`id="введение"`, `id="установка"`, `id="установка-2"`} {
		if !strings.Contains(body, id) {
			t.Fatalf("html без %s: %s", id, body)
		}
	}

	if strings.Contains(body, "не заголовок</h") {
		t.Fatal("заголовок из кода попал в страницу")
	}
}
