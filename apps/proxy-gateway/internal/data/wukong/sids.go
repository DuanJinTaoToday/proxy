package wukong

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"

	"github.com/DuanJinTaoToday/proxy/apps/proxy-gateway/internal/data/repo"
)

type sidStore struct {
	mu        sync.Mutex
	dir, name string
	values    map[string]int64
	used      map[int64]bool
}

func openSIDs(dir, account string) (*sidStore, func(), error) {
	name := fmt.Sprintf("wukong-sids-%x.json", sha256.Sum256([]byte(account)))
	release, err := lockFile(filepath.Join(dir, name+".lock"))
	if err != nil {
		return nil, nil, fmt.Errorf("lock Wukong SID state: %w", err)
	}
	store := &sidStore{dir: dir, name: name, values: make(map[string]int64), used: make(map[int64]bool)}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if errors.Is(err, os.ErrNotExist) {
		return store, release, nil
	}
	if err == nil {
		if json.Unmarshal(data, &store.values) != nil || store.values == nil {
			err = errors.New("invalid Wukong SID state")
		}
		for key, sid := range store.values {
			if key == "" || sid < 100000000 || sid > 999999999 || store.used[sid] {
				err = errors.New("invalid Wukong SID mapping")
				break
			}
			store.used[sid] = true
		}
	}
	if err != nil {
		release()
		return nil, nil, err
	}
	return store, release, nil
}

func (s *sidStore) get(key string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sid := s.values[key]; sid != 0 && (key != "probe" || sid >= 900000000) {
		return sid, nil
	}
	width, start := int64(800000000), int64(100000000)
	if key == "probe" {
		width, start = 100000000, 900000000
	}
	for int64(len(s.used)) < width {
		draw, err := rand.Int(rand.Reader, big.NewInt(width))
		if err != nil {
			return 0, err
		}
		sid := draw.Int64() + start
		if s.used[sid] {
			continue
		}
		s.values[key] = sid
		data, err := json.Marshal(s.values)
		if err == nil {
			err = repo.WriteDurable(s.dir, s.name, string(data))
		}
		if err != nil {
			delete(s.values, key)
			return 0, err
		}
		s.used[sid] = true
		return sid, nil
	}
	return 0, errors.New("the Wukong SID space is exhausted")
}
