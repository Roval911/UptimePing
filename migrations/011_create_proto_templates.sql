-- +goose Up
CREATE TABLE proto_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    type VARCHAR(50) NOT NULL,
    config JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    CONSTRAINT fk_proto_templates_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);

CREATE INDEX idx_proto_templates_tenant_id ON proto_templates(tenant_id);
CREATE INDEX idx_proto_templates_type ON proto_templates(type);
CREATE INDEX idx_proto_templates_created_at ON proto_templates(created_at);

-- +goose StatementEnd

-- +goose Down
DROP INDEX IF EXISTS idx_proto_templates_created_at;
DROP INDEX IF EXISTS idx_proto_templates_type;
DROP INDEX IF EXISTS idx_proto_templates_tenant_id;
DROP TABLE IF EXISTS proto_templates;

-- +goose StatementEnd