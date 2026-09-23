package main

import "path/filepath"

type dataPaths struct {
	Root string
	Repo string
	DB   string
}

func pathsFrom(root string) dataPaths {
	return dataPaths{
		Root: root,
		Repo: filepath.Join(root, "repo"),
		DB:   filepath.Join(root, "wiki.db"),
	}
}
