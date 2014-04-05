package main

import (
	"go/build"
	"log"
	"os"
	"path/filepath"
)

// exists returns whether the given file or directory exists or not
func exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

func resourcePaths() (staticPath string, dataPath string) {
	staticPath = "static"
	dataPath = "data"
	if exists(dataPath) && exists(staticPath) {
		return
	}

	p, err := build.Default.Import(basePkg, "", build.FindOnly)
	if err != nil {
		log.Fatal("Could not find package dir")
	}

	staticPath = filepath.Join(p.Dir, "static")
	dataPath = filepath.Join(p.Dir, "data")
	return
}
