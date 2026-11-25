-- +goose Up
ALTER TABLE tenant_admins ALTER COLUMN phone_e164 DROP NOT NULL;
ALTER TABLE tenant_admins ADD COLUMN email text UNIQUE
    CHECK (email = lower(email) AND email = btrim(email) AND octet_length(email) BETWEEN 3 AND 254);

ALTER TABLE tenant_admins ADD COLUMN email_delivery_address text;

-- Preserve known local test tenants and every foreign key to their existing IDs.
UPDATE tenant_admins SET email = CASE tenant_id
    WHEN 'alpha' THEN 'alpha@example.test'
    WHEN 'beta' THEN 'beta@example.test'
    WHEN 'visual-recharge-tenant' THEN 'chenxl-studio@example.test'
END
WHERE (tenant_id, phone_e164) IN (
    ('alpha', '+8613800000001'),
    ('beta', '+8613800000002'),
    ('visual-recharge-tenant', '+8613800136021')
);
UPDATE tenant_admins SET email_delivery_address = email WHERE email IS NOT NULL;
ALTER TABLE tenant_admins ADD CHECK ((email IS NULL) = (email_delivery_address IS NULL));

CREATE TABLE email_access_credentials (
    purpose text NOT NULL CHECK (purpose IN ('register', 'reset', 'recover')),
    subject text NOT NULL,
    email text NOT NULL,
    delivery_address text NOT NULL,
    tenant_id text REFERENCES tenants(id) ON DELETE CASCADE,
    session_version bigint,
    operator_id text REFERENCES operators(id),
    secret_hash bytea NOT NULL CHECK (octet_length(secret_hash) = 32),
    expires_at timestamptz NOT NULL,
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 5),
    consumed_at timestamptz,
    PRIMARY KEY (purpose, subject),
    CHECK ((purpose = 'register') = (tenant_id IS NULL)),
    CHECK ((purpose = 'register') = (session_version IS NULL)),
    CHECK ((purpose = 'recover') = (operator_id IS NOT NULL))
);
CREATE UNIQUE INDEX email_access_link_secret_idx ON email_access_credentials (purpose, secret_hash)
    WHERE purpose <> 'register';

CREATE TABLE email_access_rate_limits (
    key bytea PRIMARY KEY,
    expires_at timestamptz NOT NULL,
    count integer NOT NULL CHECK (count > 0)
);
CREATE INDEX email_access_rate_limits_expires_idx ON email_access_rate_limits (expires_at);

CREATE TABLE email_recovery_audit (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id text NOT NULL REFERENCES tenants(id),
    operator_id text NOT NULL REFERENCES operators(id),
    previous_email text,
    verified_email text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
-- This cannot roll back after a tenant has registered without a legacy phone number.
ALTER TABLE tenant_admins ALTER COLUMN phone_e164 SET NOT NULL;
DROP TABLE email_recovery_audit;
DROP TABLE email_access_rate_limits;
DROP TABLE email_access_credentials;
ALTER TABLE tenant_admins DROP COLUMN email, DROP COLUMN email_delivery_address;
