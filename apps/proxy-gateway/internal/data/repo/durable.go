package repo

import (
	"os"
	"path/filepath"
)

// WriteDurable replaces a spool marker only after flushing its new contents.
func WriteDurable(dir, name, text string) error {
	f, err := os.CreateTemp(dir, ".state-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err = f.WriteString(text); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return replaceFile(f.Name(), filepath.Join(dir, name))
}
