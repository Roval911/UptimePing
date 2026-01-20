-- Database Schema for UptimePing Platform
-- Created: 2026-01-20

-- Таблица пользователей
cREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    is_active BOOLEAN DEFAULT true,
    is_verified BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Таблица организаций (мультитенантность)
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    plan VARCHAR(50) DEFAULT 'free',
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Связь пользователей с организациями
ALTER TABLE users ADD CONSTRAINT fk_users_tenant_id FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;

-- Индексы для пользователей
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_tenant_id ON users(tenant_id);

-- Таблица API ключей
cREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(255) NOT NULL,
    key_prefix VARCHAR(16) NOT NULL,
    permissions JSONB DEFAULT '{}'::jsonb,
    is_active BOOLEAN DEFAULT true,
    expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Связь API ключей с организациями
ALTER TABLE api_keys ADD CONSTRAINT fk_api_keys_tenant_id FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;

-- Индексы для API ключей
CREATE INDEX idx_api_keys_tenant_id ON api_keys(tenant_id);
CREATE INDEX idx_api_keys_key_prefix ON api_keys(key_prefix);

-- Таблица сессий (JWT refresh tokens)
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    refresh_token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Связь сессий с пользователями
ALTER TABLE sessions ADD CONSTRAINT fk_sessions_user_id FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- Индексы для сессий
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_refresh_token_hash ON sessions(refresh_token_hash);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

-- Таблица конфигураций проверок
cREATE TABLE checks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    type VARCHAR(50) NOT NULL,
    target VARCHAR(255) NOT NULL,
    interval_seconds INTEGER NOT NULL,
    timeout_seconds INTEGER DEFAULT 30,
    enabled BOOLEAN DEFAULT true,
    config JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Связь проверок с организациями
ALTER TABLE checks ADD CONSTRAINT fk_checks_tenant_id FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;

-- Индексы для проверок
CREATE INDEX idx_checks_tenant_id ON checks(tenant_id);
CREATE INDEX idx_checks_enabled ON checks(enabled);
CREATE INDEX idx_checks_type ON checks(type);
CREATE INDEX idx_checks_target ON checks(target);

-- Таблица расписаний проверок
cREATE TABLE schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    check_id UUID NOT NULL,
    next_run_at TIMESTAMP WITH TIME ZONE NOT NULL,
    last_run_at TIMESTAMP WITH TIME ZONE,
    status VARCHAR(20) DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Связь расписаний с проверками
ALTER TABLE schedules ADD CONSTRAINT fk_schedules_check_id FOREIGN KEY (check_id) REFERENCES checks(id) ON DELETE CASCADE;

-- Индексы для расписаний
CREATE INDEX idx_schedules_check_id ON schedules(check_id);
CREATE INDEX idx_schedules_next_run_at ON schedules(next_run_at);
CREATE INDEX idx_schedules_status ON schedules(status);

-- Таблица результатов проверок
cREATE TABLE check_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    check_id UUID NOT NULL,
    status VARCHAR(20) NOT NULL,
    response_time_ms DECIMAL(10,3),
    status_code INTEGER,
    response_headers JSONB,
    response_body TEXT,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Связь результатов с проверками
ALTER TABLE check_results ADD CONSTRAINT fk_check_results_check_id FOREIGN KEY (check_id) REFERENCES checks(id) ON DELETE CASCADE;

-- Индексы для результатов проверок
CREATE INDEX idx_check_results_check_id ON check_results(check_id);
CREATE INDEX idx_check_results_created_at ON check_results(created_at);
CREATE INDEX idx_check_results_status ON check_results(status);
CREATE INDEX idx_check_results_status_code ON check_results(status_code);
-- Индекс для быстрого поиска последних результатов для каждой проверки
CREATE INDEX idx_check_results_check_id_created_at ON check_results(check_id, created_at DESC);

-- Таблица инцидентов
cREATE TABLE incidents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    check_id UUID NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(20) DEFAULT 'open',
    severity VARCHAR(20) DEFAULT 'low',
    started_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Связь инцидентов с проверками
ALTER TABLE incidents ADD CONSTRAINT fk_incidents_check_id FOREIGN KEY (check_id) REFERENCES checks(id) ON DELETE CASCADE;

-- Индексы для инцидентов
CREATE INDEX idx_incidents_check_id ON incidents(check_id);
CREATE INDEX idx_incidents_status ON incidents(status);
CREATE INDEX idx_incidents_severity ON incidents(severity);
CREATE INDEX idx_incidents_started_at ON incidents(started_at);
CREATE INDEX idx_incidents_resolved_at ON incidents(resolved_at);

-- Таблица событий инцидентов
cREATE TABLE incident_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id UUID NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    description TEXT,
    data JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Связь событий с инцидентами
ALTER TABLE incident_events ADD CONSTRAINT fk_incident_events_incident_id FOREIGN KEY (incident_id) REFERENCES incidents(id) ON DELETE CASCADE;

-- Индексы для событий инцидентов
CREATE INDEX idx_incident_events_incident_id ON incident_events(incident_id);
CREATE INDEX idx_incident_events_event_type ON incident_events(event_type);
CREATE INDEX idx_incident_events_created_at ON incident_events(created_at);

-- Таблица каналов уведомлений
cREATE TABLE notification_channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    config JSONB NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Связь каналов с организациями
ALTER TABLE notification_channels ADD CONSTRAINT fk_notification_channels_tenant_id FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;

-- Индексы для каналов уведомлений
CREATE INDEX idx_notification_channels_tenant_id ON notification_channels(tenant_id);
CREATE INDEX idx_notification_channels_type ON notification_channels(type);
CREATE INDEX idx_notification_channels_is_active ON notification_channels(is_active);

-- Таблица шаблонов .proto файлов
cREATE TABLE proto_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    version VARCHAR(20) NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Индекс для шаблонов
CREATE INDEX idx_proto_templates_name ON proto_templates(name);
CREATE INDEX idx_proto_templates_version ON proto_templates(version);

-- Триггер для обновления updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Создание триггеров для таблиц с updated_at
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_tenants_updated_at BEFORE UPDATE ON tenants FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_api_keys_updated_at BEFORE UPDATE ON api_keys FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_sessions_updated_at BEFORE UPDATE ON sessions FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_checks_updated_at BEFORE UPDATE ON checks FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_schedules_updated_at BEFORE UPDATE ON schedules FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_incidents_updated_at BEFORE UPDATE ON incidents FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_notification_channels_updated_at BEFORE UPDATE ON notification_channels FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_proto_templates_updated_at BEFORE UPDATE ON proto_templates FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();