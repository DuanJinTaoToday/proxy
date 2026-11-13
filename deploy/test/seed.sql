BEGIN;

INSERT INTO gateways (id, name)
VALUES ('local-test', 'Local test gateway')
ON CONFLICT (id) DO NOTHING;

INSERT INTO gateway_tokens (gateway_id, scope, token_hash)
VALUES
    ('local-test', 'auth', decode('d6f23de796d83d258c0c61fb682bf881c6e288b9bc99ae33b5ecc76542c71826', 'hex')),
    ('local-test', 'report', decode('a1574854e46b7f33756b1146fa0719ebc7c810f2643737ad6b913baba0ba2817', 'hex'))
ON CONFLICT DO NOTHING;

COMMIT;
