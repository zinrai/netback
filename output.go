package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func writeConfig(dir, group, name, content string) error {
	groupDir := filepath.Join(dir, group)
	if err := os.MkdirAll(groupDir, 0755); err != nil {
		return fmt.Errorf("create group directory: %w", err)
	}

	return replaceFile(filepath.Join(groupDir, name), content)
}

// Not written straight to its final path: a write that fails part way through
// would truncate what is already there, and a reader arriving mid-write would
// see half a file. Not a temporary name ending in the final extension either,
// which a collector watching the directory would pick up.
func replaceFile(path, content string) error {
	dir, name := filepath.Split(path)

	// Not left empty: os.CreateTemp would then use the system temp directory,
	// and the rename onto a path on another filesystem cannot work.
	if dir == "" {
		dir = "."
	}

	tmp, err := os.CreateTemp(dir, "."+name+".")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", tmp.Name(), err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmp.Name(), err)
	}

	// Not left as created: CreateTemp makes the file readable only by its
	// owner.
	if err := os.Chmod(tmp.Name(), 0644); err != nil {
		return fmt.Errorf("chmod %s: %w", tmp.Name(), err)
	}

	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("rename to %s: %w", path, err)
	}

	return nil
}
