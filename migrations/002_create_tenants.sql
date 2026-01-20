-- +goose Up
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    billing_email VARCHAR(255),
    stripe_customer_id VARCHAR(255),
    stripe_subscription_id VARCHAR(255),
    subscription_status VARCHAR(50),
    subscription_plan VARCHAR(50),
    trial_end_date TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_tenants_slug ON tenants(slug);
CREATE INDEX idx_tenants_subscription_status ON tenants(subscription_status);
CREATE INDEX idx_tenants_created_at ON tenants(created_at);

-- +goose StatementEnd

-- +goose Down
DROP INDEX IF EXISTS idx_tenants_created_at;
DROP INDEX IF EXISTS idx_tenants_subscription_status;
DROP INDEX IF EXISTS idx_tenants_slug;
DROP TABLE IF EXISTS tenants;

-- +goose StatementEnd