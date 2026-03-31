-- Создание таблицы каналов уведомлений
CREATE TABLE IF NOT EXISTS notification_channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL CHECK (type IN ('email', 'slack', 'sms', 'webhook')),
    config JSONB DEFAULT '{}'::jsonb,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Индексы для оптимизации запросов
CREATE INDEX IF NOT EXISTS idx_notification_channels_tenant_id ON notification_channels(tenant_id);
CREATE INDEX IF NOT EXISTS idx_notification_channels_type ON notification_channels(type);
CREATE INDEX IF NOT EXISTS idx_notification_channels_enabled ON notification_channels(enabled);
CREATE INDEX IF NOT EXISTS idx_notification_channels_tenant_enabled ON notification_channels(tenant_id, enabled);

-- Триггер для обновления updated_at
CREATE OR REPLACE FUNCTION update_notification_channels_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER trigger_update_notification_channels_updated_at
    BEFORE UPDATE ON notification_channels
    FOR EACH ROW
    EXECUTE FUNCTION update_notification_channels_updated_at();

-- Комментарии
COMMENT ON TABLE notification_channels IS 'Каналы уведомлений для различных типов (email, slack, sms, webhook)';
COMMENT ON COLUMN notification_channels.id IS 'Уникальный идентификатор канала';
COMMENT ON COLUMN notification_channels.tenant_id IS 'ID арендатора';
COMMENT ON COLUMN notification_channels.name IS 'Название канала';
COMMENT ON COLUMN notification_channels.type IS 'Тип канала (email, slack, sms, webhook)';
COMMENT ON COLUMN notification_channels.config IS 'Конфигурация канала в формате JSON';
COMMENT ON COLUMN notification_channels.enabled IS 'Флаг активности канала';
COMMENT ON COLUMN notification_channels.created_at IS 'Время создания';
COMMENT ON COLUMN notification_channels.updated_at IS 'Время последнего обновления';
