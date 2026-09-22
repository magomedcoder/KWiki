package db

import (
	"github.com/magomedcoder/kwiki/internal/gitstore"
	"github.com/magomedcoder/kwiki/internal/models"
	"strings"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func Open(path string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(&models.PageIndex{}, &models.Revision{}); err != nil {
		return nil, err
	}

	return db, nil
}

func SyncIndex(db *gorm.DB, store *gitstore.Store) error {
	files, err := store.ListMarkdown()
	if err != nil {
		return err
	}

	for _, f := range files {
		slug := strings.TrimSuffix(f.Path, ".md")
		slug = strings.TrimPrefix(slug, "./")
		title := strings.ReplaceAll(lastSegment(slug), "-", " ")

		page := models.PageIndex{
			Slug:      slug,
			Title:     title,
			Path:      f.Path,
			Hash:      f.Hash,
			Size:      f.Size,
			UpdatedAt: time.Now(),
		}
		if err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{
				Name: "slug",
			}},
			DoUpdates: clause.AssignmentColumns([]string{"title", "path", "hash", "size", "updated_at"}),
		}).Create(&page).Error; err != nil {
			return err
		}

		var count int64
		db.Model(&models.Revision{}).
			Where("slug = ? AND hash = ?", slug, f.Hash).
			Count(&count)
		if count == 0 {
			db.Create(&models.Revision{
				Slug:      slug,
				Hash:      f.Hash,
				Message:   "синхронизация индекса",
				Author:    "система",
				CreatedAt: time.Now(),
			})
		}
	}
	return nil
}

func lastSegment(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}

	return p
}
