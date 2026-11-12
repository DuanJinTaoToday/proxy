package usage

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"

	"github.com/go-kratos/kratos/v3/errors"
)

var errInvalidPageToken = errors.BadRequest("INVALID_PAGE_TOKEN", "page_token does not match the query")

type pageCursor[K any] struct {
	QueryHash [sha256.Size]byte
	Key       K
}

func readPage[K any](query any, size int, token string) (pageCursor[K], *K, int, error) {
	var cursor pageCursor[K]
	if size == 0 {
		size = 100
	}
	if size < 1 || size > 500 {
		return cursor, nil, 0, errors.BadRequest("INVALID_PAGE_SIZE", "page_size must be between 1 and 500")
	}
	encoded, err := json.Marshal(query)
	if err != nil {
		return cursor, nil, 0, err
	}
	cursor.QueryHash = sha256.Sum256(encoded)
	if token == "" {
		return cursor, nil, size, nil
	}
	if len(token) > 2048 {
		return cursor, nil, 0, errInvalidPageToken
	}
	data, err := base64.RawURLEncoding.DecodeString(token)
	var decoded pageCursor[K]
	if err != nil || json.Unmarshal(data, &decoded) != nil || decoded.QueryHash != cursor.QueryHash {
		return cursor, nil, 0, errInvalidPageToken
	}
	return cursor, &decoded.Key, size, nil
}

func (c pageCursor[K]) next(key K) (string, error) {
	c.Key = key
	encoded, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}
