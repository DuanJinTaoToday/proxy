package access

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"time"
)

const (
	passwordIterations = 600_000
	sessionDuration    = 8 * time.Hour
)

type Permission uint8

const (
	ReadTenantUsage Permission = 1
	ReadOperations  Permission = 3
	ManageBilling   Permission = 4
)

type Tenant struct {
	ID, Name, PrimaryAccountName, Email string
}

type Service struct {
	repo        Repository
	signingKey  [32]byte
	verifySlots chan struct{}
	delivery    *emailDelivery
}

func (s *Service) sign(payload []byte) string {
	mac := hmac.New(sha256.New, s.signingKey[:])
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// Require checks the authenticated user against the exact tenant targeted by the
// operation. Callers pass the operation's bound tenant ID, or an empty ID for
// platform-wide operations, and keep the permission fixed by the operation.
func (s *Service) Require(token, tenantID string, permission Permission) error {
	_, err := s.requirePersistent(token, tenantID, permission)
	return err
}
