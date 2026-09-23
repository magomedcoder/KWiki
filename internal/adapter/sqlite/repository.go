package sqlite

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/magomedcoder/kwiki/internal/domain"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type Repository struct {
	db *gorm.DB
}

var _ domain.PageRepository = (*Repository)(nil)

type pageRow struct {
	ID        uint   `gorm:"primaryKey"`
	Branch    string `gorm:"uniqueIndex:idx_page_branch_slug,priority:1;size:64;not null;default:main"`
	Slug      string `gorm:"uniqueIndex:idx_page_branch_slug,priority:2;size:255;not null"`
	Title     string `gorm:"size:255"`
	Path      string `gorm:"size:512"`
	Hash      string `gorm:"size:64"`
	Size      int64
	UpdatedAt time.Time
}

func (pageRow) TableName() string {
	return "page_indices"
}

type revisionRow struct {
	ID        uint   `gorm:"primaryKey"`
	Branch    string `gorm:"index:idx_rev_branch_slug,priority:1;size:64;not null;default:main"`
	Slug      string `gorm:"index:idx_rev_branch_slug,priority:2;size:255"`
	Hash      string `gorm:"size:64"`
	Message   string
	Author    string `gorm:"size:255"`
	CreatedAt time.Time
}

func (revisionRow) TableName() string {
	return "revisions"
}

func Open(path string) (*Repository, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
		}),
	})
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(&pageRow{}, &revisionRow{}, &branchRow{}, &userRow{}, &sessionRow{}, &attemptRow{}); err != nil {
		return nil, err
	}

	if err := migrateBranches(db); err != nil {
		return nil, err
	}

	return &Repository{db: db}, nil
}

func migrateBranches(db *gorm.DB) error {
	if err := db.Exec(`UPDATE page_indices SET branch = 'main' WHERE branch IS NULL OR branch = ''`).Error; err != nil {
		return err
	}

	if err := db.Exec(`UPDATE revisions SET branch = 'main' WHERE branch IS NULL OR branch = ''`).Error; err != nil {
		return err
	}

	return db.Exec(`DROP INDEX IF EXISTS idx_page_indices_slug`).Error
}

func (r *Repository) List(ctx context.Context, branch string) ([]domain.Page, error) {
	var rows []pageRow
	if err := r.db.WithContext(ctx).Where("branch = ?", branch).Order("slug ASC").Find(&rows).Error; err != nil {
		return nil, err
	}

	pages := make([]domain.Page, 0, len(rows))
	for _, row := range rows {
		pages = append(pages, toPage(row))
	}

	return pages, nil
}

func (r *Repository) Revisions(ctx context.Context, branch, slug string, limit int) ([]domain.Revision, error) {
	var rows []revisionRow
	err := r.db.WithContext(ctx).
		Where("branch = ? AND slug = ?", branch, slug).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	revs := make([]domain.Revision, 0, len(rows))
	for _, row := range rows {
		revs = append(revs, domain.Revision{
			Branch:    row.Branch,
			Slug:      row.Slug,
			Hash:      row.Hash,
			Message:   row.Message,
			Author:    row.Author,
			CreatedAt: row.CreatedAt,
		})
	}

	return revs, nil
}

func (r *Repository) Upsert(ctx context.Context, page domain.Page) error {
	row := pageRow{
		Branch:    page.Branch,
		Slug:      page.Slug,
		Title:     page.Title,
		Path:      page.Path,
		Hash:      page.Hash,
		Size:      page.Size,
		UpdatedAt: page.UpdatedAt,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "branch"}, {Name: "slug"}},
		DoUpdates: clause.AssignmentColumns([]string{"title", "path", "hash", "size", "updated_at"}),
	}).Create(&row).Error
}

func (r *Repository) AddRevisionIfMissing(ctx context.Context, rev domain.Revision) error {
	var count int64
	err := r.db.WithContext(ctx).Model(&revisionRow{}).
		Where("branch = ? AND slug = ? AND hash = ?", rev.Branch, rev.Slug, rev.Hash).
		Count(&count).Error
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	return r.db.WithContext(ctx).Create(&revisionRow{
		Branch:    rev.Branch,
		Slug:      rev.Slug,
		Hash:      rev.Hash,
		Message:   rev.Message,
		Author:    rev.Author,
		CreatedAt: rev.CreatedAt,
	}).Error
}

func (r *Repository) Prune(ctx context.Context, branch string, slugs []string) error {
	query := r.db.WithContext(ctx).Where("branch = ?", branch)
	if len(slugs) > 0 {
		query = query.Where("slug NOT IN ?", slugs)
	}

	return query.Delete(&pageRow{}).Error
}

func (r *Repository) DeleteBranchPages(ctx context.Context, branch string) error {
	if err := r.db.WithContext(ctx).Where("branch = ?", branch).Delete(&pageRow{}).Error; err != nil {
		return err
	}

	return r.db.WithContext(ctx).Where("branch = ?", branch).Delete(&revisionRow{}).Error
}

func toPage(row pageRow) domain.Page {
	return domain.Page{
		Branch:    row.Branch,
		Slug:      row.Slug,
		Title:     row.Title,
		Path:      row.Path,
		Hash:      row.Hash,
		Size:      row.Size,
		UpdatedAt: row.UpdatedAt,
	}
}
