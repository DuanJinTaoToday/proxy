package connectionid

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

const prefix = "conn:"

func New(key, client string) string {
	payload := prefix + base64.RawURLEncoding.EncodeToString([]byte(client)) + ":" + rand.Text()
	return payload + ":" + signature(key, payload)
}

func Stable(key, id string) (string, bool) {
	if !strings.HasPrefix(id, prefix) {
		return id, true
	}
	encoded, rest, ok := strings.Cut(strings.TrimPrefix(id, prefix), ":")
	nonce, mac, signed := strings.Cut(rest, ":")
	if !ok || !signed || nonce == "" || !hmac.Equal([]byte(mac), []byte(signature(key, prefix+encoded+":"+nonce))) {
		return "", false
	}
	client, err := base64.RawURLEncoding.DecodeString(encoded)
	return string(client), err == nil && len(client) != 0
}

func signature(key, payload string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:16])
}
