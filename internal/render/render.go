package render

import (
	"bytes"
	"html/template"
	"log"
	"net/http"
	"regexp"
)

var (
	indexTmpl *template.Template
	pageTmpl  *template.Template
	editTmpl  *template.Template

	reTags   = regexp.MustCompile(`>\s+<`)
	reSpaces = regexp.MustCompile(`\s{2,}`)
)

func LoadTemplates(dir string) {
	indexTmpl = template.Must(template.ParseFiles(dir+"/layout.tmpl", dir+"/index.tmpl"))
	pageTmpl = template.Must(template.ParseFiles(dir+"/layout.tmpl", dir+"/page.tmpl"))
	editTmpl = template.Must(template.ParseFiles(dir+"/layout.tmpl", dir+"/edit.tmpl"))
}

func minifyHTML(s string) string {
	s = reTags.ReplaceAllString(s, "><")
	s = reSpaces.ReplaceAllString(s, " ")

	return s
}

func exec(w http.ResponseWriter, t *template.Template, data any) {
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout.tmpl", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("template error: %v", err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(minifyHTML(buf.String())))
}

func RenderIndex(w http.ResponseWriter, data any) {
	exec(w, indexTmpl, data)
}

func RenderPage(w http.ResponseWriter, data any) {
	exec(w, pageTmpl, data)
}

func RenderEdit(w http.ResponseWriter, data any) {
	exec(w, editTmpl, data)
}
