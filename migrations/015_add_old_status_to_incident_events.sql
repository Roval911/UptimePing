-- Добавление колонки old_status в таблицу incident_events
BEGIN;

-- Добавляем колонку old_status для отслеживания предыдущего статуса инцидента
ALTER TABLE incident_events 
ADD COLUMN old_status VARCHAR(20);

-- Добавляем комментарий для колонки
COMMENT ON COLUMN incident_events.old_status IS 'Previous status of the incident before this event';

-- Создаем индекс для новой колонки для оптимизации запросов
CREATE INDEX IF NOT EXISTS idx_incident_events_old_status ON incident_events(old_status);

COMMIT;
