package wukong

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/data/repo"
)

// Recorder files can outlive the local GOST bearer token.
func openConnectionKey(dir string) (string, error) {
	const name = "connection-signing.key"
	release, err := lockFile(filepath.Join(dir, name+".lock"))
	if err != nil {
		return "", err
	}
	defer release()

	raw, err := os.ReadFile(filepath.Join(dir, name))
	switch {
	case err == nil:
		key, decodeErr := base64.RawURLEncoding.DecodeString(string(raw))
		if decodeErr != nil || len(key) != 32 {
			return "", errors.New("invalid Recorder connection signing key")
		}
		return string(raw), nil
	case !errors.Is(err, os.ErrNotExist):
		return "", err
	default:
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return "", err
		}
		encoded := base64.RawURLEncoding.EncodeToString(key)
		return encoded, repo.WriteDurable(dir, name, encoded)
	}
}
