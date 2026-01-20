-- +goose Up
-- Insert development tenant
INSERT INTO tenants (id, name, slug, created_at, updated_at) VALUES 
    ('a1a1a1a1-a1a1-a1a1-a1a1-a1a1a1a1a1a1', 'Development Tenant', 'dev-tenant', NOW(), NOW());

-- Insert development users
INSERT INTO users (id, tenant_id, email, password_hash, first_name, last_name, role, status, email_verified, created_at, updated_at) VALUES 
    ('b1b1b1b1-b1b1-b1b1-b1b1-b1b1b1b1b1b1', 'a1a1a1a1-a1a1-a1a1-a1a1-a1a1a1a1a1a1', 'admin@dev.local', '$2a$10$N.y1M9THCv3eIPW6j8C0Ue6L6W.h9t0Uq4pIqR8Yz6GxZgBm1U6vO', 'Admin', 'User', 'admin', 'active', TRUE, NOW(), NOW()),
    ('c1c1c1c1-c1c1-c1c1-c1c1-c1c1c1c1c1c1', 'a1a1a1a1-a1a1-a1a1-a1a1-a1a1a1a1a1a1', 'user@dev.local', '$2a$10$N.y1M9THCv3eIPW6j8C0Ue6L6W.h9t0Uq4pIqR8Yz6GxZgBm1U6vO', 'Regular', 'User', 'user', 'active', TRUE, NOW(), NOW());

-- Insert API keys
INSERT INTO api_keys (id, tenant_id, name, key_hash, permissions, created_at, updated_at) VALUES 
    ('d1d1d1d1-d1d1-d1d1-d1d1-d1d1d1d1d1d1', 'a1a1a1a1-a1a1-a1a1-a1a1-a1a1a1a1a1a1', 'Development API Key', 'dev-key-hash', '{"checks:read": true, "checks:write": true, "incidents:read": true}', NOW(), NOW());

-- Insert notification channels
INSERT INTO notification_channels (id, tenant_id, name, type, config, status, created_at, updated_at) VALUES 
    ('e1e1e1e1-e1e1-e1e1-e1e1-e1e1e1e1e1e1', 'a1a1a1a1-a1a1-a1a1-a1a1-a1a1a1a1a1a1', 'Development Email', 'email', '{"email": "admin@dev.local"}', 'active', NOW(), NOW());

-- +goose StatementEnd

-- +goose Down
DELETE FROM notification_channels WHERE id IN ('e1e1e1e1-e1e1-e1e1-e1e1-e1e1e1e1e1e1');
DELETE FROM api_keys WHERE id IN ('d1d1d1d1-d1d1-d1d1-d1d1-d1d1d1d1d1d1');
DELETE FROM users WHERE id IN ('b1b1b1b1-b1b1-b1b1-b1b1-b1b1b1b1b1b1', 'c1c1c1c1-c1c1-c1c1-c1c1-c1c1c1c1c1c1');
DELETE FROM tenants WHERE id IN ('a1a1a1a1-a1a1-a1a1-a1a1-a1a1a1a1a1a1');

-- +goose StatementEnd