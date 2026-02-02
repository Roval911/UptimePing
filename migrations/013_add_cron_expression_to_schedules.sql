-- Add cron_expression column to schedules table
ALTER TABLE schedules 
ADD COLUMN cron_expression VARCHAR(100) NOT NULL DEFAULT '';

-- Add index for better performance
CREATE INDEX idx_schedules_cron_expression ON schedules(cron_expression);

-- Update existing schedules with default cron expression
UPDATE schedules SET cron_expression = '0 * * * *' WHERE cron_expression = '';
