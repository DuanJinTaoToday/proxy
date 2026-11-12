//go:build !windows

package repo

import (
	"errors"
	"os"
	"path/filepath"
)

func replaceFile(src, dst string) error {
	if err := os.Rename(src, dst); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(dst))
	if err != nil {
		return err
	}
	return errors.Join(dir.Sync(), dir.Close())
}
