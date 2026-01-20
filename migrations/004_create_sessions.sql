-- +goose Up
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    refresh_token_hash VARCHAR(255) NOT NULL,
    user_agent VARCHAR(255),
    client_ip VARCHAR(45),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    last_used_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_refresh_token_hash ON sessions(refresh_token_hash);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX idx_sessions_created_at ON sessions(created_at);

-- +goose StatementEnd

-- +goose Down
DROP INDEX IF EXISTS idx_sessions_created_at;
DROP INDEX IF EXISTS idx_sessions_expires_at;
DROP INDEX IF EXISTS idx_sessions_refresh_token_hash;
DROP INDEX IF EXISTS idx_sessions_user_id;
DROP TABLE IF EXISTS sessions;

-- +goose StatementEnd