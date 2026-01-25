-- Создание таблицы задач
CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    check_id UUID NOT NULL,
    scheduled_time TIMESTAMP WITH TIME ZONE NOT NULL,
    priority INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    
    CONSTRAINT fk_tasks_check_id 
        FOREIGN KEY (check_id) REFERENCES checks(id) ON DELETE CASCADE
);

-- Создание индексов для таблицы tasks
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_scheduled_time ON tasks(scheduled_time);
CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority DESC);
CREATE INDEX IF NOT EXISTS idx_tasks_check_id ON tasks(check_id);

-- Создание таблицы результатов выполнения задач
CREATE TABLE IF NOT EXISTS task_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL,
    check_id UUID NOT NULL,
    status VARCHAR(20) NOT NULL,
    error_message TEXT,
    duration_ms BIGINT NOT NULL,
    completed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    
    CONSTRAINT fk_task_results_task_id 
        FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    CONSTRAINT fk_task_results_check_id 
        FOREIGN KEY (check_id) REFERENCES checks(id) ON DELETE CASCADE
);

-- Создание индексов для таблицы task_results
CREATE INDEX IF NOT EXISTS idx_task_results_task_id ON task_results(task_id);
CREATE INDEX IF NOT EXISTS idx_task_results_check_id ON task_results(check_id);
CREATE INDEX IF NOT EXISTS idx_task_results_status ON task_results(status);
CREATE INDEX IF NOT EXISTS idx_task_results_completed_at ON task_results(completed_at);

-- Создание триггера для обновления updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_tasks_updated_at 
    BEFORE UPDATE ON tasks 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_task_results_updated_at 
    BEFORE UPDATE ON task_results 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Добавление комментариев к таблицам
COMMENT ON TABLE tasks IS 'Таблица задач для выполнения проверок';
COMMENT ON TABLE task_results IS 'Таблица результатов выполнения задач';

-- Добавление комментариев к колонкам
COMMENT ON COLUMN tasks.status IS 'Статус задачи: pending, running, completed, failed';
COMMENT ON COLUMN task_results.status IS 'Статус выполнения: pending, running, completed, failed';
COMMENT ON COLUMN task_results.duration_ms IS 'Длительность выполнения в миллисекундах';
