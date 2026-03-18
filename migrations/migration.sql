BEGIN;

-- Создание функции для автоматического обновления updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- 1. Таблица арендаторов (тенантов)
CREATE TABLE IF NOT EXISTS tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL UNIQUE,
    plan VARCHAR(50) DEFAULT 'free',
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    settings JSONB DEFAULT '{}'::jsonb
);

-- 2. Таблица пользователей (зависит от tenants)
CREATE TABLE IF NOT EXISTS users (
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

-- 3. Таблица проверок (зависит от tenants)
CREATE TABLE IF NOT EXISTS checks (
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

-- 4. Таблица API-ключей (зависит от tenants)
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(255) NOT NULL,
    key_prefix VARCHAR(16) NOT NULL,
    secret_hash VARCHAR(255) NOT NULL,
    permissions JSONB DEFAULT '0'::jsonb,
    is_active BOOLEAN DEFAULT true,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_api_keys_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE
);

-- 5. Таблица каналов уведомлений (зависит от tenants)
CREATE TABLE IF NOT EXISTS notification_channels (
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

-- 6. Таблица шаблонов протоколов (не зависит от других)
CREATE TABLE IF NOT EXISTS proto_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    content TEXT NOT NULL,
    version VARCHAR(20),
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- 7. Таблица инцидентов (зависит от checks)
CREATE TABLE IF NOT EXISTS incidents (
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

-- 8. Таблица расписаний (зависит от checks)
CREATE TABLE IF NOT EXISTS schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    check_id UUID NOT NULL,
    next_run_at TIMESTAMPTZ,
    last_run_at TIMESTAMPTZ,
    status VARCHAR(20) DEFAULT 'pending',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_schedules_check FOREIGN KEY (check_id) REFERENCES checks(id) ON DELETE CASCADE
);

-- 9. Таблица результатов проверок (зависит от checks)
CREATE TABLE IF NOT EXISTS check_results (
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

-- 10. Таблица событий инцидентов (зависит от incidents)
CREATE TABLE IF NOT EXISTS incident_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id UUID NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    description TEXT,
    data JSONB,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_incident_events_incident FOREIGN KEY (incident_id) REFERENCES incidents(id) ON DELETE CASCADE
);

-- 11. Таблица сессий пользователей (зависит от users)
CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    refresh_token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- 12. Таблица ролей
CREATE TABLE IF NOT EXISTS roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) NOT NULL UNIQUE,
    description TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 13. Таблица разрешений
CREATE TABLE IF NOT EXISTS permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,
    resource VARCHAR(100) NOT NULL,
    action VARCHAR(50) NOT NULL,
    description TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT unique_resource_action UNIQUE (resource, action)
);

-- 14. Таблица связи ролей и разрешений
CREATE TABLE IF NOT EXISTS role_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT unique_role_permission UNIQUE (role_id, permission_id)
);

-- 15. Таблица связи пользователей и ролей (с поддержкой старой миграции)
DO $$
BEGIN
    -- Если существует старая таблица user_roles, сохраняем данные
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'user_roles') THEN
        CREATE TEMP TABLE temp_user_roles_backup AS SELECT * FROM user_roles;
        DROP TABLE user_roles;
        RAISE NOTICE 'Старая таблица user_roles удалена, данные сохранены во временной таблице';
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS user_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    assigned_by UUID REFERENCES users(id),
    assigned_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT unique_user_role UNIQUE (user_id, role_id),
    CONSTRAINT check_expiry CHECK (expires_at IS NULL OR expires_at > assigned_at)
);

-- Восстановление данных из временной таблицы, если они были
DO $$
BEGIN
    IF EXISTS (SELECT FROM pg_tables WHERE tablename = 'temp_user_roles_backup') THEN
        IF EXISTS (SELECT FROM information_schema.columns WHERE table_name = 'temp_user_roles_backup' AND column_name = 'user_id')
           AND EXISTS (SELECT FROM information_schema.columns WHERE table_name = 'temp_user_roles_backup' AND column_name = 'role_id') THEN
            INSERT INTO user_roles (user_id, role_id, assigned_at, created_at, updated_at)
            SELECT user_id, role_id, COALESCE(created_at, NOW()), COALESCE(created_at, NOW()), COALESCE(updated_at, NOW())
            FROM temp_user_roles_backup;
            RAISE NOTICE 'Данные из старой таблицы user_roles восстановлены';
        END IF;
        DROP TABLE temp_user_roles_backup;
    END IF;
END $$;

-- 16. Добавление колонки cron_expression в таблицу schedules (если ещё не добавлена)
ALTER TABLE schedules ADD COLUMN IF NOT EXISTS cron_expression VARCHAR(100) NOT NULL DEFAULT '';
UPDATE schedules SET cron_expression = '0 * * * *' WHERE cron_expression = '';

-- Индексы для всех таблиц (с проверкой существования)

-- Индексы для tenants
CREATE INDEX IF NOT EXISTS idx_tenants_status ON tenants(status);
CREATE INDEX IF NOT EXISTS idx_tenants_plan ON tenants(plan);

-- Индексы для users
CREATE INDEX IF NOT EXISTS idx_users_tenant_id ON users(tenant_id);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_is_active ON users(is_active);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_tenant ON users(email, tenant_id);

-- Индексы для checks
CREATE INDEX IF NOT EXISTS idx_checks_tenant_id ON checks(tenant_id);
CREATE INDEX IF NOT EXISTS idx_checks_type ON checks(type);
CREATE INDEX IF NOT EXISTS idx_checks_enabled ON checks(enabled);
CREATE INDEX IF NOT EXISTS idx_checks_created_at ON checks(created_at);

-- Индексы для api_keys
CREATE INDEX IF NOT EXISTS idx_api_keys_tenant_id ON api_keys(tenant_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_is_active ON api_keys(is_active);
CREATE INDEX IF NOT EXISTS idx_api_keys_expires_at ON api_keys(expires_at);
CREATE INDEX IF NOT EXISTS idx_api_keys_key_prefix ON api_keys(key_prefix);

-- Индексы для notification_channels
CREATE INDEX IF NOT EXISTS idx_notification_channels_tenant_id ON notification_channels(tenant_id);
CREATE INDEX IF NOT EXISTS idx_notification_channels_type ON notification_channels(type);
CREATE INDEX IF NOT EXISTS idx_notification_channels_is_active ON notification_channels(is_active);

-- Индексы для proto_templates
CREATE INDEX IF NOT EXISTS idx_proto_templates_name ON proto_templates(name);
CREATE INDEX IF NOT EXISTS idx_proto_templates_version ON proto_templates(version);

-- Индексы для incidents
CREATE INDEX IF NOT EXISTS idx_incidents_check_id ON incidents(check_id);
CREATE INDEX IF NOT EXISTS idx_incidents_status ON incidents(status);
CREATE INDEX IF NOT EXISTS idx_incidents_severity ON incidents(severity);
CREATE INDEX IF NOT EXISTS idx_incidents_started_at ON incidents(started_at);
CREATE INDEX IF NOT EXISTS idx_incidents_resolved_at ON incidents(resolved_at);
CREATE INDEX IF NOT EXISTS idx_incidents_created_at ON incidents(created_at);

-- Индексы для schedules
CREATE INDEX IF NOT EXISTS idx_schedules_check_id ON schedules(check_id);
CREATE INDEX IF NOT EXISTS idx_schedules_status ON schedules(status);
CREATE INDEX IF NOT EXISTS idx_schedules_next_run_at ON schedules(next_run_at);
CREATE INDEX IF NOT EXISTS idx_schedules_last_run_at ON schedules(last_run_at);
CREATE INDEX IF NOT EXISTS idx_schedules_next_run_status ON schedules(next_run_at, status) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_schedules_cron_expression ON schedules(cron_expression);

-- Индексы для check_results
CREATE INDEX IF NOT EXISTS idx_check_results_check_id ON check_results(check_id);
CREATE INDEX IF NOT EXISTS idx_check_results_status ON check_results(status);
CREATE INDEX IF NOT EXISTS idx_check_results_created_at ON check_results(created_at);
CREATE INDEX IF NOT EXISTS idx_check_results_check_created ON check_results(check_id, created_at DESC);

-- Индексы для incident_events
CREATE INDEX IF NOT EXISTS idx_incident_events_incident_id ON incident_events(incident_id);
CREATE INDEX IF NOT EXISTS idx_incident_events_event_type ON incident_events(event_type);
CREATE INDEX IF NOT EXISTS idx_incident_events_created_at ON incident_events(created_at);

-- Индексы для sessions
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_sessions_refresh_token_hash ON sessions(refresh_token_hash);

-- Индексы для roles
CREATE INDEX IF NOT EXISTS idx_roles_name ON roles(name);
CREATE INDEX IF NOT EXISTS idx_roles_active ON roles(is_active);

-- Индексы для permissions
CREATE INDEX IF NOT EXISTS idx_permissions_resource_action ON permissions(resource, action);

-- Индексы для role_permissions
CREATE INDEX IF NOT EXISTS idx_role_permissions_role_id ON role_permissions(role_id);
CREATE INDEX IF NOT EXISTS idx_role_permissions_permission_id ON role_permissions(permission_id);

-- Индексы для user_roles
CREATE INDEX IF NOT EXISTS idx_user_roles_user_id ON user_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_role_id ON user_roles(role_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_active ON user_roles(is_active);
CREATE INDEX IF NOT EXISTS idx_user_roles_expires_at ON user_roles(expires_at);

-- Триггеры для автоматического обновления updated_at

-- Триггер для tenants
DROP TRIGGER IF EXISTS update_tenants_updated_at ON tenants;
CREATE TRIGGER update_tenants_updated_at
    BEFORE UPDATE ON tenants
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Триггер для users
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Триггер для checks
DROP TRIGGER IF EXISTS update_checks_updated_at ON checks;
CREATE TRIGGER update_checks_updated_at
    BEFORE UPDATE ON checks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Триггер для api_keys
DROP TRIGGER IF EXISTS update_api_keys_updated_at ON api_keys;
CREATE TRIGGER update_api_keys_updated_at
    BEFORE UPDATE ON api_keys
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Триггер для notification_channels
DROP TRIGGER IF EXISTS update_notification_channels_updated_at ON notification_channels;
CREATE TRIGGER update_notification_channels_updated_at
    BEFORE UPDATE ON notification_channels
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Триггер для proto_templates
DROP TRIGGER IF EXISTS update_proto_templates_updated_at ON proto_templates;
CREATE TRIGGER update_proto_templates_updated_at
    BEFORE UPDATE ON proto_templates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Триггер для incidents
DROP TRIGGER IF EXISTS update_incidents_updated_at ON incidents;
CREATE TRIGGER update_incidents_updated_at
    BEFORE UPDATE ON incidents
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Триггер для schedules
DROP TRIGGER IF EXISTS update_schedules_updated_at ON schedules;
CREATE TRIGGER update_schedules_updated_at
    BEFORE UPDATE ON schedules
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Триггер для sessions
DROP TRIGGER IF EXISTS update_sessions_updated_at ON sessions;
CREATE TRIGGER update_sessions_updated_at
    BEFORE UPDATE ON sessions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Триггер для roles
DROP TRIGGER IF EXISTS update_roles_updated_at ON roles;
CREATE TRIGGER update_roles_updated_at
    BEFORE UPDATE ON roles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Триггер для permissions
DROP TRIGGER IF EXISTS update_permissions_updated_at ON permissions;
CREATE TRIGGER update_permissions_updated_at
    BEFORE UPDATE ON permissions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Триггер для user_roles
DROP TRIGGER IF EXISTS update_user_roles_updated_at ON user_roles;
CREATE TRIGGER update_user_roles_updated_at
    BEFORE UPDATE ON user_roles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Представление для удобного получения разрешений пользователя
CREATE OR REPLACE VIEW user_permissions_view AS
SELECT
    u.id as user_id,
    u.email,
    r.id as role_id,
    r.name as role_name,
    p.id as permission_id,
    p.name as permission_name,
    p.resource,
    p.action,
    ur.assigned_at,
    ur.expires_at,
    ur.is_active as role_assignment_active,
    r.is_active as role_active,
    p.is_active as permission_active
FROM users u
JOIN user_roles ur ON u.id = ur.user_id
JOIN roles r ON ur.role_id = r.id
JOIN role_permissions rp ON r.id = rp.role_id
JOIN permissions p ON rp.permission_id = p.id
WHERE ur.is_active = true
  AND r.is_active = true
  AND p.is_active = true
  AND (ur.expires_at IS NULL OR ur.expires_at > NOW());

-- Вставка базовых ролей (если отсутствуют)
INSERT INTO roles (name, description) VALUES
    ('admin', 'Администратор системы - полный доступ ко всем функциям'),
    ('user', 'Обычный пользователь - базовый доступ'),
    ('viewer', 'Наблюдатель - только чтение')
ON CONFLICT (name) DO NOTHING;

-- Вставка базовых разрешений (если отсутствуют)
INSERT INTO permissions (name, resource, action, description) VALUES
    -- checks
    ('checks:read', 'checks', 'read', 'Просмотр проверок'),
    ('checks:write', 'checks', 'write', 'Создание и изменение проверок'),
    ('checks:delete', 'checks', 'delete', 'Удаление проверок'),
    ('checks:manage', 'checks', 'manage', 'Полное управление проверками'),
    -- users
    ('users:read', 'users', 'read', 'Просмотр пользователей'),
    ('users:write', 'users', 'write', 'Создание и изменение пользователей'),
    ('users:delete', 'users', 'delete', 'Удаление пользователей'),
    ('users:manage', 'users', 'manage', 'Полное управление пользователями'),
    -- incidents
    ('incidents:read', 'incidents', 'read', 'Просмотр инцидентов'),
    ('incidents:write', 'incidents', 'write', 'Создание и изменение инцидентов'),
    ('incidents:resolve', 'incidents', 'resolve', 'Разрешение инцидентов'),
    ('incidents:manage', 'incidents', 'manage', 'Полное управление инцидентами'),
    -- settings
    ('settings:read', 'settings', 'read', 'Просмотр настроек'),
    ('settings:write', 'settings', 'write', 'Изменение настроек'),
    ('settings:manage', 'settings', 'manage', 'Полное управление настройками'),
    -- metrics
    ('metrics:read', 'metrics', 'read', 'Просмотр метрик'),
    ('metrics:manage', 'metrics', 'manage', 'Полное управление метриками'),
    -- roles
    ('roles:read', 'roles', 'read', 'Просмотр ролей'),
    ('roles:write', 'roles', 'write', 'Создание и изменение ролей'),
    ('roles:assign', 'roles', 'assign', 'Назначение ролей пользователям'),
    ('roles:manage', 'roles', 'manage', 'Полное управление ролями')
ON CONFLICT (name) DO NOTHING;

-- Назначение разрешений ролям
DO $$
DECLARE
    admin_role_id UUID;
    user_role_id UUID;
    viewer_role_id UUID;
BEGIN
    SELECT id INTO admin_role_id FROM roles WHERE name = 'admin';
    SELECT id INTO user_role_id FROM roles WHERE name = 'user';
    SELECT id INTO viewer_role_id FROM roles WHERE name = 'viewer';

    -- Администратору — все разрешения
    INSERT INTO role_permissions (role_id, permission_id)
    SELECT admin_role_id, id FROM permissions
    ON CONFLICT DO NOTHING;

    -- Пользователю — базовые разрешения
    INSERT INTO role_permissions (role_id, permission_id)
    SELECT user_role_id, id FROM permissions
    WHERE name IN (
        'checks:read', 'checks:write', 'checks:delete',
        'incidents:read', 'incidents:write', 'incidents:resolve',
        'metrics:read'
    )
    ON CONFLICT DO NOTHING;

    -- Наблюдателю — только чтение
    INSERT INTO role_permissions (role_id, permission_id)
    SELECT viewer_role_id, id FROM permissions
    WHERE name IN ('checks:read', 'incidents:read', 'metrics:read')
    ON CONFLICT DO NOTHING;
END $$;

COMMIT;