-- Seed: default admin user (password: admin123, bcrypt cost 12)
INSERT INTO users (email, password_hash, role, balance, status, created_at, updated_at)
VALUES (
    'admin@relay.local',
    '$2a$12$erGXRlx1uoz8krEiMV9ZAO1Nxk1ZjgfTGyWwa26CTZkPtlX7cA9iu',
    'admin',
    0,
    'active',
    NOW(),
    NOW()
) ON CONFLICT DO NOTHING;

-- Seed: Claude model configs
INSERT INTO model_configs (model_name, display_name, rate, input_price, output_price, enabled, created_at, updated_at)
VALUES
    ('claude-opus-4', 'Claude Opus 4', 5.0, 15.000000, 75.000000, true, NOW(), NOW()),
    ('claude-sonnet-4', 'Claude Sonnet 4', 1.0, 3.000000, 15.000000, true, NOW(), NOW()),
    ('claude-haiku-4', 'Claude Haiku 4', 0.2, 0.250000, 1.250000, true, NOW(), NOW())
ON CONFLICT DO NOTHING;

-- Seed: default site settings
INSERT INTO settings (key, value)
VALUES
    ('site_name', 'AI Relay'),
    ('register_enabled', 'true'),
    ('default_balance', '0')
ON CONFLICT DO NOTHING;
