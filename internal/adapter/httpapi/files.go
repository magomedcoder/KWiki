package httpapi

import (
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/magomedcoder/kwiki/internal/domain"
	"github.com/magomedcoder/kwiki/internal/usecase"
)

const fileFormMemory = domain.MaxFileBytes + 64<<10

func splitFilesPath(path string) (branch, rel string, ok bool) {
	path = strings.TrimSuffix(path, "/")
	if path == "/files" {
		return domain.DefaultBranch, "", true
	}

	if after, ok0 := strings.CutPrefix(path, "/files/"); ok0 {
		rel, err := domain.NormalizeRepoPath(after)
		if err != nil {
			return "", "", false
		}

		return domain.DefaultBranch, rel, true
	}

	if after, ok0 := strings.CutPrefix(path, "/b/"); ok0 {
		name, rest, found := strings.Cut(strings.Trim(after, "/"), "/")
		if !found {
			return "", "", false
		}

		if rest == "files" {
			return name, "", true
		}

		if afterFiles, ok1 := strings.CutPrefix(rest, "files/"); ok1 {
			rel, err := domain.NormalizeRepoPath(afterFiles)
			if err != nil {
				return "", "", false
			}

			return name, rel, true
		}
	}

	return "", "", false
}

func (h *Handler) filesPage(w http.ResponseWriter, r *http.Request) {
	branchName, rel, ok := splitFilesPath(r.URL.Path)
	if !ok {
		h.notFound(w, r)
		return
	}

	actor := actorFrom(r)
	if actor.Email == "" {
		h.denyPrivate(w, r)
		return
	}

	branch, err := h.wiki.OpenBranch(r.Context(), branchName)
	if errors.Is(err, domain.ErrInvalidBranch) || errors.Is(err, domain.ErrNotFound) {
		h.notFound(w, r)
		return
	}

	if err != nil {
		log.Printf("файлы ветка: %v", err)
		http.Error(w, h.t(r, "errors.open_branch"), http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case http.MethodGet, http.MethodHead:
		if r.URL.Query().Get("raw") == "1" {
			h.serveFileRaw(w, r, branch.Name, rel)
			return
		}
		h.showFiles(w, r, branch.Name, rel, actor)
	case http.MethodPost:
		if !h.ensureMutation(w, r, actor) {
			return
		}
		h.mutateFiles(w, r, branch.Name, rel, actor)
	case http.MethodDelete:
		if !h.ensureMutation(w, r, actor) {
			return
		}
		h.deleteFiles(w, r, branch.Name, rel, actor)
	default:
		http.Error(w, h.t(r, "errors.method_not_allowed"), http.StatusMethodNotAllowed)
	}
}

func (h *Handler) ensureMutation(w http.ResponseWriter, r *http.Request, actor usecase.Actor) bool {
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(fileFormMemory); err != nil {
			http.Error(w, h.t(r, "errors.bad_request"), http.StatusBadRequest)
			return false
		}
	} else if r.Form == nil {
		if err := r.ParseForm(); err != nil {
			http.Error(w, h.t(r, "errors.bad_request"), http.StatusBadRequest)
			return false
		}
	}
	if !usecase.TokenEqual(r.FormValue("csrf"), actor.CSRF) {
		http.Error(w, h.t(r, "errors.bad_request"), http.StatusForbidden)
		return false
	}
	return true
}

func (h *Handler) showFiles(w http.ResponseWriter, r *http.Request, branch, rel string, actor usecase.Actor) {
	view, err := h.wiki.BrowseFiles(r.Context(), branch, rel, actor.Email != "")
	if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrInvalidPath) {
		h.notFound(w, r)
		return
	}

	if err != nil {
		log.Printf("файлы: %v", err)
		http.Error(w, h.t(r, "errors.open_file"), http.StatusInternalServerError)
		return
	}

	clone := ""
	if h.git != nil {
		clone = h.git.CloneURL(r, h.secure)
	}
	view.CloneURL = clone
	view.PushHint = clone
	view.Branches = h.fileBranches(r, branch)

	markIndexable(w, false)
	h.view.Files(w, view, actor, langFrom(r))
}

func (h *Handler) fileBranches(r *http.Request, current string) []usecase.FileBranch {
	list, err := h.wiki.VisibleBranches(r.Context(), true)
	if err != nil {
		return []usecase.FileBranch{{
			Name:    current,
			Href:    domain.FilesBrowsePath(current, ""),
			Current: true,
		}}
	}

	out := make([]usecase.FileBranch, 0, len(list))
	for _, b := range list {
		out = append(out, usecase.FileBranch{
			Name:    b.Name,
			Href:    domain.FilesBrowsePath(b.Name, ""),
			Current: b.Name == current,
			Public:  b.Public,
		})
	}
	
	return out
}

func (h *Handler) serveFileRaw(w http.ResponseWriter, r *http.Request, branch, rel string) {
	data, contentType, err := h.wiki.ReadFileBytes(r.Context(), branch, rel)
	if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrInvalidPath) || errors.Is(err, domain.ErrIsDirectory) {
		h.notFound(w, r)
		return
	}

	if err != nil {
		log.Printf("raw файл: %v", err)
		http.Error(w, h.t(r, "errors.open_file"), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", "inline; filename=\""+domain.BaseName(rel)+"\"")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(data)
	}
}

func (h *Handler) mutateFiles(w http.ResponseWriter, r *http.Request, branch, rel string, actor usecase.Actor) {
	action := r.FormValue("action")
	if action == "" {
		action = "create"
	}

	switch action {
	case "create":
		name := strings.TrimSpace(r.FormValue("name"))
		asDir := r.FormValue("dir") == "1" || r.FormValue("dir") == "true"
		content := []byte(r.FormValue("content"))
		target, err := domain.JoinRepoPath(rel, name)
		if err != nil {
			http.Error(w, h.t(r, "errors.invalid_path"), http.StatusBadRequest)
			return
		}

		err = h.wiki.CreateFile(r.Context(), branch, target, content, asDir, actor)
		h.redirectFilesErr(w, r, branch, rel, err)
	case "upload":
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, h.t(r, "errors.file_missing"), http.StatusBadRequest)
			return
		}
		defer file.Close()

		data, err := io.ReadAll(io.LimitReader(file, domain.MaxFileBytes+1))
		if err != nil {
			http.Error(w, h.t(r, "errors.read_file"), http.StatusBadRequest)
			return
		}

		if len(data) > domain.MaxFileBytes {
			http.Error(w, h.t(r, "errors.file_too_large"), http.StatusRequestEntityTooLarge)
			return
		}

		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" && header != nil {
			name = header.Filename
		}

		folder := rel
		if folder == "" {
			folder = strings.TrimSpace(r.FormValue("folder"))
		}

		overwrite := r.FormValue("overwrite") == "1" || r.FormValue("overwrite") == "true"
		_, err = h.wiki.UploadFile(r.Context(), branch, folder, name, data, overwrite, actor)
		h.redirectFilesErr(w, r, branch, folder, err)
	case "move":
		from := strings.TrimSpace(r.FormValue("from"))
		to := strings.TrimSpace(r.FormValue("to"))
		err := h.wiki.MoveFilePath(r.Context(), branch, from, to, actor)
		parent := domain.ParentPath(to)
		if parent == "" {
			parent = domain.ParentPath(from)
		}

		h.redirectFilesErr(w, r, branch, parent, err)
	case "delete":
		path := strings.TrimSpace(r.FormValue("path"))
		if path == "" {
			path = rel
		}

		err := h.wiki.DeleteFilePath(r.Context(), branch, path, actor)
		h.redirectFilesErr(w, r, branch, domain.ParentPath(path), err)
	default:
		http.Error(w, h.t(r, "errors.bad_request"), http.StatusBadRequest)
	}
}

func (h *Handler) deleteFiles(w http.ResponseWriter, r *http.Request, branch, rel string, actor usecase.Actor) {
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		path = rel
	}

	err := h.wiki.DeleteFilePath(r.Context(), branch, path, actor)
	if err != nil {
		h.writeFilesAPIError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) redirectFilesErr(w http.ResponseWriter, r *http.Request, branch, rel string, err error) {
	if err != nil {
		h.writeFilesAPIError(w, r, err)
		return
	}

	http.Redirect(w, r, domain.FilesBrowsePath(branch, rel), http.StatusSeeOther)
}

func (h *Handler) writeFilesAPIError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrPathExists), errors.Is(err, domain.ErrMediaExists):
		http.Error(w, h.t(r, "errors.path_exists"), http.StatusConflict)
	case errors.Is(err, domain.ErrFileTooLarge), errors.Is(err, domain.ErrMediaTooLarge):
		http.Error(w, h.t(r, "errors.file_too_large"), http.StatusRequestEntityTooLarge)
	case errors.Is(err, domain.ErrInvalidPath), errors.Is(err, domain.ErrInvalidSlug):
		http.Error(w, h.t(r, "errors.invalid_path"), http.StatusBadRequest)
	case errors.Is(err, domain.ErrNotFound):
		http.Error(w, h.t(r, "errors.file_not_found"), http.StatusNotFound)
	case errors.Is(err, domain.ErrIsDirectory), errors.Is(err, domain.ErrNotDirectory):
		http.Error(w, h.t(r, "errors.invalid_path"), http.StatusBadRequest)
	default:
		log.Printf("файлы операция: %v", err)
		http.Error(w, h.t(r, "errors.save_failed"), http.StatusInternalServerError)
	}
}
