package balancer

import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	"UptimePingPlatform/pkg/logger"
)

// HealthChecker проверяет доступность инстансов
type HealthChecker struct {
	conn     *grpc.ClientConn
	address  string
	log      logger.Logger
	mu       sync.RWMutex
	lastSeen time.Time
	healthy  bool
}

// NewHealthChecker создает новый HealthChecker
func NewHealthChecker(address string, log logger.Logger) *HealthChecker {
	return &HealthChecker{
		address: address,
		log:     log,
		healthy: false,
	}
}

// Start запускает проверку доступности
func (h *HealthChecker) Start(ctx context.Context) error {
	// Создаем соединение с использованием нового API
	conn, err := grpc.NewClient(h.address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		h.log.Error("Failed to create gRPC client", logger.String("address", h.address), logger.String("error", err.Error()))
		return err
	}
	h.conn = conn

	// Обновляем время последнего подключения и статус
	h.mu.Lock()
	h.lastSeen = time.Now()
	h.healthy = true
	h.mu.Unlock()

	// Запускаем проверку состояния
	go h.checkLoop(ctx)

	return nil
}

// checkLoop периодически проверяет состояние соединения
func (h *HealthChecker) checkLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := h.check(); err != nil {
				h.mu.Lock()
				h.healthy = false
				h.mu.Unlock()
				h.log.Warn("Health check failed", logger.String("address", h.address), logger.String("error", err.Error()))
			} else {
				h.mu.Lock()
				h.healthy = true
				h.mu.Unlock()
				h.log.Debug("Health check passed", logger.String("address", h.address))
			}
		}
	}
}

// check проверяет состояние соединения
func (h *HealthChecker) check() error {
	// Проверяем состояние соединения
	state := h.conn.GetState()

	// Если соединение готово, ничего не делаем
	if state == connectivity.Ready {
		h.mu.Lock()
		h.lastSeen = time.Now()
		h.mu.Unlock()
		return nil
	}

	// Если соединение не готово, пытаемся восстановить
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Ждем изменения состояния
	changed := h.conn.WaitForStateChange(ctx, state)
	if !changed {
		// Таймаут, считаем инстанс недоступным
		return context.DeadlineExceeded
	}

	// Проверяем новое состояние
	newState := h.conn.GetState()
	if newState == connectivity.Ready {
		h.mu.Lock()
		h.lastSeen = time.Now()
		if time.Since(h.lastSeen) > 30*time.Second {
			h.log.Info("Instance recovered", logger.String("address", h.address))
		}
		h.mu.Unlock()
		return nil
	}

	// Если состояние все еще не Ready, пытаемся инициировать подключение
	// Note: Connect() не возвращает ошибку, это асинхронная операция
	h.conn.Connect()

	// Ждем еще немного для проверки состояния
	time.Sleep(1 * time.Second)
	finalState := h.conn.GetState()

	if finalState == connectivity.Ready {
		h.mu.Lock()
		h.lastSeen = time.Now()
		h.mu.Unlock()
		return nil
	}

	return &ConnectivityError{State: finalState.String()}
}

// IsHealthy возвращает true, если инстанс доступен
func (h *HealthChecker) IsHealthy() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Проверяем флаг здоровья и время последнего подключения
	return h.healthy && time.Since(h.lastSeen) < 60*time.Second
}

// LastSeen возвращает время последнего успешного подключения
func (h *HealthChecker) LastSeen() time.Time {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastSeen
}

// Address возвращает адрес инстанса
func (h *HealthChecker) Address() string {
	return h.address
}

// Close закрывает соединение
func (h *HealthChecker) Close() error {
	if h.conn != nil {
		return h.conn.Close()
	}
	return nil
}

// ConnectivityError ошибка подключения
type ConnectivityError struct {
	State string
}

func (e *ConnectivityError) Error() string {
	return "connection not ready, state: " + e.State
}
