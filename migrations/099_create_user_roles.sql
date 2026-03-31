-- roles_permissions_schema.sql
-- Схема управления ролями и разрешениями для UptimePing Platform
-- Заменяет существующую таблицу user_roles на расширенную версию

BEGIN;

-- 1. Создание таблицы ролей
CREATE TABLE IF NOT EXISTS roles (
                                     id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                     name VARCHAR(50) NOT NULL UNIQUE,
                                     description TEXT,
                                     is_active BOOLEAN DEFAULT true,
                                     created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
                                     updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

COMMENT ON TABLE roles IS 'Таблица ролей пользователей';
COMMENT ON COLUMN roles.name IS 'Название роли (уникальное)';
COMMENT ON COLUMN roles.description IS 'Описание роли';
COMMENT ON COLUMN roles.is_active IS 'Активна ли роль';

-- 2. Создание таблицы разрешений
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

COMMENT ON TABLE permissions IS 'Таблица разрешений (permissions)';
COMMENT ON COLUMN permissions.name IS 'Уникальное название разрешения';
COMMENT ON COLUMN permissions.resource IS 'Ресурс (например: checks, users, settings)';
COMMENT ON COLUMN permissions.action IS 'Действие (например: read, write, delete, manage)';

-- 3. Создание таблицы связи ролей и разрешений
CREATE TABLE IF NOT EXISTS role_permissions (
                                                id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                                                role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
                                                permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
                                                created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
                                                CONSTRAINT unique_role_permission UNIQUE (role_id, permission_id)
);

COMMENT ON TABLE role_permissions IS 'Связь ролей и разрешений (многие-ко-многим)';

-- 4. Замена существующей таблицы user_roles
-- Сохраняем данные из старой таблицы (если нужно)
DO $$
    BEGIN
        -- Проверяем, существует ли старая таблица user_roles
        IF EXISTS (SELECT FROM information_schema.tables
                   WHERE table_schema = 'public'
                     AND table_name = 'user_roles') THEN
            -- Создаем временную таблицу для сохранения данных
            CREATE TEMP TABLE temp_user_roles_backup AS
            SELECT * FROM user_roles;

            -- Удаляем старую таблицу
            DROP TABLE user_roles;

            RAISE NOTICE 'Старая таблица user_roles удалена, данные сохранены во временной таблице';
        ELSE
            RAISE NOTICE 'Таблица user_roles не существует, создаем новую';
        END IF;
    END $$;

-- 5. Создание новой таблицы user_roles
CREATE TABLE user_roles (
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

COMMENT ON TABLE user_roles IS 'Связь пользователей и ролей';
COMMENT ON COLUMN user_roles.user_id IS 'ID пользователя';
COMMENT ON COLUMN user_roles.role_id IS 'ID роли';
COMMENT ON COLUMN user_roles.assigned_by IS 'Кто назначил роль (ID пользователя)';
COMMENT ON COLUMN user_roles.assigned_at IS 'Когда назначена роль';
COMMENT ON COLUMN user_roles.expires_at IS 'Когда истекает роль (NULL - бессрочно)';
COMMENT ON COLUMN user_roles.is_active IS 'Активна ли связь';

-- 6. Восстановление данных из временной таблицы (если нужно)
DO $$
    BEGIN
        IF EXISTS (SELECT FROM pg_tables WHERE tablename = 'temp_user_roles_backup') THEN
            -- Проверяем структуру временной таблицы
            IF EXISTS (
                SELECT FROM information_schema.columns
                WHERE table_name = 'temp_user_roles_backup'
                  AND column_name = 'user_id'
            ) AND EXISTS (
                SELECT FROM information_schema.columns
                WHERE table_name = 'temp_user_roles_backup'
                  AND column_name = 'role_id'
            ) THEN
                -- Вставляем данные в новую таблицу
                INSERT INTO user_roles (user_id, role_id, assigned_at, created_at, updated_at)
                SELECT
                    user_id,
                    role_id,
                    COALESCE(created_at, NOW()),
                    COALESCE(created_at, NOW()),
                    COALESCE(updated_at, NOW())
                FROM temp_user_roles_backup;

                RAISE NOTICE 'Данные из старой таблицы user_roles восстановлены';
            ELSE
                RAISE WARNING 'Не удалось восстановить данные: несовместимая структура временной таблицы';
            END IF;

            -- Удаляем временную таблицу
            DROP TABLE temp_user_roles_backup;
        END IF;
    END $$;

-- 7. Создание индексов для улучшения производительности
CREATE INDEX IF NOT EXISTS idx_roles_name ON roles(name);
CREATE INDEX IF NOT EXISTS idx_roles_active ON roles(is_active);
CREATE INDEX IF NOT EXISTS idx_permissions_resource_action ON permissions(resource, action);
CREATE INDEX IF NOT EXISTS idx_role_permissions_role_id ON role_permissions(role_id);
CREATE INDEX IF NOT EXISTS idx_role_permissions_permission_id ON role_permissions(permission_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_user_id ON user_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_role_id ON user_roles(role_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_active ON user_roles(is_active);
CREATE INDEX IF NOT EXISTS idx_user_roles_expires_at ON user_roles(expires_at);

-- 8. Создание триггера для автоматического обновления updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
    RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_roles_updated_at
    BEFORE UPDATE ON roles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_permissions_updated_at
    BEFORE UPDATE ON permissions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_user_roles_updated_at
    BEFORE UPDATE ON user_roles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 9. Создание представлений для удобства
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

COMMENT ON VIEW user_permissions_view IS 'Представление для получения всех активных разрешений пользователя';

-- 10. Вставка базовых данных (опционально)
-- Базовые роли
INSERT INTO roles (name, description) VALUES
                                          ('admin', 'Администратор системы - полный доступ ко всем функциям'),
                                          ('user', 'Обычный пользователь - базовый доступ'),
                                          ('viewer', 'Наблюдатель - только чтение')
ON CONFLICT (name) DO NOTHING;

-- Базовые разрешения
INSERT INTO permissions (name, resource, action, description) VALUES
                                                                  -- Разрешения для проверок (checks)
                                                                  ('checks:read', 'checks', 'read', 'Просмотр проверок'),
                                                                  ('checks:write', 'checks', 'write', 'Создание и изменение проверок'),
                                                                  ('checks:delete', 'checks', 'delete', 'Удаление проверок'),
                                                                  ('checks:manage', 'checks', 'manage', 'Полное управление проверками'),

                                                                  -- Разрешения для пользователей (users)
                                                                  ('users:read', 'users', 'read', 'Просмотр пользователей'),
                                                                  ('users:write', 'users', 'write', 'Создание и изменение пользователей'),
                                                                  ('users:delete', 'users', 'delete', 'Удаление пользователей'),
                                                                  ('users:manage', 'users', 'manage', 'Полное управление пользователями'),

                                                                  -- Разрешения для инцидентов (incidents)
                                                                  ('incidents:read', 'incidents', 'read', 'Просмотр инцидентов'),
                                                                  ('incidents:write', 'incidents', 'write', 'Создание и изменение инцидентов'),
                                                                  ('incidents:resolve', 'incidents', 'resolve', 'Разрешение инцидентов'),
                                                                  ('incidents:manage', 'incidents', 'manage', 'Полное управление инцидентами'),

                                                                  -- Разрешения для настроек (settings)
                                                                  ('settings:read', 'settings', 'read', 'Просмотр настроек'),
                                                                  ('settings:write', 'settings', 'write', 'Изменение настроек'),
                                                                  ('settings:manage', 'settings', 'manage', 'Полное управление настройками'),

                                                                  -- Разрешения для метрик (metrics)
                                                                  ('metrics:read', 'metrics', 'read', 'Просмотр метрик'),
                                                                  ('metrics:manage', 'metrics', 'manage', 'Полное управление метриками'),

                                                                  -- Разрешения для ролей (roles)
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
        -- Получаем ID ролей
        SELECT id INTO admin_role_id FROM roles WHERE name = 'admin';
        SELECT id INTO user_role_id FROM roles WHERE name = 'user';
        SELECT id INTO viewer_role_id FROM roles WHERE name = 'viewer';

        -- Администратору - все разрешения
        INSERT INTO role_permissions (role_id, permission_id)
        SELECT admin_role_id, id FROM permissions
        ON CONFLICT DO NOTHING;

        -- Пользователю - базовые разрешения
        INSERT INTO role_permissions (role_id, permission_id)
        SELECT user_role_id, id FROM permissions
        WHERE name IN (
                       'checks:read', 'checks:write', 'checks:delete',
                       'incidents:read', 'incidents:write', 'incidents:resolve',
                       'metrics:read'
            )
        ON CONFLICT DO NOTHING;

        -- Наблюдателю - только чтение
        INSERT INTO role_permissions (role_id, permission_id)
        SELECT viewer_role_id, id FROM permissions
        WHERE name IN ('checks:read', 'incidents:read', 'metrics:read')
        ON CONFLICT DO NOTHING;
    END $$;

COMMIT;

-- Информационное сообщение
DO $$
    BEGIN
        RAISE NOTICE 'Схема управления ролями успешно создана!';
        RAISE NOTICE 'Созданы таблицы: roles, permissions, role_permissions, user_roles';
        RAISE NOTICE 'Созданы базовые роли: admin, user, viewer';
        RAISE NOTICE 'Созданы индексы и триггеры для обновления updated_at';
        RAISE NOTICE 'Таблица user_roles заменена на расширенную версию';
    END $$;