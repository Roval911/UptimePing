-- Создание таблицы метрик
CREATE TABLE IF NOT EXISTS metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    check_id UUID REFERENCES checks(id) ON DELETE CASCADE,
    metric_type VARCHAR(50) NOT NULL CHECK (metric_type IN ('uptime', 'response_time', 'availability', 'error_rate', 'success_rate')),
    value DECIMAL(15,6) NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Индексы для оптимизации запросов
CREATE INDEX IF NOT EXISTS idx_metrics_tenant_id ON metrics(tenant_id);
CREATE INDEX IF NOT EXISTS idx_metrics_check_id ON metrics(check_id);
CREATE INDEX IF NOT EXISTS idx_metrics_type ON metrics(metric_type);
CREATE INDEX IF NOT EXISTS idx_metrics_timestamp ON metrics(timestamp);
CREATE INDEX IF NOT EXISTS idx_metrics_tenant_timestamp ON metrics(tenant_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_metrics_check_timestamp ON metrics(check_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_metrics_type_timestamp ON metrics(metric_type, timestamp);

-- Композитный индекс для типичных запросов
CREATE INDEX IF NOT EXISTS idx_metrics_tenant_check_timestamp ON metrics(tenant_id, check_id, timestamp);

-- Триггер для обновления updated_at
CREATE OR REPLACE FUNCTION update_metrics_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER trigger_update_metrics_updated_at
    BEFORE UPDATE ON metrics
    FOR EACH ROW
    EXECUTE FUNCTION update_metrics_updated_at();

-- Комментарии
COMMENT ON TABLE metrics IS 'Агрегированные метрики проверок';
COMMENT ON COLUMN metrics.id IS 'Уникальный идентификатор метрики';
COMMENT ON COLUMN metrics.tenant_id IS 'ID арендатора';
COMMENT ON COLUMN metrics.check_id IS 'ID проверки';
COMMENT ON COLUMN metrics.metric_type IS 'Тип метрики (uptime, response_time, availability, error_rate, success_rate)';
COMMENT ON COLUMN metrics.value IS 'Значение метрики';
COMMENT ON COLUMN metrics.timestamp IS 'Временная метка метрики';
COMMENT ON COLUMN metrics.metadata IS 'Дополнительные метаданные в формате JSON';
COMMENT ON COLUMN metrics.created_at IS 'Время создания';
COMMENT ON COLUMN metrics.updated_at IS 'Время последнего обновления';
