package models

import "time"

type PageIndex struct {
	ID        uint   `gorm:"primaryKey"`
	Slug      string `gorm:"uniqueIndex;size:255"`
	Title     string `gorm:"size:255"`
	Path      string `gorm:"size:512"`
	Hash      string `gorm:"size:64"`
	Size      int64
	UpdatedAt time.Time
}

type Revision struct {
	ID        uint   `gorm:"primaryKey"`
	Slug      string `gorm:"index;size:255"`
	Hash      string `gorm:"size:64"`
	Message   string
	Author    string `gorm:"size:255"`
	CreatedAt time.Time
}

type PageData struct {
	Title     string
	Slug      string
	Body      any // template.HTML
	Revisions []Revision
}

type IndexData struct {
	Title string
	Pages []PageIndex
}
