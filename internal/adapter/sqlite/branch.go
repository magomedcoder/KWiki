package sqlite

import (
	"context"
	"errors"
	"time"

	"github.com/magomedcoder/kwiki/internal/domain"
	"gorm.io/gorm"
)

var _ domain.BranchRepository = (*Repository)(nil)

type branchRow struct {
	Name      string `gorm:"primaryKey;size:64"`
	Public    bool
	CreatedAt time.Time
}

func (branchRow) TableName() string {
	return "branches"
}

func (r *Repository) CreateBranch(ctx context.Context, branch domain.Branch) error {
	return r.db.WithContext(ctx).Create(&branchRow{
		Name:      branch.Name,
		Public:    branch.Public,
		CreatedAt: branch.CreatedAt,
	}).Error
}

func (r *Repository) FindBranch(ctx context.Context, name string) (domain.Branch, error) {
	var row branchRow
	err := r.db.WithContext(ctx).First(&row, "name = ?", name).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Branch{}, domain.ErrNotFound
	}

	if err != nil {
		return domain.Branch{}, err
	}

	return toBranch(row), nil
}

func (r *Repository) ListBranches(ctx context.Context) ([]domain.Branch, error) {
	var rows []branchRow
	if err := r.db.WithContext(ctx).Order("name ASC").Find(&rows).Error; err != nil {
		return nil, err
	}

	branches := make([]domain.Branch, 0, len(rows))
	var head []domain.Branch
	for _, row := range rows {
		branch := toBranch(row)
		if branch.Name == domain.DefaultBranch {
			head = append(head, branch)
			continue
		}

		branches = append(branches, branch)
	}

	return append(head, branches...), nil
}

func (r *Repository) SetBranchPublic(ctx context.Context, name string, public bool) error {
	res := r.db.WithContext(ctx).Model(&branchRow{}).Where("name = ?", name).Update("public", public)
	if res.Error != nil {
		return res.Error
	}

	if res.RowsAffected > 0 {
		return nil
	}

	var n int64
	if err := r.db.WithContext(ctx).Model(&branchRow{}).Where("name = ?", name).Count(&n).Error; err != nil {
		return err
	}

	if n == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *Repository) DeleteBranch(ctx context.Context, name string) error {
	res := r.db.WithContext(ctx).Delete(&branchRow{}, "name = ?", name)
	if res.Error != nil {
		return res.Error
	}

	if res.RowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func toBranch(row branchRow) domain.Branch {
	return domain.Branch{
		Name:      row.Name,
		Public:    row.Public,
		CreatedAt: row.CreatedAt,
	}
}
