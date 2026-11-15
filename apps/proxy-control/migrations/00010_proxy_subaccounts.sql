-- +goose Up
CREATE TABLE proxy_subaccounts (
    client_id text PRIMARY KEY,
    tenant_id text NOT NULL REFERENCES tenants(id),
    name text NOT NULL CHECK (name ~ '^[a-z][a-z0-9]{3,7}$'),
    password_ciphertext bytea NOT NULL,
    remark text NOT NULL DEFAULT '',
    active boolean NOT NULL,
    limit_bytes bigint CHECK (limit_bytes >= 0),
    credential_version bigint NOT NULL DEFAULT 1 CHECK (credential_version > 0),
    used_bytes bigint NOT NULL DEFAULT 0 CHECK (used_bytes >= 0),
    UNIQUE (tenant_id, name)
);

CREATE TABLE gateway_auth_references (
    gateway_id text NOT NULL REFERENCES gateways(id),
    client_id text NOT NULL REFERENCES proxy_subaccounts(client_id),
    reference text NOT NULL UNIQUE,
    PRIMARY KEY (gateway_id, client_id)
);

-- Reserve the upper 100 million values for the gateway's existing health probes.
CREATE SEQUENCE upstream_session_sids MINVALUE 100000000 MAXVALUE 899999999;

CREATE TABLE sticky_sessions (
    client_id text NOT NULL REFERENCES proxy_subaccounts(client_id),
    session_key text NOT NULL CHECK (session_key ~ '^[a-z0-9]{0,32}$'),
    upstream_sid bigint NOT NULL UNIQUE,
    gateway_id text NOT NULL REFERENCES gateways(id),
    lease_token text NOT NULL,
    lease_until timestamptz NOT NULL,
    established_at timestamptz,
    expires_at timestamptz,
    PRIMARY KEY (client_id, session_key)
);

-- +goose Down
DROP TABLE sticky_sessions;
DROP SEQUENCE upstream_session_sids;
DROP TABLE gateway_auth_references, proxy_subaccounts;
