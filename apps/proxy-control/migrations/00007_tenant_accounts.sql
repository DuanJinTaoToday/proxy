-- +goose Up
CREATE TABLE tenants (
    id text PRIMARY KEY,
    name text NOT NULL,
    primary_account_name text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE operators (
    id text PRIMARY KEY,
    username text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    session_version bigint NOT NULL DEFAULT 1 CHECK (session_version > 0)
);

CREATE TABLE operator_permissions (
    operator_id text NOT NULL REFERENCES operators(id),
    permission text NOT NULL CHECK (permission IN ('read_operations', 'manage_billing', 'manage_tenant_accounts')),
    PRIMARY KEY (operator_id, permission)
);

CREATE TABLE tenant_admins (
    tenant_id text PRIMARY KEY REFERENCES tenants(id),
    phone_e164 text NOT NULL UNIQUE CHECK (phone_e164 ~ '^[+]861[3-9][0-9]{9}$'),
    password_hash text NOT NULL,
    session_version bigint NOT NULL DEFAULT 1 CHECK (session_version > 0),
    password_reset_at timestamptz,
    password_reset_by text REFERENCES operators(id)
);

CREATE TABLE wallets (
    tenant_id text PRIMARY KEY REFERENCES tenants(id),
    available_minor bigint NOT NULL DEFAULT 0 CHECK (available_minor >= 0)
);

-- 流量余额沿用既有账户表。#45 将在此基础上切换扣减模型。

CREATE TABLE tenant_balances (
    tenant_id text PRIMARY KEY REFERENCES tenants(id),
    available_bytes bigint NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL
);

-- +goose Down
DROP TABLE tenant_balances, wallets, tenant_admins, operator_permissions, operators, tenants;
