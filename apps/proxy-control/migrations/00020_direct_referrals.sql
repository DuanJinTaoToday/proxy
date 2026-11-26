-- +goose Up
CREATE TABLE tenant_promotions (
    tenant_id text PRIMARY KEY REFERENCES tenants(id),
    invite_code uuid NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    paused boolean NOT NULL DEFAULT false
);

CREATE TABLE tenant_promotion_audit (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id text NOT NULL REFERENCES tenant_promotions(tenant_id),
    operator_id text NOT NULL REFERENCES operators(id),
    previous_paused boolean NOT NULL,
    paused boolean NOT NULL CHECK (paused <> previous_paused),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX tenant_promotion_audit_tenant_idx ON tenant_promotion_audit (tenant_id, id);

ALTER TABLE tenants
    ADD COLUMN referrer_tenant_id text REFERENCES tenants(id),
    ADD COLUMN referral_source text,
    ADD CONSTRAINT tenants_referral_source CHECK (
        (referrer_tenant_id IS NULL) = (referral_source IS NULL) AND referral_source IN ('manual', 'link')
    ),
    ADD CONSTRAINT tenants_referral_not_self CHECK (referrer_tenant_id <> id);
CREATE INDEX tenants_referrer_idx ON tenants (referrer_tenant_id) WHERE referrer_tenant_id IS NOT NULL;

-- +goose StatementBegin
CREATE FUNCTION guard_tenant_referral() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'UPDATE' THEN
        IF (OLD.referrer_tenant_id, OLD.referral_source) IS DISTINCT FROM
            (NEW.referrer_tenant_id, NEW.referral_source) THEN
            RAISE EXCEPTION 'direct referral is fixed at registration' USING ERRCODE = '23514';
        END IF;
    ELSIF NEW.referrer_tenant_id IS NOT NULL
        AND NOT EXISTS (SELECT 1 FROM tenants WHERE id = NEW.referrer_tenant_id) THEN
        -- An edge can only point to an already inserted tenant, so inserts cannot introduce cycles.
        RAISE EXCEPTION 'direct referrer must already exist' USING ERRCODE = '23503';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER guard_tenant_referral BEFORE INSERT OR UPDATE OF referrer_tenant_id, referral_source ON tenants
    FOR EACH ROW EXECUTE FUNCTION guard_tenant_referral();

-- +goose Down
DROP TRIGGER guard_tenant_referral ON tenants;
DROP FUNCTION guard_tenant_referral();
ALTER TABLE tenants DROP COLUMN referral_source, DROP COLUMN referrer_tenant_id;
DROP TABLE tenant_promotion_audit;
DROP TABLE tenant_promotions;
