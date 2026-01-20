-- +goose Up
CREATE TYPE check_type AS ENUM ('http', 'tcp', 'dns', 'ping', 'ssl');
CREATE TYPE check_status AS ENUM ('active', 'paused', 'disabled');

CREATE TABLE checks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    type check_type NOT NULL,
    target VARCHAR(255) NOT NULL,
    frequency INTEGER NOT NULL DEFAULT 60,
    timeout INTEGER NOT NULL DEFAULT 30,
    max_retries INTEGER NOT NULL DEFAULT 1,
    retry_interval INTEGER NOT NULL DEFAULT 10,
    method VARCHAR(10) DEFAULT 'GET',
    headers JSONB DEFAULT '{}',
    body TEXT,
    expected_status INTEGER,
    expected_response TEXT,
    follow_redirects BOOLEAN DEFAULT TRUE,
    verify_ssl BOOLEAN DEFAULT TRUE,
    port INTEGER,
    dns_server VARCHAR(255),
    dns_record_type VARCHAR(10),
    ssl_check_days INTEGER DEFAULT 30,
    notification_delay INTEGER NOT NULL DEFAULT 300,
    status check_status NOT NULL DEFAULT 'active',
    tags JSONB DEFAULT '[]',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    CONSTRAINT fk_checks_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);

CREATE INDEX idx_checks_tenant_id ON checks(tenant_id);
CREATE INDEX idx_checks_status ON checks(status);
CREATE INDEX idx_checks_type ON checks(type);
CREATE INDEX idx_checks_created_at ON checks(created_at);
CREATE INDEX idx_checks_tags ON checks USING GIN (tags);

-- +goose StatementEnd

-- +goose Down
DROP INDEX IF EXISTS idx_checks_tags;
DROP INDEX IF EXISTS idx_checks_created_at;
DROP INDEX IF EXISTS idx_checks_type;
DROP INDEX IF EXISTS idx_checks_status;
DROP INDEX IF EXISTS idx_checks_tenant_id;
DROP TABLE IF EXISTS checks;
DROP TYPE IF EXISTS check_status;
DROP TYPE IF EXISTS check_type;

-- +goose StatementEnd