-- +goose Up
CREATE TYPE incident_status AS ENUM ('detected', 'acknowledged', 'resolved', 'ignored');
CREATE TYPE incident_severity AS ENUM ('critical', 'high', 'medium', 'low');

CREATE TABLE incidents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    check_id UUID NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    status incident_status NOT NULL DEFAULT 'detected',
    severity incident_severity NOT NULL DEFAULT 'critical',
    detected_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    acknowledged_at TIMESTAMP WITH TIME ZONE,
    acknowledged_by UUID,
    resolved_at TIMESTAMP WITH TIME ZONE,
    resolved_by UUID,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    CONSTRAINT fk_incidents_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CONSTRAINT fk_incidents_check FOREIGN KEY (check_id) REFERENCES checks(id) ON DELETE CASCADE,
    CONSTRAINT fk_incidents_acknowledged_by FOREIGN KEY (acknowledged_by) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT fk_incidents_resolved_by FOREIGN KEY (resolved_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_incidents_tenant_id ON incidents(tenant_id);
CREATE INDEX idx_incidents_check_id ON incidents(check_id);
CREATE INDEX idx_incidents_status ON incidents(status);
CREATE INDEX idx_incidents_severity ON incidents(severity);
CREATE INDEX idx_incidents_detected_at ON incidents(detected_at);
CREATE INDEX idx_incidents_created_at ON incidents(created_at);

-- +goose StatementEnd

-- +goose Down
DROP INDEX IF EXISTS idx_incidents_created_at;
DROP INDEX IF EXISTS idx_incidents_detected_at;
DROP INDEX IF EXISTS idx_incidents_severity;
DROP INDEX IF EXISTS idx_incidents_status;
DROP INDEX IF EXISTS idx_incidents_check_id;
DROP INDEX IF EXISTS idx_incidents_tenant_id;
DROP TABLE IF EXISTS incidents;
DROP TYPE IF EXISTS incident_severity;
DROP TYPE IF EXISTS incident_status;

-- +goose StatementEnd