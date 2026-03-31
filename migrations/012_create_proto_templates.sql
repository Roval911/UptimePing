-- Создание таблицы шаблонов протоколов
CREATE TABLE IF NOT EXISTS proto_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL CHECK (type IN ('http', 'tcp', 'icmp', 'grpc', 'graphql', 'script')),
    content TEXT NOT NULL,
    description TEXT,
    version VARCHAR(20) DEFAULT '1.0.0',
    tags JSONB DEFAULT '[]'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Индексы для оптимизации запросов
CREATE INDEX IF NOT EXISTS idx_proto_templates_tenant_id ON proto_templates(tenant_id);
CREATE INDEX IF NOT EXISTS idx_proto_templates_type ON proto_templates(type);
CREATE INDEX IF NOT EXISTS idx_proto_templates_name ON proto_templates(name);
CREATE INDEX IF NOT EXISTS idx_proto_templates_tenant_type ON proto_templates(tenant_id, type);
CREATE INDEX IF NOT EXISTS idx_proto_templates_tenant_name ON proto_templates(tenant_id, name);

-- GIN индекс для тегов
CREATE INDEX IF NOT EXISTS idx_proto_templates_tags ON proto_templates USING GIN(tags);

-- Триггер для обновления updated_at
CREATE OR REPLACE FUNCTION update_proto_templates_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER trigger_update_proto_templates_updated_at
    BEFORE UPDATE ON proto_templates
    FOR EACH ROW
    EXECUTE FUNCTION update_proto_templates_updated_at();

-- Уникальный индекс для имени шаблона в рамках tenant
CREATE UNIQUE INDEX IF NOT EXISTS idx_proto_templates_tenant_name_unique 
ON proto_templates(tenant_id, name);

-- Комментарии
COMMENT ON TABLE proto_templates IS 'Шаблоны протоколов и генерируемого кода';
COMMENT ON COLUMN proto_templates.id IS 'Уникальный идентификатор шаблона';
COMMENT ON COLUMN proto_templates.tenant_id IS 'ID арендатора';
COMMENT ON COLUMN proto_templates.name IS 'Название шаблона';
COMMENT ON COLUMN proto_templates.type IS 'Тип шаблона (http, tcp, icmp, grpc, graphql, script)';
COMMENT ON COLUMN proto_templates.content IS 'Содержимое шаблона';
COMMENT ON COLUMN proto_templates.description IS 'Описание шаблона';
COMMENT ON COLUMN proto_templates.version IS 'Версия шаблона';
COMMENT ON COLUMN proto_templates.tags IS 'Теги для категоризации шаблонов';
COMMENT ON COLUMN proto_templates.created_at IS 'Время создания';
COMMENT ON COLUMN proto_templates.updated_at IS 'Время последнего обновления';
