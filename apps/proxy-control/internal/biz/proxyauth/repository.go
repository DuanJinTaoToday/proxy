package proxyauth

import (
	"context"
	"time"
)

type Repository interface {
	InsertCredential(ctx context.Context, clientID, tenant, username string, hash []byte, expiry *time.Time, cidrs []string, actor string) (bool, error)
	DisableCredential(ctx context.Context, tenant, clientID string) (bool, error)
	WithGateway(ctx context.Context, hash []byte, handle func(GatewayTx, GatewayIdentity) error) error
}

type GatewayIdentity struct {
	ID, Status string
	Found      bool
}

type GatewayTx interface {
	FindCredential(username string) (Credential, bool, error)
	Reference(gateway, clientID, newReference string) (string, error)
	FindReferences(gateway string, references []string) ([]Credential, error)
	Quota(tenant string, now time.Time) (*Quota, error)
}

type Credential struct {
	Reference, Username, ClientID, TenantID string
	SecretHash                              []byte
	Status                                  string
	ExpiresAt                               *time.Time
	AllowedSourceCIDRs                      []string
}

type IssuedCredential struct {
	ClientID, Username, Password string
}

type Snapshot struct {
	Reference, Username, ClientID, TenantID string
	PasswordHash                            []byte
	Active                                  bool
	ExpiresAt                               *time.Time
	AllowedSourceCIDRs                      []string
	Quota                                   *Quota
}

type Quota struct {
	AvailableBytes          int64
	AdmissionThresholdBytes uint64
	ValidUntil              *time.Time
}
