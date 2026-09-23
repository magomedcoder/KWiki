package usecase

import (
	"context"
	"errors"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/magomedcoder/kwiki/internal/domain"
)

type MediaItem struct {
	Path   string
	URL    string
	Ref    string
	Size   int64
	Staged bool
	Image  bool
	Name   string
}

var reMediaURL = regexp.MustCompile(`/b/([a-z0-9-]+)/media/([^)\s"']+)`)

func (p *PageUseCase) StageMedia(ctx context.Context, draft, rawBranch, folder, name string, data []byte, overwrite bool) (MediaItem, error) {
	if len(data) == 0 {
		return MediaItem{}, domain.ErrInvalidMediaPath
	}

	if len(data) > domain.MaxMediaBytes {
		return MediaItem{}, domain.ErrMediaTooLarge
	}

	branch, err := p.OpenBranch(ctx, rawBranch)
	if err != nil {
		return MediaItem{}, err
	}

	rel, err := domain.NormalizeMediaRel(folder, name)
	if err != nil {
		return MediaItem{}, err
	}

	exists, err := p.mediaExists(ctx, draft, branch.Name, rel)
	if err != nil {
		return MediaItem{}, err
	}

	if exists && !overwrite {
		return MediaItem{}, domain.ErrMediaExists
	}

	if err := p.staging.put(draft, branch.Name, rel, data); err != nil {
		return MediaItem{}, err
	}

	return mediaItem(branch.Name, rel, int64(len(data)), true), nil
}

func (p *PageUseCase) ListMedia(ctx context.Context, draft, rawBranch string) ([]MediaItem, error) {
	branch, err := p.OpenBranch(ctx, rawBranch)
	if err != nil {
		return nil, err
	}

	committed, err := p.content.ListPrefix(ctx, branch.Name, domain.MediaDir+"/")
	if err != nil {
		return nil, err
	}

	staged, err := p.staging.list(draft, branch.Name)
	if err != nil {
		return nil, err
	}

	byPath := map[string]MediaItem{}
	for _, file := range committed {
		byPath[file.Path] = mediaItem(branch.Name, file.Path, file.Size, false)
	}

	for _, file := range staged {
		byPath[file.Path] = mediaItem(branch.Name, file.Path, file.Size, true)
	}

	out := make([]MediaItem, 0, len(byPath))
	for _, item := range byPath {
		out = append(out, item)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})

	return out, nil
}

func (p *PageUseCase) DeleteMedia(ctx context.Context, draft, rawBranch, rawPath string) error {
	branch, err := p.OpenBranch(ctx, rawBranch)
	if err != nil {
		return err
	}

	rel, err := domain.NormalizeMediaPath(rawPath)
	if err != nil {
		return err
	}

	if _, ok, err := p.staging.get(draft, branch.Name, rel); err != nil {
		return err
	} else if ok {
		return p.staging.remove(draft, branch.Name, rel)
	}

	exists, err := p.content.Exists(ctx, branch.Name, rel)
	if err != nil {
		return err
	}

	if !exists {
		return domain.ErrNotFound
	}

	return p.content.WriteBatch(ctx, branch.Name, []domain.ContentChange{{
		Path:   rel,
		Delete: true,
	}}, "удаление медиа: "+branch.Name+"/"+rel)
}

func (p *PageUseCase) ReadMedia(ctx context.Context, draft, rawBranch, rawPath string) ([]byte, string, error) {
	branch, err := p.OpenBranch(ctx, rawBranch)
	if err != nil {
		return nil, "", err
	}

	rel, err := domain.NormalizeMediaPath(rawPath)
	if err != nil {
		return nil, "", err
	}

	if draft != "" {
		if data, ok, err := p.staging.get(draft, branch.Name, rel); err != nil {
			return nil, "", err
		} else if ok {
			return data, domain.MediaContentType(rel), nil
		}
	}

	data, err := p.content.Read(ctx, branch.Name, rel)
	if err != nil {
		return nil, "", err
	}

	return data, domain.MediaContentType(rel), nil
}

func (p *PageUseCase) mediaExists(ctx context.Context, draft, branch, rel string) (bool, error) {
	if _, ok, err := p.staging.get(draft, branch, rel); err != nil {
		return false, err
	} else if ok {
		return true, nil
	}

	return p.content.Exists(ctx, branch, rel)
}

func mediaItem(branch, rel string, size int64, staged bool) MediaItem {
	return MediaItem{
		Path:   rel,
		URL:    domain.MediaURL(branch, rel),
		Ref:    domain.MediaRef(rel),
		Size:   size,
		Staged: staged,
		Image:  domain.IsImageMedia(rel),
		Name:   path.Base(rel),
	}
}

func (p *PageUseCase) copyReferencedMedia(ctx context.Context, destBranch, content string) (string, []domain.ContentChange, error) {
	matches := reMediaURL.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return content, nil, nil
	}

	var changes []domain.ContentChange
	seen := map[string]bool{}
	out := content

	for _, match := range matches {
		srcBranch, relTail := match[1], match[2]
		rel := domain.MediaDir + "/" + strings.TrimPrefix(relTail, "/")
		rel, err := domain.NormalizeMediaPath(rel)
		if err != nil {
			continue
		}

		if srcBranch == destBranch {
			continue
		}

		key := srcBranch + "\x00" + rel
		if seen[key] {
			out = strings.ReplaceAll(out, match[0], domain.MediaRef(rel))
			continue
		}
		seen[key] = true

		data, err := p.content.Read(ctx, srcBranch, rel)
		if errors.Is(err, domain.ErrNotFound) {
			continue
		}
		if err != nil {
			return "", nil, err
		}

		exists, err := p.content.Exists(ctx, destBranch, rel)
		if err != nil {
			return "", nil, err
		}

		if !exists {
			changes = append(changes, domain.ContentChange{Path: rel, Data: data})
		}

		out = strings.ReplaceAll(out, match[0], domain.MediaRef(rel))
	}

	return out, changes, nil
}
