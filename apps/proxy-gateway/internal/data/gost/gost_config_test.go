package gost

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGOSTRequiresRegisteredRelayAuther(t *testing.T) {
	raw, err := os.ReadFile("../../../gost/auth.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	token := "configured-local-token"
	rendered := strings.ReplaceAll(string(raw), "${LOCAL_AUTH_TOKEN}", token)
	const endpoint = "http://127.0.0.1:8091/v1/gateway/auth"
	for _, tc := range []struct {
		name, config string
		valid        bool
	}{
		{"registered", rendered, true},
		{"missing", strings.ReplaceAll(rendered, "auther: proxy-auth-relay", "auther: missing"), false},
		{"second-authenticator", strings.ReplaceAll(rendered, "auther: proxy-auth-relay", "auther: proxy-auth-relay\n      authers: [other]"), false},
		{"wrong-token", strings.ReplaceAll(rendered, token, "different-token"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "gost.yaml")
			if err := os.WriteFile(file, []byte(tc.config), 0o600); err != nil {
				t.Fatal(err)
			}
			err := validateGOSTConfig(file, endpoint, token)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%t, got %v", tc.valid, err)
			}
		})
	}
}
