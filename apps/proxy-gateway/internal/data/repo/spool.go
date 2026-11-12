package repo

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/biz/recorder"
)

const (
	SessionFile   = "sessions.jsonl"
	ExitFile      = "exited_at"
	cursorFile    = "confirmed.offset"
	exitAckFile   = "exit.confirmed"
	replayAckFile = "replay.confirmed"
)

type Spool struct{ root string }

func NewSpool(root string) *Spool { return &Spool{root: root} }

func (s *Spool) Boots() ([]string, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	var boots []string
	for _, entry := range entries {
		if entry.IsDir() && entry.Name()[0] != '.' {
			boots = append(boots, entry.Name())
		}
	}
	return boots, nil
}

func (s *Spool) SessionSize(boot string) (int64, error) {
	info, err := os.Stat(filepath.Join(s.root, boot, SessionFile))
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func (s *Spool) OpenSession(boot string) (*os.File, error) {
	return os.Open(filepath.Join(s.root, boot, SessionFile))
}

func (s *Spool) Offset(boot string) (uint64, error) {
	raw, err := os.ReadFile(filepath.Join(s.root, boot, cursorFile))
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	offset, err := strconv.ParseUint(string(bytes.TrimSpace(raw)), 10, 64)
	if err != nil {
		return 0, recorder.PermanentError{Err: errors.New("invalid persisted Recorder offset")}
	}
	return offset, nil
}

func (s *Spool) SaveOffset(boot string, offset uint64) error {
	return WriteDurable(filepath.Join(s.root, boot), cursorFile, strconv.FormatUint(offset, 10)+"\n")
}

func (s *Spool) ReadExit(boot string) ([]byte, error) {
	return os.ReadFile(filepath.Join(s.root, boot, ExitFile))
}

func (s *Spool) ExitConfirmed(boot string) (bool, error) {
	_, err := os.Stat(filepath.Join(s.root, boot, exitAckFile))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func (s *Spool) ConfirmExit(boot string) error {
	return WriteDurable(filepath.Join(s.root, boot), exitAckFile, "confirmed\n")
}

func (s *Spool) StatExit(boot string) error {
	_, err := os.Stat(filepath.Join(s.root, boot, ExitFile))
	return err
}

func (s *Spool) ReplayCompleted(boot string) (bool, error) {
	_, err := os.Stat(filepath.Join(s.root, boot, replayAckFile))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func (s *Spool) ConfirmReplay(boot string) error {
	return WriteDurable(filepath.Join(s.root, boot), replayAckFile, "confirmed\n")
}

func (s *Spool) CleanupSession(boot string, cutoff time.Time) error {
	dir := filepath.Join(s.root, boot)
	ack, err := os.Stat(filepath.Join(dir, replayAckFile))
	if err != nil || ack.ModTime().After(cutoff) {
		return err
	}
	path := filepath.Join(dir, SessionFile)
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	offset, err := s.Offset(boot)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || uint64(info.Size()) != offset {
		return errors.New("closed Recorder file changed after replay completion")
	}
	return os.Remove(path)
}
