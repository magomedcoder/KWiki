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
	Slug      string `gorm:"uniqueIndex;size:255"`
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
	Slug      string `gorm:"index;size:255"`
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

	if err := db.AutoMigrate(&pageRow{}, &revisionRow{}, &userRow{}, &sessionRow{}, &attemptRow{}); err != nil {
		return nil, err
	}

	return &Repository{db: db}, nil
}

func (r *Repository) List(ctx context.Context) ([]domain.Page, error) {
	var rows []pageRow
	if err := r.db.WithContext(ctx).Order("slug ASC").Find(&rows).Error; err != nil {
		return nil, err
	}

	pages := make([]domain.Page, 0, len(rows))
	for _, row := range rows {
		pages = append(pages, toPage(row))
	}

	return pages, nil
}

func (r *Repository) Revisions(ctx context.Context, slug string, limit int) ([]domain.Revision, error) {
	var rows []revisionRow
	err := r.db.WithContext(ctx).
		Where("slug = ?", slug).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	revs := make([]domain.Revision, 0, len(rows))
	for _, row := range rows {
		revs = append(revs, domain.Revision{
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
		Slug:      page.Slug,
		Title:     page.Title,
		Path:      page.Path,
		Hash:      page.Hash,
		Size:      page.Size,
		UpdatedAt: page.UpdatedAt,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "slug"}},
		DoUpdates: clause.AssignmentColumns([]string{"title", "path", "hash", "size", "updated_at"}),
	}).Create(&row).Error
}

func (r *Repository) AddRevisionIfMissing(ctx context.Context, rev domain.Revision) error {
	var count int64
	err := r.db.WithContext(ctx).Model(&revisionRow{}).
		Where("slug = ? AND hash = ?", rev.Slug, rev.Hash).
		Count(&count).Error
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	return r.db.WithContext(ctx).Create(&revisionRow{
		Slug:      rev.Slug,
		Hash:      rev.Hash,
		Message:   rev.Message,
		Author:    rev.Author,
		CreatedAt: rev.CreatedAt,
	}).Error
}

func toPage(row pageRow) domain.Page {
	return domain.Page{
		Slug:      row.Slug,
		Title:     row.Title,
		Path:      row.Path,
		Hash:      row.Hash,
		Size:      row.Size,
		UpdatedAt: row.UpdatedAt,
	}
}
