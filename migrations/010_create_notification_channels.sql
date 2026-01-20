-- +goose Up
CREATE TYPE channel_type AS ENUM ('email', 'slack', 'telegram', 'webhook', 'sms');
CREATE TYPE channel_status AS ENUM ('active', 'inactive', 'error');

CREATE TABLE notification_channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    type channel_type NOT NULL,
    config JSONB NOT NULL,
    status channel_status NOT NULL DEFAULT 'active',
    error_message TEXT,
    last_sent_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    CONSTRAINT fk_notification_channels_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);

CREATE INDEX idx_notification_channels_tenant_id ON notification_channels(tenant_id);
CREATE INDEX idx_notification_channels_type ON notification_channels(type);
CREATE INDEX idx_notification_channels_status ON notification_channels(status);
CREATE INDEX idx_notification_channels_created_at ON notification_channels(created_at);

-- +goose StatementEnd

-- +goose Down
DROP INDEX IF EXISTS idx_notification_channels_created_at;
DROP INDEX IF EXISTS idx_notification_channels_status;
DROP INDEX IF EXISTS idx_notification_channels_type;
DROP INDEX IF EXISTS idx_notification_channels_tenant_id;
DROP TABLE IF EXISTS notification_channels;
DROP TYPE IF EXISTS channel_status;
DROP TYPE IF EXISTS channel_type;

-- +goose StatementEnd