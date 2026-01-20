-- +goose Up
CREATE TABLE schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    check_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    timezone VARCHAR(50) NOT NULL DEFAULT 'UTC',
    type VARCHAR(20) NOT NULL DEFAULT 'cron',
    cron_expression VARCHAR(100),
    interval_seconds INTEGER,
    start_date TIMESTAMP WITH TIME ZONE,
    end_date TIMESTAMP WITH TIME ZONE,
    days_of_week JSONB DEFAULT '[]',
    days_of_month JSONB DEFAULT '[]',
    months JSONB DEFAULT '[1,2,3,4,5,6,7,8,9,10,11,12]',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    CONSTRAINT fk_schedules_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT fk_schedules_check FOREIGN KEY (check_id) REFERENCES checks(id) ON DELETE CASCADE
);

CREATE INDEX idx_schedules_tenant_id ON schedules(tenant_id);
CREATE INDEX idx_schedules_check_id ON schedules(check_id);
CREATE INDEX idx_schedules_type ON schedules(type);
CREATE INDEX idx_schedules_created_at ON schedules(created_at);

-- +goose StatementEnd

-- +goose Down
DROP INDEX IF EXISTS idx_schedules_created_at;
DROP INDEX IF EXISTS idx_schedules_type;
DROP INDEX IF EXISTS idx_schedules_check_id;
DROP INDEX IF EXISTS idx_schedules_tenant_id;
DROP TABLE IF EXISTS schedules;

-- +goose StatementEnd