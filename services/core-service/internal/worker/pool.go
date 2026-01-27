package worker

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"UptimePingPlatform/services/core-service/internal/domain"
	"UptimePingPlatform/services/core-service/internal/logging"
	"UptimePingPlatform/services/core-service/internal/metrics"
	"UptimePingPlatform/services/core-service/internal/service/checker"
	"UptimePingPlatform/pkg/logger"
)

// Task представляет задачу для выполнения проверки
type Task struct {
	ID          string                 `json:"id"`
	CheckID     string                 `json:"check_id"`
	Target      string                 `json:"target"`
	Type        domain.TaskType        `json:"type"`
	Config      map[string]interface{} `json:"config"`
	ExecutionID string                 `json:"execution_id"`
	ScheduledTime time.Time            `json:"scheduled_time"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
	TenantID     string                 `json:"tenant_id"`
	Priority     int                    `json:"priority"`
	RetryCount   int                    `json:"retry_count"`
	MaxRetries   int                    `json:"max_retries"`
}

// TaskResult представляет результат выполнения задачи
type TaskResult struct {
	TaskID       string            `json:"task_id"`
	CheckID      string            `json:"check_id"`
	ExecutionID  string            `json:"execution_id"`
	Success      bool              `json:"success"`
	DurationMs   int64             `json:"duration_ms"`
	StatusCode   int               `json:"status_code,omitempty"`
	Error        string            `json:"error,omitempty"`
	ResponseBody string            `json:"response_body,omitempty"`
	CheckedAt    time.Time         `json:"checked_at"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	RetryCount   int               `json:"retry_count"`
	ShouldRetry  bool              `json:"should_retry"`
}

// Worker представляет рабочего для выполнения задач
type Worker struct {
	id        int
	taskChan  <-chan *Task
	resultChan chan<- *TaskResult
	quit      chan bool
	logger    *logging.UptimeLogger
	metrics   *metrics.UptimeMetrics
	checkers  map[domain.TaskType]checker.Checker
	pool      *Pool // Добавляем ссылку на пул для доступа к конфигурации
}

// Config конфигурация worker pool
type Config struct {
	// Количество рабочих
	WorkerCount int `json:"worker_count"`
	
	// Размер очереди задач
	QueueSize int `json:"queue_size"`
	
	// Таймауты для разных типов проверок
	Timeouts map[domain.TaskType]time.Duration `json:"timeouts"`
	
	// Настройки повторных попыток
	RetryConfig RetryConfig `json:"retry_config"`
	
	// Graceful shutdown таймаут
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
	
	// Максимальное количество одновременных проверок
	MaxConcurrentChecks int `json:"max_concurrent_checks"`
	
	// Интервал очистки статистики
	StatsCleanupInterval time.Duration `json:"stats_cleanup_interval"`
}

// RetryConfig конфигурация повторных попыток
type RetryConfig struct {
	MaxRetries      int           `json:"max_retries"`
	InitialDelay    time.Duration `json:"initial_delay"`
	MaxDelay        time.Duration `json:"max_delay"`
	RetryMultiplier float64       `json:"retry_multiplier"`
	RetryJitter     float64       `json:"retry_jitter"`
}

// DefaultConfig возвращает конфигурацию по умолчанию
func DefaultConfig() *Config {
	return &Config{
		WorkerCount:         10,
		QueueSize:           1000,
		ShutdownTimeout:     30 * time.Second,
		MaxConcurrentChecks: 100,
		StatsCleanupInterval: 1 * time.Minute,
		Timeouts: map[domain.TaskType]time.Duration{
			domain.TaskTypeHTTP:    30 * time.Second,
			domain.TaskTypeTCP:     10 * time.Second,
			domain.TaskTypeICMP:    5 * time.Second,
			domain.TaskTypeGRPC:    15 * time.Second,
			domain.TaskTypeGraphQL: 30 * time.Second,
		},
		RetryConfig: RetryConfig{
			MaxRetries:      3,
			InitialDelay:    1 * time.Second,
			MaxDelay:        30 * time.Second,
			RetryMultiplier: 2.0,
			RetryJitter:     0.1,
		},
	}
}

// Validate проверяет конфигурацию
func (c *Config) Validate() error {
	if c.WorkerCount <= 0 {
		return fmt.Errorf("worker count must be positive")
	}
	if c.QueueSize <= 0 {
		return fmt.Errorf("queue size must be positive")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("shutdown timeout must be positive")
	}
	if c.MaxConcurrentChecks <= 0 {
		return fmt.Errorf("max concurrent checks must be positive")
	}
	if c.StatsCleanupInterval <= 0 {
		return fmt.Errorf("stats cleanup interval must be positive")
	}
	
	// Проверяем таймауты
	for taskType, timeout := range c.Timeouts {
		if timeout <= 0 {
			return fmt.Errorf("timeout for %s must be positive", taskType)
		}
	}
	
	// Проверяем конфигурацию retry
	if c.RetryConfig.MaxRetries < 0 {
		return fmt.Errorf("max retries must be non-negative")
	}
	if c.RetryConfig.InitialDelay <= 0 {
		return fmt.Errorf("initial delay must be positive")
	}
	if c.RetryConfig.MaxDelay <= 0 {
		return fmt.Errorf("max delay must be positive")
	}
	if c.RetryConfig.RetryMultiplier <= 1.0 {
		return fmt.Errorf("retry multiplier must be greater than 1.0")
	}
	if c.RetryConfig.RetryJitter < 0 || c.RetryConfig.RetryJitter > 1.0 {
		return fmt.Errorf("retry jitter must be between 0 and 1")
	}
	
	return nil
}

// Pool представляет пул рабочих
type Pool struct {
	config     *Config
	workers    []*Worker
	taskChan   chan *Task
	resultChan chan *TaskResult
	quit       chan bool
	wg         sync.WaitGroup
	logger     *logging.UptimeLogger
	metrics    *metrics.UptimeMetrics
	checkers   map[domain.TaskType]checker.Checker
	
	// Статистика
	stats *PoolStats
	
	// Graceful shutdown
	shutdownInProgress int32
	shutdownComplete   chan struct{}
}

// PoolStats статистика пула
type PoolStats struct {
	TasksReceived    int64     `json:"tasks_received"`
	TasksCompleted   int64     `json:"tasks_completed"`
	TasksFailed      int64     `json:"tasks_failed"`
	TasksRetried     int64     `json:"tasks_retried"`
	ActiveWorkers    int64     `json:"active_workers"`
	QueueLength      int64     `json:"queue_length"`
	TotalDuration    int64     `json:"total_duration_ms"`
	AverageDuration  float64   `json:"average_duration_ms"`
	LastTaskTime     time.Time `json:"last_task_time"`
	
	// Для атомарного обновления AverageDuration
	averageDurationValue atomic.Value // хранит float64
}

// NewPool создает новый пул рабочих
func NewPool(config *Config, logger *logging.UptimeLogger, metrics *metrics.UptimeMetrics, checkers map[domain.TaskType]checker.Checker) (*Pool, error) {
	if config == nil {
		config = DefaultConfig()
	}
	
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	
	pool := &Pool{
		config:           config,
		taskChan:         make(chan *Task, config.QueueSize),
		resultChan:       make(chan *TaskResult, config.QueueSize),
		quit:             make(chan bool),
		logger:           logger,
		metrics:          metrics,
		checkers:         checkers,
		stats:            &PoolStats{},
		shutdownComplete: make(chan struct{}),
	}
	
	// Создаем рабочих
	for i := 0; i < config.WorkerCount; i++ {
		worker := &Worker{
			id:        i,
			taskChan:  pool.taskChan,
			resultChan: pool.resultChan,
			quit:      make(chan bool),
			logger:    logger.WithComponent(fmt.Sprintf("worker-%d", i)),
			metrics:   metrics,
			checkers:  checkers,
			pool:      pool, // Добавляем ссылку на пул
		}
		pool.workers = append(pool.workers, worker)
	}
	
	return pool, nil
}

// Start запускает пул рабочих
func (p *Pool) Start(ctx context.Context) error {
	p.logger.GetBaseLogger().Info("Starting worker pool",
		logger.Int("worker_count", p.config.WorkerCount),
		logger.Int("queue_size", p.config.QueueSize))
	
	// Запускаем рабочих
	for _, worker := range p.workers {
		p.wg.Add(1)
		go worker.start(&p.wg)
		atomic.AddInt64(&p.stats.ActiveWorkers, 1)
	}
	
	// Запускаем обработчик результатов
	p.wg.Add(1)
	go p.handleResults(ctx)
	
	// Запускаем очистку статистики
	p.wg.Add(1)
	go p.cleanupStats(ctx)
	
	return nil
}

// Stop останавливает пул рабочих с graceful shutdown
func (p *Pool) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&p.shutdownInProgress, 0, 1) {
		return nil // Уже останавливается
	}
	
	p.logger.GetBaseLogger().Info("Starting graceful shutdown of worker pool")
	
	// Создаем контекст с таймаутом
	shutdownCtx, cancel := context.WithTimeout(ctx, p.config.ShutdownTimeout)
	defer cancel()
	
	// Останавливаем прием новых задач
	close(p.taskChan)
	
	// Останавливаем рабочих
	for _, worker := range p.workers {
		close(worker.quit)
	}
	
	// Ждем завершения всех рабочих или таймаута
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		p.logger.GetBaseLogger().Info("All workers stopped gracefully")
	case <-shutdownCtx.Done():
		p.logger.GetBaseLogger().Warn("Shutdown timeout reached, forcing stop")
	}
	
	// Закрываем канал результатов
	close(p.resultChan)
	
	close(p.shutdownComplete)
	
	return nil
}

// SubmitTask отправляет задачу в пул
func (p *Pool) SubmitTask(ctx context.Context, task *Task) error {
	if atomic.LoadInt32(&p.shutdownInProgress) == 1 {
		return fmt.Errorf("pool is shutting down")
	}
	
	select {
	case p.taskChan <- task:
		atomic.AddInt64(&p.stats.TasksReceived, 1)
		p.stats.LastTaskTime = time.Now()
		p.logger.GetBaseLogger().Debug("Task submitted to pool",
			logger.String("task_id", task.ID),
			logger.String("check_id", task.CheckID),
			logger.String("check_type", string(task.Type)))
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("task queue is full")
	}
}

// SubmitTaskWithTimeout отправляет задачу с таймаутом
func (p *Pool) SubmitTaskWithTimeout(task *Task, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	return p.SubmitTask(ctx, task)
}

// GetStats возвращает статистику пула
func (p *Pool) GetStats() *PoolStats {
	stats := &PoolStats{
		TasksReceived:   atomic.LoadInt64(&p.stats.TasksReceived),
		TasksCompleted:  atomic.LoadInt64(&p.stats.TasksCompleted),
		TasksFailed:     atomic.LoadInt64(&p.stats.TasksFailed),
		TasksRetried:    atomic.LoadInt64(&p.stats.TasksRetried),
		ActiveWorkers:   atomic.LoadInt64(&p.stats.ActiveWorkers),
		QueueLength:     int64(len(p.taskChan)),
		TotalDuration:   atomic.LoadInt64(&p.stats.TotalDuration),
		AverageDuration: p.stats.AverageDuration,
		LastTaskTime:    p.stats.LastTaskTime,
	}
	
	// Вычисляем среднее время выполнения
	if stats.TasksCompleted > 0 {
		stats.AverageDuration = float64(stats.TotalDuration) / float64(stats.TasksCompleted)
	}
	
	return stats
}

// IsShutdownInProgress проверяет, идет ли процесс остановки
func (p *Pool) IsShutdownInProgress() bool {
	return atomic.LoadInt32(&p.shutdownInProgress) == 1
}

// WaitShutdownComplete ждет завершения shutdown
func (p *Pool) WaitShutdownComplete() <-chan struct{} {
	return p.shutdownComplete
}

// start запускает рабочего
func (w *Worker) start(wg *sync.WaitGroup) {
	defer wg.Done()
	// Worker не имеет доступа к статистике пула напрямую
	
	w.logger.GetBaseLogger().Info("Worker started",
		logger.Int("worker_id", w.id))
	
	for {
		select {
		case task := <-w.taskChan:
			if task != nil {
				w.processTask(task)
			}
		case <-w.quit:
			w.logger.GetBaseLogger().Info("Worker stopping",
				logger.Int("worker_id", w.id))
			return
		}
	}
}

// processTask обрабатывает задачу
func (w *Worker) processTask(task *Task) {
	ctx := logging.WithCheckContext(context.Background(), 
		logging.GenerateTraceID(), 
		task.CheckID, 
		task.ExecutionID, 
		task.TenantID)
	
	start := time.Now()
	
	w.logger.LogCheckStart(ctx, string(task.Type), task.Target, task.CheckID, task.ExecutionID)
	
	result := &TaskResult{
		TaskID:      task.ID,
		CheckID:     task.CheckID,
		ExecutionID: task.ExecutionID,
		CheckedAt:   time.Now(),
		RetryCount:  task.RetryCount,
	}
	
	// Получаем checker для типа задачи
	checker, exists := w.checkers[task.Type]
	if !exists {
		result.Success = false
		result.Error = fmt.Sprintf("checker not found for type %s", task.Type)
		result.ShouldRetry = false
	} else {
		// Выполняем проверку
		checkResult, err := checker.Execute(&domain.Task{
			ID:     task.ID,
			Type:   string(task.Type),
			Target: task.Target,
			Config: task.Config,
		})
		if err != nil {
			result.Success = false
			result.Error = err.Error()
			result.ShouldRetry = w.shouldRetry(err, task.RetryCount)
		} else {
			result.Success = checkResult.Success
			result.DurationMs = checkResult.DurationMs
			result.StatusCode = checkResult.StatusCode
			result.ResponseBody = checkResult.ResponseBody
			result.Metadata = checkResult.Metadata
			
			if !checkResult.Success {
				result.Error = checkResult.Error
				result.ShouldRetry = w.shouldRetryFromResult(checkResult, task.RetryCount)
			}
		}
	}
	
	duration := time.Since(start)
	result.DurationMs = duration.Milliseconds()
	
	// Обновляем статистику
	w.updateMetrics(task, result, duration)
	
	w.logger.LogCheckComplete(ctx, string(task.Type), task.Target, task.CheckID, task.ExecutionID, 
		duration, result.Success, result.StatusCode, int64(len(result.ResponseBody)))
	
	// Отправляем результат
	select {
	case w.resultChan <- result:
	default:
		w.logger.GetBaseLogger().Warn("Result channel is full, dropping result",
			logger.String("task_id", task.ID))
	}
}

// getTimeout возвращает таймаут для типа проверки
func (w *Worker) getTimeout(taskType domain.TaskType) time.Duration {
	// Получаем timeout из конфигурации пула
	if timeout, exists := w.pool.config.Timeouts[taskType]; exists {
		return timeout
	}
	
	// Если timeout не найден в конфигурации, используем значения по умолчанию
	defaultTimeouts := map[domain.TaskType]time.Duration{
		domain.TaskTypeHTTP:    30 * time.Second,
		domain.TaskTypeTCP:     10 * time.Second,
		domain.TaskTypeICMP:    5 * time.Second,
		domain.TaskTypeGRPC:    15 * time.Second,
		domain.TaskTypeGraphQL: 30 * time.Second,
	}
	
	if timeout, exists := defaultTimeouts[taskType]; exists {
		return timeout
	}
	return 30 * time.Second // по умолчанию
}

// shouldRetry определяет, нужно ли повторять попытку на основе ошибки
func (w *Worker) shouldRetry(err error, retryCount int) bool {
	// Используем конфигурацию retry из пула
	maxRetries := w.pool.config.RetryConfig.MaxRetries
	if retryCount >= maxRetries {
		return false
	}
	
	// Простая логика retry для определенных типов ошибок
	errStr := err.Error()
	if contains(errStr, "timeout") || contains(errStr, "connection") {
		return true
	}
	
	return false
}

// shouldRetryFromResult определяет, нужно ли повторять попытку на основе результата
func (w *Worker) shouldRetryFromResult(result *domain.CheckResult, retryCount int) bool {
	// Используем конфигурацию retry из пула
	maxRetries := w.pool.config.RetryConfig.MaxRetries
	if retryCount >= maxRetries {
		return false
	}
	
	// Retry для сетевых ошибок
	if result.StatusCode >= 500 || result.StatusCode == 0 {
		return true
	}
	
	return false
}

// updateMetrics обновляет метрики
func (w *Worker) updateMetrics(task *Task, result *TaskResult, duration time.Duration) {
	// Обновляем метрики uptime проверок
	w.metrics.RecordCheckResult(string(task.Type), task.Target, duration, result.Success, 
		int64(len(result.ResponseBody)), result.Error)
}

// handleResults обрабатывает результаты задач
func (p *Pool) handleResults(ctx context.Context) {
	defer p.wg.Done()
	
	for result := range p.resultChan {
		if result.Success {
			atomic.AddInt64(&p.stats.TasksCompleted, 1)
		} else {
			atomic.AddInt64(&p.stats.TasksFailed, 1)
			
			if result.ShouldRetry {
				atomic.AddInt64(&p.stats.TasksRetried, 1)
				// Отправляем задачу обратно в очередь для retry
				// Создаем новую задачу с увеличенным счетчиком повторов
				retryTask := &Task{
					ID:          result.TaskID,
					CheckID:     result.CheckID,
					ExecutionID: result.ExecutionID,
					Type:        "", // Будет заполнено из оригинальной задачи
					Target:      "", // Будет заполнено из оригинальной задачи
					Config:      nil, // Будет заполнено из оригинальной задачи
					ScheduledTime: time.Now().Add(p.calculateRetryDelay(result.RetryCount)),
					CreatedAt:     time.Now(),
					UpdatedAt:     time.Now(),
					TenantID:     "", // Будет заполнено из оригинальной задачи
					Priority:     0,  // Будет заполнено из оригинальной задачи
					RetryCount:   result.RetryCount + 1,
					MaxRetries:   p.config.RetryConfig.MaxRetries,
				}
				
				// Пытаемся отправить задачу обратно в очередь
				select {
				case p.taskChan <- retryTask:
					p.logger.GetBaseLogger().Debug("Task queued for retry",
						logger.String("task_id", result.TaskID),
						logger.String("check_id", result.CheckID),
						logger.Int("retry_count", result.RetryCount+1))
				default:
					p.logger.GetBaseLogger().Warn("Retry queue is full, dropping retry task",
						logger.String("task_id", result.TaskID),
						logger.String("check_id", result.CheckID))
				}
			}
		}
		
		atomic.AddInt64(&p.stats.TotalDuration, result.DurationMs)
	}
}

// calculateRetryDelay вычисляет задержку для retry с экспоненциальным backoff и jitter
func (p *Pool) calculateRetryDelay(retryCount int) time.Duration {
	config := p.config.RetryConfig
	
	// Экспоненциальный backoff: delay = initialDelay * multiplier^retryCount
	delay := float64(config.InitialDelay) * math.Pow(config.RetryMultiplier, float64(retryCount))
	
	// Ограничиваем максимальную задержку
	if delay > float64(config.MaxDelay) {
		delay = float64(config.MaxDelay)
	}
	
	// Добавляем jitter для предотвращения thundering herd
	if config.RetryJitter > 0 {
		jitter := delay * config.RetryJitter * (rand.Float64() - 0.5) // ±jitter/2
		delay += jitter
	}
	
	return time.Duration(delay)
}

// cleanupStats периодически очищает статистику
func (p *Pool) cleanupStats(ctx context.Context) {
	defer p.wg.Done()
	
	ticker := time.NewTicker(p.config.StatsCleanupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Реализуем очистку старой статистики
			p.cleanupOldStats()
		case <-ctx.Done():
			return
		case <-p.quit:
			return
		}
	}
}

// cleanupOldStats очищает старую статистику и пересчитывает средние значения
func (p *Pool) cleanupOldStats() {
	p.logger.GetBaseLogger().Debug("Cleaning up old statistics")
	
	// Сохраняем текущую статистику для логирования
	tasksCompleted := atomic.LoadInt64(&p.stats.TasksCompleted)
	tasksFailed := atomic.LoadInt64(&p.stats.TasksFailed)
	tasksRetried := atomic.LoadInt64(&p.stats.TasksRetried)
	queueLength := int64(len(p.taskChan))
	
	// Пересчитываем среднюю длительность
	totalDuration := atomic.LoadInt64(&p.stats.TotalDuration)
	totalTasks := tasksCompleted + tasksFailed
	
	if totalTasks > 0 {
		averageDuration := float64(totalDuration) / float64(totalTasks)
		p.stats.averageDurationValue.Store(averageDuration)
		p.stats.AverageDuration = averageDuration // Обновляем для JSON сериализации
	}
	
	// Обновляем длину очереди
	atomic.StoreInt64(&p.stats.QueueLength, queueLength)
	
	// Логируем статистику
	p.logger.GetBaseLogger().Info("Statistics cleanup completed",
		logger.Int64("tasks_completed", tasksCompleted),
		logger.Int64("tasks_failed", tasksFailed),
		logger.Int64("tasks_retried", tasksRetried),
		logger.Int64("queue_length", queueLength),
		logger.Float64("average_duration_ms", p.stats.AverageDuration))
	
	// Здесь можно добавить дополнительную логику:
	// - Сброс старых метрик в Prometheus
	// - Очистка кешей старых задач
	// - Архивация статистики в базу данных
}

// contains проверяет наличие подстроки без учета регистра
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && 
			(s[:len(substr)] == substr || 
			 s[len(s)-len(substr):] == substr || 
			 findSubstring(s, substr))))
}

// findSubstring ищет подстроку в строке
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
