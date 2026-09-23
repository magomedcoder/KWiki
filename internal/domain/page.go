package domain

import "time"

type Page struct {
	Branch    string
	Slug      string
	Title     string
	Path      string
	Hash      string
	Size      int64
	UpdatedAt time.Time
}

type Revision struct {
	Branch    string
	Slug      string
	Hash      string
	Message   string
	Author    string
	CreatedAt time.Time
}

type ContentFile struct {
	Path string
	Hash string
	Size int64
}
