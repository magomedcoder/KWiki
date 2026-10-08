package usecase

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/magomedcoder/kwiki/internal/domain"
)

const maxTextPreviewBytes = 256 << 10

type FileBreadcrumb struct {
	Name string
	Path string
	Href string
}

type FileItemView struct {
	domain.DirEntry
	Href     string
	PageHref string
	Icon     string
	SizeText string
}

type FilesBrowse struct {
	Branch      string
	Path        string
	IsDir       bool
	Entries     []FileItemView
	Crumbs      []FileBreadcrumb
	Content     string
	ContentType string
	Raw         []byte
	Preview     string
	Image       bool
	Video       bool
	Markdown    bool
	PageHref    string
	EditHref    string
	CloneURL    string
	PushHint    string
	CanEdit     bool
	Empty       bool
}

func (p *PageUseCase) BrowseFiles(ctx context.Context, rawBranch, rawPath string, canEdit bool) (FilesBrowse, error) {
	branch, err := p.OpenBranch(ctx, rawBranch)
	if err != nil {
		return FilesBrowse{}, err
	}

	rel, err := domain.NormalizeRepoPath(rawPath)
	if err != nil {
		return FilesBrowse{}, err
	}

	view := FilesBrowse{
		Branch:  branch.Name,
		Path:    rel,
		CanEdit: canEdit,
		Crumbs:  fileCrumbs(branch.Name, rel),
		IsDir:   true,
	}

	if rel == "" {
		entries, err := p.content.ListDir(ctx, branch.Name, "")
		if err != nil {
			return FilesBrowse{}, err
		}

		view.Entries = mapFileEntries(branch.Name, entries)
		view.Empty = len(view.Entries) == 0
		return view, nil
	}

	entry, err := p.content.Stat(ctx, branch.Name, rel)
	if err != nil {
		return FilesBrowse{}, err
	}

	if entry.Kind == domain.EntryDir {
		entries, err := p.content.ListDir(ctx, branch.Name, rel)
		if err != nil {
			return FilesBrowse{}, err
		}

		view.Entries = mapFileEntries(branch.Name, entries)
		view.Empty = len(view.Entries) == 0
		return view, nil
	}

	data, err := p.content.Read(ctx, branch.Name, rel)
	if err != nil {
		return FilesBrowse{}, err
	}

	view.IsDir = false
	view.Raw = data
	view.ContentType = domain.FileContentType(rel)
	view.Image = domain.IsImageMedia(rel)
	view.Video = domain.IsVideoMedia(rel)
	view.Markdown = domain.IsMarkdownPath(rel)
	if view.Markdown {
		slug := domain.SlugFromPath(rel)
		view.PageHref = domain.PagePath(branch.Name, slug)
		view.EditHref = domain.EditPath(branch.Name, slug)
	}

	if domain.IsTextPreview(rel) && len(data) <= maxTextPreviewBytes && utf8.Valid(data) {
		view.Preview = string(data)
		view.Content = view.Preview
	}

	return view, nil
}

func (p *PageUseCase) CreateFile(ctx context.Context, rawBranch, rawPath string, content []byte, asDir bool, actor Actor) error {
	branch, err := p.OpenBranch(ctx, rawBranch)
	if err != nil {
		return err
	}

	rel, err := domain.NormalizeRepoPath(rawPath)
	if err != nil || rel == "" {
		if err != nil {
			return err
		}

		return domain.ErrInvalidPath
	}
	if len(content) > domain.MaxFileBytes {
		return domain.ErrFileTooLarge
	}

	if _, err := p.content.Stat(ctx, branch.Name, rel); err == nil {
		return domain.ErrPathExists
	} else if !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	message := commitMessage("создание", branch.Name, rel, actor)
	if asDir {
		keep := path.Join(rel, domain.GitKeepName)
		keep, err = domain.NormalizeRepoPath(keep)
		if err != nil {
			return err
		}

		if err := p.content.Write(ctx, branch.Name, keep, []byte{}, message); err != nil {
			return err
		}

		return p.syncBranch(ctx, branch.Name)
	}

	if err := p.content.Write(ctx, branch.Name, rel, content, message); err != nil {
		return err
	}

	if domain.IsMarkdownPath(rel) {
		return p.syncBranch(ctx, branch.Name)
	}

	return nil
}

func (p *PageUseCase) UploadFile(ctx context.Context, rawBranch, folder, name string, data []byte, overwrite bool, actor Actor) (domain.DirEntry, error) {
	if len(data) == 0 {
		return domain.DirEntry{}, domain.ErrInvalidPath
	}

	if len(data) > domain.MaxFileBytes {
		return domain.DirEntry{}, domain.ErrFileTooLarge
	}

	branch, err := p.OpenBranch(ctx, rawBranch)
	if err != nil {
		return domain.DirEntry{}, err
	}

	folder, err = domain.NormalizeRepoPath(folder)
	if err != nil {
		return domain.DirEntry{}, err
	}

	name = strings.Trim(strings.ReplaceAll(name, `\`, "/"), "/")
	if name == "" || strings.Contains(name, "/") || domain.IsHiddenRepoName(name) {
		return domain.DirEntry{}, domain.ErrInvalidPath
	}

	rel, err := domain.JoinRepoPath(folder, name)
	if err != nil || rel == "" {
		if err != nil {
			return domain.DirEntry{}, err
		}

		return domain.DirEntry{}, domain.ErrInvalidPath
	}

	if existing, err := p.content.Stat(ctx, branch.Name, rel); err == nil {
		if existing.Kind == domain.EntryDir {
			return domain.DirEntry{}, domain.ErrIsDirectory
		}

		if !overwrite {
			return domain.DirEntry{}, domain.ErrPathExists
		}
	} else if !errors.Is(err, domain.ErrNotFound) {
		return domain.DirEntry{}, err
	}

	message := commitMessage("загрузка", branch.Name, rel, actor)
	if err := p.content.Write(ctx, branch.Name, rel, data, message); err != nil {
		return domain.DirEntry{}, err
	}

	if domain.IsMarkdownPath(rel) {
		if err := p.syncBranch(ctx, branch.Name); err != nil {
			return domain.DirEntry{}, err
		}
	}

	return domain.DirEntry{
		Name: name,
		Path: rel,
		Kind: domain.EntryFile,
		Size: int64(len(data)),
	}, nil
}

func (p *PageUseCase) DeleteFilePath(ctx context.Context, rawBranch, rawPath string, actor Actor) error {
	branch, err := p.OpenBranch(ctx, rawBranch)
	if err != nil {
		return err
	}

	rel, err := domain.NormalizeRepoPath(rawPath)
	if err != nil || rel == "" {
		if err != nil {
			return err
		}

		return domain.ErrInvalidPath
	}

	message := commitMessage("удаление", branch.Name, rel, actor)
	if err := p.content.DeletePath(ctx, branch.Name, rel, message); err != nil {
		return err
	}

	return p.syncBranch(ctx, branch.Name)
}

func (p *PageUseCase) MoveFilePath(ctx context.Context, rawBranch, from, to string, actor Actor) error {
	branch, err := p.OpenBranch(ctx, rawBranch)
	if err != nil {
		return err
	}

	from, err = domain.NormalizeRepoPath(from)
	if err != nil || from == "" {
		if err != nil {
			return err
		}

		return domain.ErrInvalidPath
	}

	to, err = domain.NormalizeRepoPath(to)
	if err != nil || to == "" {
		if err != nil {
			return err
		}

		return domain.ErrInvalidPath
	}

	message := commitMessage("перемещение", branch.Name, from+" -> "+to, actor)
	if err := p.content.MovePath(ctx, branch.Name, from, to, message); err != nil {
		return err
	}

	return p.syncBranch(ctx, branch.Name)
}

func (p *PageUseCase) ReadFileBytes(ctx context.Context, rawBranch, rawPath string) ([]byte, string, error) {
	branch, err := p.OpenBranch(ctx, rawBranch)
	if err != nil {
		return nil, "", err
	}

	rel, err := domain.NormalizeRepoPath(rawPath)
	if err != nil || rel == "" {
		if err != nil {
			return nil, "", err
		}

		return nil, "", domain.ErrInvalidPath
	}

	entry, err := p.content.Stat(ctx, branch.Name, rel)
	if err != nil {
		return nil, "", err
	}

	if entry.Kind == domain.EntryDir {
		return nil, "", domain.ErrIsDirectory
	}

	data, err := p.content.Read(ctx, branch.Name, rel)
	if err != nil {
		return nil, "", err
	}

	return data, domain.FileContentType(rel), nil
}

func commitMessage(action, branch, path string, actor Actor) string {
	who := "система"
	if actor.Name != "" {
		who = actor.Name
	} else if actor.Email != "" {
		who = actor.Email
	}

	return fmt.Sprintf("%s: %s/%s (%s)", action, branch, path, who)
}

func fileCrumbs(branch, rel string) []FileBreadcrumb {
	out := []FileBreadcrumb{{
		Name: branch,
		Path: "",
		Href: domain.FilesBrowsePath(branch, ""),
	}}
	if rel == "" {
		return out
	}

	parts := strings.Split(rel, "/")
	cur := ""
	for _, part := range parts {
		if cur == "" {
			cur = part
		} else {
			cur += "/" + part
		}

		out = append(out, FileBreadcrumb{
			Name: part,
			Path: cur,
			Href: domain.FilesBrowsePath(branch, cur),
		})
	}

	return out
}

func mapFileEntries(branch string, entries []domain.DirEntry) []FileItemView {
	out := make([]FileItemView, 0, len(entries))
	for _, entry := range entries {
		item := FileItemView{
			DirEntry: entry,
			Href:     domain.FilesBrowsePath(branch, entry.Path),
			Icon:     fileIcon(entry),
			SizeText: formatSize(entry.Size, entry.Kind == domain.EntryDir),
		}

		if entry.Kind == domain.EntryFile && domain.IsMarkdownPath(entry.Path) {
			item.PageHref = domain.PagePath(branch, domain.SlugFromPath(entry.Path))
		}

		out = append(out, item)
	}

	return out
}

func fileIcon(entry domain.DirEntry) string {
	if entry.Kind == domain.EntryDir {
		return "dir"
	}

	switch {
	case domain.IsMarkdownPath(entry.Path):
		return "md"
	case domain.IsImageMedia(entry.Path):
		return "image"
	case domain.IsVideoMedia(entry.Path):
		return "video"
	default:
		return "file"
	}
}

func formatSize(n int64, dir bool) string {
	if dir {
		return "-"
	}

	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}
