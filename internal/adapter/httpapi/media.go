package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/magomedcoder/kwiki/internal/domain"
)

const mediaFormMemory = domain.MaxMediaBytes + 64<<10

func splitMediaPath(path string) (branch, rel string, ok bool) {
	after, ok0 := strings.CutPrefix(path, "/b/")
	if !ok0 {
		return "", "", false
	}

	parts := strings.Split(strings.Trim(after, "/"), "/")
	if len(parts) < 3 || parts[1] != domain.MediaDir {
		return "", "", false
	}

	branch = parts[0]
	rel = domain.MediaDir + "/" + strings.Join(parts[2:], "/")
	return branch, rel, true
}

func (h *Handler) serveMedia(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, h.t(r, "errors.method_not_allowed"), http.StatusMethodNotAllowed)
		return
	}

	branchName, rel, ok := splitMediaPath(r.URL.Path)
	if !ok {
		h.notFound(w, r)
		return
	}

	branch, err := h.wiki.OpenBranch(r.Context(), branchName)
	if errors.Is(err, domain.ErrInvalidBranch) || errors.Is(err, domain.ErrNotFound) {
		h.notFound(w, r)
		return
	}
	if err != nil {
		log.Printf("медиа ветка: %v", err)
		http.Error(w, h.t(r, "errors.open_file"), http.StatusInternalServerError)
		return
	}

	actor := actorFrom(r)
	if !branch.Public && actor.Email == "" {
		h.denyPrivate(w, r)
		return
	}

	draft := ""
	if actor.Email != "" {
		draft = h.sessionToken(r)
	}

	data, contentType, err := h.wiki.ReadMedia(r.Context(), draft, branch.Name, rel)
	if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrInvalidMediaPath) {
		h.notFound(w, r)
		return
	}
	if err != nil {
		log.Printf("медиа: %v", err)
		http.Error(w, h.t(r, "errors.open_file"), http.StatusInternalServerError)
		return
	}

	markIndexable(w, branch.Public)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if contentType == "image/svg+xml" {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
		w.Header().Set("Content-Disposition", "inline")
	}

	w.Header().Set("Cache-Control", "private, max-age=60")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(data)
	}
}

func (h *Handler) mediaAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listMedia(w, r)
	case http.MethodPost:
		h.uploadMedia(w, r)
	case http.MethodDelete:
		h.deleteMedia(w, r)
	default:
		http.Error(w, h.t(r, "errors.method_not_allowed"), http.StatusMethodNotAllowed)
	}
}

func (h *Handler) listMedia(w http.ResponseWriter, r *http.Request) {
	branch := requestedBranch(r)
	items, err := h.wiki.ListMedia(r.Context(), h.sessionToken(r), branch)
	if errors.Is(err, domain.ErrInvalidBranch) || errors.Is(err, domain.ErrNotFound) {
		http.Error(w, h.t(r, "errors.branch_not_found"), http.StatusNotFound)
		return
	}

	if err != nil {
		log.Printf("список медиа: %v", err)
		http.Error(w, h.t(r, "errors.load_media"), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) uploadMedia(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, h.t(r, "errors.file_missing"), http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, domain.MaxMediaBytes+1))
	if err != nil {
		http.Error(w, h.t(r, "errors.read_file"), http.StatusBadRequest)
		return
	}
	if len(data) > domain.MaxMediaBytes {
		http.Error(w, h.t(r, "errors.file_too_large"), http.StatusRequestEntityTooLarge)
		return
	}

	folder := strings.TrimSpace(r.FormValue("folder"))
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" && header != nil {
		name = header.Filename
	}
	overwrite := r.FormValue("overwrite") == "1" || r.FormValue("overwrite") == "true"

	item, err := h.wiki.StageMedia(r.Context(), h.sessionToken(r), requestedBranch(r), folder, name, data, overwrite)
	if errors.Is(err, domain.ErrMediaTooLarge) {
		http.Error(w, h.t(r, "errors.file_too_large"), http.StatusRequestEntityTooLarge)
		return
	}

	if errors.Is(err, domain.ErrMediaExists) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "exists"})
		return
	}

	if errors.Is(err, domain.ErrInvalidMediaPath) {
		http.Error(w, h.t(r, "errors.invalid_path"), http.StatusBadRequest)
		return
	}

	if errors.Is(err, domain.ErrInvalidBranch) || errors.Is(err, domain.ErrNotFound) {
		http.Error(w, h.t(r, "errors.branch_not_found"), http.StatusNotFound)
		return
	}

	if err != nil {
		log.Printf("загрузка медиа: %v", err)
		http.Error(w, h.t(r, "errors.upload_file"), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, item)
}

func (h *Handler) deleteMedia(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		path = strings.TrimSpace(r.FormValue("path"))
	}

	err := h.wiki.DeleteMedia(r.Context(), h.sessionToken(r), requestedBranch(r), path)
	if errors.Is(err, domain.ErrNotFound) {
		http.Error(w, h.t(r, "errors.file_not_found"), http.StatusNotFound)
		return
	}

	if errors.Is(err, domain.ErrInvalidMediaPath) {
		http.Error(w, h.t(r, "errors.invalid_path"), http.StatusBadRequest)
		return
	}

	if errors.Is(err, domain.ErrInvalidBranch) {
		http.Error(w, h.t(r, "errors.branch_not_found"), http.StatusNotFound)
		return
	}

	if err != nil {
		log.Printf("удаление медиа: %v", err)
		http.Error(w, h.t(r, "errors.delete_file"), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
