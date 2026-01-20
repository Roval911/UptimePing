-- +goose Up
CREATE TYPE event_type AS ENUM ('created', 'acknowledged', 'resolved', 'comment', 'status_change');

CREATE TABLE incident_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id UUID NOT NULL,
    type event_type NOT NULL,
    user_id UUID,
    message TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    CONSTRAINT fk_incident_events_incident FOREIGN KEY (incident_id) REFERENCES incidents(id) ON DELETE CASCADE,
    CONSTRAINT fk_incident_events_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_incident_events_incident_id ON incident_events(incident_id);
CREATE INDEX idx_incident_events_type ON incident_events(type);
CREATE INDEX idx_incident_events_created_at ON incident_events(created_at);
CREATE INDEX idx_incident_events_created_at_incident_id ON incident_events(created_at, incident_id);

-- +goose StatementEnd

-- +goose Down
DROP INDEX IF EXISTS idx_incident_events_created_at_incident_id;
DROP INDEX IF EXISTS idx_incident_events_created_at;
DROP INDEX IF EXISTS idx_incident_events_type;
DROP INDEX IF EXISTS idx_incident_events_incident_id;
DROP TABLE IF EXISTS incident_events;
DROP TYPE IF EXISTS event_type;

-- +goose StatementEnd