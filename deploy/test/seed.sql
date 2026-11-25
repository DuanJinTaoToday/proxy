BEGIN;

INSERT INTO gateways (id, name)
VALUES ('local-test', 'Local test gateway')
ON CONFLICT (id) DO NOTHING;

INSERT INTO gateway_tokens (gateway_id, scope, token_hash)
VALUES
    ('local-test', 'auth', decode('d6f23de796d83d258c0c61fb682bf881c6e288b9bc99ae33b5ecc76542c71826', 'hex')),
    ('local-test', 'report', decode('a1574854e46b7f33756b1146fa0719ebc7c810f2643737ad6b913baba0ba2817', 'hex'))
ON CONFLICT DO NOTHING;

-- Accounts below are only for the local test deployment.
INSERT INTO tenants (id, name, primary_account_name)
VALUES ('alpha', 'Alpha', 'localalpha'), ('beta', 'Beta', 'localbeta')
ON CONFLICT (id) DO NOTHING;

INSERT INTO tenant_admins (tenant_id, email, email_delivery_address, password_hash)
VALUES ('alpha', 'alpha@example.test', 'alpha@example.test', '2bd806c97f0e00af1a1fc3328fa763a9:38c0e51748906648e4e5318e2990cea0f17670ee335eafe9556e85bbb437a725'),
       ('beta', 'beta@example.test', 'beta@example.test', '81b637d8fcd2c6da6359e6963113a117:3722a65d26a8a497f340de7d6380f3ae49f7e1e2be710439ddc6a374bf176a91')
ON CONFLICT (tenant_id) DO NOTHING;

INSERT INTO wallets (tenant_id) VALUES ('alpha'), ('beta')
ON CONFLICT (tenant_id) DO NOTHING;

INSERT INTO tenant_balances (tenant_id, updated_at) VALUES ('alpha', now()), ('beta', now())
ON CONFLICT (tenant_id) DO NOTHING;

-- The recharge verification uses the same tenant and balance as the operator prototype.
INSERT INTO tenants (id, name, primary_account_name, created_at)
VALUES ('visual-recharge-tenant', '陈小岚工作室', 'chenxl-studio', '2026-10-31T13:14:00Z')
ON CONFLICT (id) DO NOTHING;

INSERT INTO tenant_admins (tenant_id, email, email_delivery_address, password_hash)
VALUES ('visual-recharge-tenant', 'chenxl-studio@example.test', 'chenxl-studio@example.test', '2bd806c97f0e00af1a1fc3328fa763a9:38c0e51748906648e4e5318e2990cea0f17670ee335eafe9556e85bbb437a725')
ON CONFLICT (tenant_id) DO NOTHING;

INSERT INTO wallets (tenant_id, available_minor)
VALUES ('visual-recharge-tenant', 22900)
ON CONFLICT (tenant_id) DO NOTHING;

INSERT INTO tenant_balances (tenant_id, available_bytes, updated_at)
VALUES ('visual-recharge-tenant', 20150000000, now())
ON CONFLICT (tenant_id) DO NOTHING;

INSERT INTO operators (id, username, password_hash)
VALUES ('local-operator', 'operator', '06e55b633481f7bb072957eabcf110c9:53683f816375ff5369a9d2b7d62b18d3d790341b14b425f87422353219d5c229'),
       ('local-operator-readonly', 'operator-readonly', '06e55b633481f7bb072957eabcf110c9:53683f816375ff5369a9d2b7d62b18d3d790341b14b425f87422353219d5c229'),
       ('local-operator-billing', 'operator-billing', '06e55b633481f7bb072957eabcf110c9:53683f816375ff5369a9d2b7d62b18d3d790341b14b425f87422353219d5c229'),
       ('local-operator-support', 'operator-support', '06e55b633481f7bb072957eabcf110c9:53683f816375ff5369a9d2b7d62b18d3d790341b14b425f87422353219d5c229')
ON CONFLICT (id) DO NOTHING;

INSERT INTO operator_permissions (operator_id, permission)
VALUES ('local-operator', 'read_operations'), ('local-operator', 'manage_billing'),
       ('local-operator', 'manage_tenant_accounts'),
       ('local-operator-readonly', 'read_operations'),
       ('local-operator-billing', 'read_operations'), ('local-operator-billing', 'manage_billing'),
       ('local-operator-support', 'read_operations'), ('local-operator-support', 'manage_tenant_accounts')
ON CONFLICT DO NOTHING;

COMMIT;
