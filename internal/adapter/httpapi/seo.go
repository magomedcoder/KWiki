package httpapi

import (
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/magomedcoder/kwiki/internal/domain"
)

func splitWikiPath(path string) (branch, slug string, ok bool) {
	if path == "/" {
		return domain.DefaultBranch, "", true
	}

	if path == "/b" || path == "/b/" {
		return "", "", false
	}

	if strings.HasPrefix(path, "/b/") {
		rest := strings.Trim(strings.TrimPrefix(path, "/b/"), "/")
		if rest == "" {
			return "", "", false
		}

		name, page, _ := strings.Cut(rest, "/")
		if name == "" {
			return "", "", false
		}

		return name, page, true
	}

	return domain.DefaultBranch, strings.Trim(path, "/"), true
}

func markIndexable(w http.ResponseWriter, indexable bool) {
	if indexable {
		w.Header().Set("X-Robots-Tag", "index, follow, max-image-preview:large, max-snippet:-1, max-video-preview:-1")
		return
	}

	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
}

func (h *Handler) absolute(r *http.Request, path string) string {
	host := r.Host
	if host == "" || strings.ContainsAny(host, " \r\n\\/") {
		return ""
	}

	scheme := "http"
	if h.secure || r.TLS != nil {
		scheme = "https"
	}

	return scheme + "://" + host + path
}

func (h *Handler) robots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("X-Robots-Tag", "noindex")
	body := "User-agent: *\nAllow: /\nDisallow: /login\nDisallow: /logout\nDisallow: /edit\nDisallow: /users\nDisallow: /branches\n"
	if origin := h.absolute(r, ""); origin != "" {
		body += "\nSitemap: " + origin + "/sitemap.xml\n"
	}
	_, _ = fmt.Fprint(w, body)
}

func (h *Handler) sitemap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	origin := h.absolute(r, "")
	if origin == "" {
		http.Error(w, "некорректный запрос", http.StatusBadRequest)
		return
	}

	entries, err := h.wiki.Sitemap(r.Context())
	if err != nil {
		log.Printf("карта сайта: %v", err)
		http.Error(w, "не удалось построить карту сайта", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("X-Robots-Tag", "noindex")

	var b strings.Builder
	b.WriteString(xml.Header)
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`)
	for _, entry := range entries {
		b.WriteString("<url><loc>")
		xmlEscape(&b, origin+entry.Path)
		b.WriteString("</loc>")
		if !entry.Updated.IsZero() {
			b.WriteString("<lastmod>")
			b.WriteString(entry.Updated.UTC().Format(time.DateOnly))
			b.WriteString("</lastmod>")
		}
		b.WriteString("</url>")
	}
	b.WriteString("</urlset>")
	_, _ = w.Write([]byte(b.String()))
}

func xmlEscape(b *strings.Builder, value string) {
	_ = xml.EscapeText(b, []byte(value))
}
