-- Создание таблиц в правильном порядке с учетом зависимостей
-- Начинаем с базовых таблиц, на которые ссылаются другие

-- 1. tenants - базовая таблица для арендаторов
CREATE TABLE tenants (
                         id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                         name VARCHAR(255) NOT NULL,
                         slug VARCHAR(100) NOT NULL UNIQUE,
                         plan VARCHAR(50) DEFAULT 'free',
                         status VARCHAR(20) DEFAULT 'active',
                         created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                         updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                         settings JSONB DEFAULT '{}'::jsonb
);

-- 2. users - пользователи (зависит от tenants)
CREATE TABLE users (
                       id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                       tenant_id UUID NOT NULL,
                       email VARCHAR(255) NOT NULL,
                       first_name VARCHAR(100),
                       last_name VARCHAR(100),
                       password_hash VARCHAR(255) NOT NULL,
                       is_active BOOLEAN DEFAULT true,
                       is_verified BOOLEAN DEFAULT false,
                       is_admin BOOLEAN DEFAULT false,
                       created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                       updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                       CONSTRAINT fk_users_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);

-- 3. checks - проверки (зависит от tenants)
CREATE TABLE checks (
                        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                        tenant_id UUID NOT NULL,
                        name VARCHAR(255) NOT NULL,
                        description TEXT,
                        type VARCHAR(50) NOT NULL,
                        target VARCHAR(255) NOT NULL,
                        interval_seconds INTEGER NOT NULL,
                        timeout_seconds INTEGER DEFAULT 30,
                        enabled BOOLEAN DEFAULT true,
                        config JSONB,
                        created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                        updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                        CONSTRAINT fk_checks_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);

-- 4. api_keys - API ключи (зависит от tenants)
CREATE TABLE api_keys (
                          id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                          tenant_id UUID NOT NULL,
                          name VARCHAR(255) NOT NULL,
                          key_hash VARCHAR(255) NOT NULL,
                          key_prefix VARCHAR(16) NOT NULL,
                          permissions JSONB DEFAULT '0'::jsonb,
                          is_active BOOLEAN DEFAULT true,
                          expires_at TIMESTAMPTZ,
                          created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                          updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                          CONSTRAINT fk_api_keys_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);

-- 5. notification_channels - каналы уведомлений (зависит от tenants)
CREATE TABLE notification_channels (
                                       id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                       tenant_id UUID NOT NULL,
                                       name VARCHAR(255) NOT NULL,
                                       type VARCHAR(50) NOT NULL,
                                       config JSONB,
                                       is_active BOOLEAN DEFAULT true,
                                       created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                                       updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                                       CONSTRAINT fk_notification_channels_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);

-- 6. proto_templates - шаблоны протоколов (не зависит от других)
CREATE TABLE proto_templates (
                                 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                 name VARCHAR(255) NOT NULL,
                                 content TEXT NOT NULL,
                                 version VARCHAR(20),
                                 description TEXT,
                                 created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                                 updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- 7. incidents - инциденты (зависит от checks)
CREATE TABLE incidents (
                           id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                           check_id UUID NOT NULL,
                           title VARCHAR(255) NOT NULL,
                           description TEXT,
                           status VARCHAR(20) DEFAULT 'open',
                           severity VARCHAR(20) DEFAULT 'low',
                           started_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                           resolved_at TIMESTAMPTZ,
                           created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                           updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                           CONSTRAINT fk_incidents_check FOREIGN KEY (check_id) REFERENCES checks(id) ON DELETE CASCADE
);

-- 8. schedules - расписания (зависит от checks)
CREATE TABLE schedules (
                           id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                           check_id UUID NOT NULL,
                           next_run_at TIMESTAMPTZ,
                           last_run_at TIMESTAMPTZ,
                           status VARCHAR(20) DEFAULT 'pending',
                           created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                           updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                           CONSTRAINT fk_schedules_check FOREIGN KEY (check_id) REFERENCES checks(id) ON DELETE CASCADE
);

-- 9. check_results - результаты проверок (зависит от checks)
CREATE TABLE check_results (
                               id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                               check_id UUID NOT NULL,
                               status VARCHAR(20) NOT NULL,
                               response_time_ms NUMERIC(10,3),
                               status_code INTEGER,
                               response_headers JSONB,
                               response_body TEXT,
                               error_message TEXT,
                               created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                               CONSTRAINT fk_check_results_check FOREIGN KEY (check_id) REFERENCES checks(id) ON DELETE CASCADE
);

-- 10. incident_events - события инцидентов (зависит от incidents)
CREATE TABLE incident_events (
                                 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                 incident_id UUID NOT NULL,
                                 event_type VARCHAR(50) NOT NULL,
                                 description TEXT,
                                 data JSONB,
                                 created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                                 CONSTRAINT fk_incident_events_incident FOREIGN KEY (incident_id) REFERENCES incidents(id) ON DELETE CASCADE
);

-- 11. sessions - сессии пользователей (зависит от users)
CREATE TABLE sessions (
                          id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                          user_id UUID NOT NULL,
                          refresh_token_hash VARCHAR(255) NOT NULL,
                          expires_at TIMESTAMPTZ NOT NULL,
                          created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                          updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
                          CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Создание индексов для ускорения запросов

-- Индексы для таблицы users
CREATE INDEX idx_users_tenant_id ON users(tenant_id);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_is_active ON users(is_active);
CREATE UNIQUE INDEX idx_users_email_tenant ON users(email, tenant_id);

-- Индексы для таблицы checks
CREATE INDEX idx_checks_tenant_id ON checks(tenant_id);
CREATE INDEX idx_checks_type ON checks(type);
CREATE INDEX idx_checks_enabled ON checks(enabled);
CREATE INDEX idx_checks_created_at ON checks(created_at);

-- Индексы для таблицы api_keys
CREATE INDEX idx_api_keys_tenant_id ON api_keys(tenant_id);
CREATE INDEX idx_api_keys_is_active ON api_keys(is_active);
CREATE INDEX idx_api_keys_expires_at ON api_keys(expires_at);
CREATE INDEX idx_api_keys_key_prefix ON api_keys(key_prefix);

-- Индексы для таблицы notification_channels
CREATE INDEX idx_notification_channels_tenant_id ON notification_channels(tenant_id);
CREATE INDEX idx_notification_channels_type ON notification_channels(type);
CREATE INDEX idx_notification_channels_is_active ON notification_channels(is_active);

-- Индексы для таблицы proto_templates
CREATE INDEX idx_proto_templates_name ON proto_templates(name);
CREATE INDEX idx_proto_templates_version ON proto_templates(version);

-- Индексы для таблицы incidents
CREATE INDEX idx_incidents_check_id ON incidents(check_id);
CREATE INDEX idx_incidents_status ON incidents(status);
CREATE INDEX idx_incidents_severity ON incidents(severity);
CREATE INDEX idx_incidents_started_at ON incidents(started_at);
CREATE INDEX idx_incidents_resolved_at ON incidents(resolved_at);
CREATE INDEX idx_incidents_created_at ON incidents(created_at);

-- Индексы для таблицы schedules
CREATE INDEX idx_schedules_check_id ON schedules(check_id);
CREATE INDEX idx_schedules_status ON schedules(status);
CREATE INDEX idx_schedules_next_run_at ON schedules(next_run_at);
CREATE INDEX idx_schedules_last_run_at ON schedules(last_run_at);
CREATE INDEX idx_schedules_next_run_status ON schedules(next_run_at, status) WHERE status = 'pending';

-- Индексы для таблицы check_results
CREATE INDEX idx_check_results_check_id ON check_results(check_id);
CREATE INDEX idx_check_results_status ON check_results(status);
CREATE INDEX idx_check_results_created_at ON check_results(created_at);
CREATE INDEX idx_check_results_check_created ON check_results(check_id, created_at DESC);

-- Индексы для таблицы incident_events
CREATE INDEX idx_incident_events_incident_id ON incident_events(incident_id);
CREATE INDEX idx_incident_events_event_type ON incident_events(event_type);
CREATE INDEX idx_incident_events_created_at ON incident_events(created_at);

-- Индексы для таблицы sessions
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX idx_sessions_refresh_token_hash ON sessions(refresh_token_hash);

-- Индекс для уникального slug в tenants (уже есть как UNIQUE constraint)
-- Добавим индекс для других часто используемых полей
CREATE INDEX idx_tenants_status ON tenants(status);
CREATE INDEX idx_tenants_plan ON tenants(plan);

-- Создание триггеров для автоматического обновления updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
    RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Применение триггеров к таблицам с updated_at
CREATE TRIGGER update_tenants_updated_at BEFORE UPDATE ON tenants
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_checks_updated_at BEFORE UPDATE ON checks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_api_keys_updated_at BEFORE UPDATE ON api_keys
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_notification_channels_updated_at BEFORE UPDATE ON notification_channels
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_proto_templates_updated_at BEFORE UPDATE ON proto_templates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_incidents_updated_at BEFORE UPDATE ON incidents
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_schedules_updated_at BEFORE UPDATE ON schedules
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_sessions_updated_at BEFORE UPDATE ON sessions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();