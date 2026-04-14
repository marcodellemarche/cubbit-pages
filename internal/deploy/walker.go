package deploy

import (
	"os"
	"path/filepath"
	"strings"
)

// hiddenFiles are files/directories to skip during walk.
var hiddenPrefixes = []string{".", "_"}

// FileEntry represents a file to deploy.
type FileEntry struct {
	// RelPath is the path relative to the source directory (uses forward slashes).
	RelPath string
	// AbsPath is the absolute path on disk.
	AbsPath string
}

// WalkDir recursively walks a directory and returns all files, skipping hidden files.
func WalkDir(root string) ([]FileEntry, error) {
	var files []FileEntry

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		name := info.Name()

		// Skip hidden files and directories
		if isHidden(name) && path != root {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		// Normalize to forward slashes for S3 keys
		relPath = filepath.ToSlash(relPath)

		files = append(files, FileEntry{
			RelPath: relPath,
			AbsPath: path,
		})

		return nil
	})

	return files, err
}

// isHidden returns true if the filename starts with a dot or underscore.
func isHidden(name string) bool {
	for _, prefix := range hiddenPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
