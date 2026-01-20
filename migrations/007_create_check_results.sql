-- +goose Up
CREATE TYPE result_status AS ENUM ('up', 'down', 'unknown');

CREATE TABLE check_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    check_id UUID NOT NULL,
    status result_status NOT NULL,
    response_time DOUBLE PRECISION,
    response_code INTEGER,
    response_body TEXT,
    error_message TEXT,
    location VARCHAR(100) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    CONSTRAINT fk_check_results_check FOREIGN KEY (check_id) REFERENCES checks(id) ON DELETE CASCADE
);

CREATE INDEX idx_check_results_check_id ON check_results(check_id);
CREATE INDEX idx_check_results_status ON check_results(status);
CREATE INDEX idx_check_results_created_at ON check_results(created_at);
CREATE INDEX idx_check_results_created_at_check_id ON check_results(created_at, check_id);

-- +goose StatementEnd

-- +goose Down
DROP INDEX IF EXISTS idx_check_results_created_at_check_id;
DROP INDEX IF EXISTS idx_check_results_created_at;
DROP INDEX IF EXISTS idx_check_results_status;
DROP INDEX IF EXISTS idx_check_results_check_id;
DROP TABLE IF EXISTS check_results;
DROP TYPE IF EXISTS result_status;

-- +goose StatementEnd