package markdown

import (
	"strings"
	"testing"
)

func TestDocumentHeadings(t *testing.T) {
	body, headings := Document([]byte("# Введение\n\nТекст\n\n## Установка\n\n## Установка\n\n```\n# не заголовок\n```\n"), "")
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

func TestDocumentImages(t *testing.T) {
	body, _ := Document([]byte("Картинка ![схема](shots/a.png) и файл ![док](a.pdf) и корень ![иконка](icon.png)"), "docs")
	if !strings.Contains(body, `<img src="/b/docs/media/shots/a.png" alt="схема">`) {
		t.Fatalf("img: %s", body)
	}

	if !strings.Contains(body, `<img src="/b/docs/media/icon.png" alt="иконка">`) {
		t.Fatalf("root img: %s", body)
	}

	if !strings.Contains(body, `<a href="/b/docs/media/a.pdf">док</a>`) {
		t.Fatalf("pdf link: %s", body)
	}

	if strings.Contains(body, `<img src="/b/docs/media/a.pdf"`) {
		t.Fatal("pdf не должен быть img")
	}
}

func TestDocumentMediaSize(t *testing.T) {
	body, _ := Document([]byte("![схема](shots/a.png =400x300) и ![клип](clip.mp4 =640x) и ![ролик](demo.webm =x240)"), "docs")
	if !strings.Contains(body, `<img src="/b/docs/media/shots/a.png" alt="схема" width="400" height="300" style="max-width:100%;width:400px;height:300px">`) {
		t.Fatalf("img size: %s", body)
	}

	if !strings.Contains(body, `<video src="/b/docs/media/clip.mp4" controls playsinline width="640" style="max-width:100%;width:640px;height:auto"></video>`) {
		t.Fatalf("video width: %s", body)
	}

	if !strings.Contains(body, `<video src="/b/docs/media/demo.webm" controls playsinline height="240" style="max-width:100%;height:240px"></video>`) {
		t.Fatalf("video height: %s", body)
	}
}

func TestDocumentExtraSyntax(t *testing.T) {
	body, _ := Document([]byte("1. один\n2. два\n\n> цитата\n\n---\n\n~~нет~~ и __жирный__ и _курсив_\n\n| A | B |\n| --- | --- |\n| 1 | **x** |\n"), "")
	if !strings.Contains(body, "<ol>") || !strings.Contains(body, "<li>один</li>") {
		t.Fatalf("ol: %s", body)
	}

	if !strings.Contains(body, "<blockquote><p>цитата</p></blockquote>") {
		t.Fatalf("quote: %s", body)
	}

	if !strings.Contains(body, "<hr>") {
		t.Fatalf("hr: %s", body)
	}

	if !strings.Contains(body, "<del>нет</del>") || !strings.Contains(body, "<strong>жирный</strong>") || !strings.Contains(body, "<em>курсив</em>") {
		t.Fatalf("inline: %s", body)
	}

	if !strings.Contains(body, "<table>") || !strings.Contains(body, "<th>A</th>") || !strings.Contains(body, "<strong>x</strong>") {
		t.Fatalf("table: %s", body)
	}
}
