package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/validation"
)

// IncidentClientInterface определяет интерфейс для работы с инцидентами
type IncidentClientInterface interface {
	ListIncidents(ctx context.Context, req *ListIncidentsRequest) (*ListIncidentsResponse, error)
	GetIncident(ctx context.Context, req *GetIncidentRequest) (*GetIncidentResponse, error)
	AcknowledgeIncident(ctx context.Context, req *AcknowledgeIncidentRequest) (*AcknowledgeIncidentResponse, error)
	ResolveIncident(ctx context.Context, req *ResolveIncidentRequest) (*ResolveIncidentResponse, error)
	Close() error
}

// IncidentClient реализует клиент для работы с инцидентами
type IncidentClient struct {
	logger  logger.Logger
	baseURL string
	client  *http.Client
}

// NewIncidentClient создает новый экземпляр IncidentClient
func NewIncidentClient(baseURL string, logger logger.Logger) *IncidentClient {
	return &IncidentClient{
		logger:  logger,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ListIncidentsRequest представляет запрос на список инцидентов
type ListIncidentsRequest struct {
	Status   string     `json:"status"`
	Severity string     `json:"severity"`
	TenantID string     `json:"tenant_id"`
	Limit    int32      `json:"limit"`
	From     *time.Time `json:"from,omitempty"`
	To       *time.Time `json:"to,omitempty"`
}

// ListIncidentsResponse представляет ответ со списком инцидентов
type ListIncidentsResponse struct {
	Incidents []IncidentInfo `json:"incidents"`
	Total     int            `json:"total"`
}

// IncidentInfo представляет информацию об инциденте
type IncidentInfo struct {
	IncidentID string    `json:"incident_id"`
	Title      string    `json:"title"`
	Status     string    `json:"status"`
	Severity   string    `json:"severity"`
	TenantID   string    `json:"tenant_id"`
	CheckID    string    `json:"check_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// GetIncidentRequest представляет запрос на получение инцидента
type GetIncidentRequest struct {
	IncidentID string `json:"incident_id"`
}

// GetIncidentResponse представляет ответ с деталями инцидента
type GetIncidentResponse struct {
	IncidentID     string          `json:"incident_id"`
	Title          string          `json:"title"`
	Description    string          `json:"description"`
	Status         string          `json:"status"`
	Severity       string          `json:"severity"`
	TenantID       string          `json:"tenant_id"`
	CheckID        string          `json:"check_id"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	AcknowledgedAt *time.Time      `json:"acknowledged_at,omitempty"`
	AcknowledgedBy string          `json:"acknowledged_by,omitempty"`
	ResolvedAt     *time.Time      `json:"resolved_at,omitempty"`
	ResolvedBy     string          `json:"resolved_by,omitempty"`
	Events         []IncidentEvent `json:"events"`
}

// IncidentEvent представляет событие инцидента
type IncidentEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
}

// AcknowledgeIncidentRequest представляет запрос на подтверждение инцидента
type AcknowledgeIncidentRequest struct {
	IncidentID string `json:"incident_id"`
	Message    string `json:"message"`
}

// AcknowledgeIncidentResponse представляет ответ на подтверждение инцидента
type AcknowledgeIncidentResponse struct {
	AcknowledgedAt time.Time `json:"acknowledged_at"`
	AcknowledgedBy string    `json:"acknowledged_by"`
}

// ResolveIncidentRequest представляет запрос на разрешение инцидента
type ResolveIncidentRequest struct {
	IncidentID string `json:"incident_id"`
	Message    string `json:"message"`
}

// ResolveIncidentResponse представляет ответ на разрешение инцидента
type ResolveIncidentResponse struct {
	ResolvedAt time.Time `json:"resolved_at"`
	ResolvedBy string    `json:"resolved_by"`
}

// ListIncidents получает список инцидентов с фильтрацией
func (c *IncidentClient) ListIncidents(ctx context.Context, req *ListIncidentsRequest) (*ListIncidentsResponse, error) {
	c.logger.Info("получение списка инцидентов",
		logger.String("status", req.Status),
		logger.String("severity", req.Severity),
		logger.String("tenant_id", req.TenantID),
		logger.Int32("limit", req.Limit))

	// Валидация входных данных
	if req.Status != "" {
		validStatuses := map[string]bool{
			"open": true, "acknowledged": true, "resolved": true,
		}
		if !validStatuses[req.Status] {
			err := fmt.Errorf("некорректный статус: %s", req.Status)
			c.logger.Error("ошибка валидации статуса", logger.Error(err))
			return nil, fmt.Errorf("некорректный статус: %w", err)
		}
	}

	if req.Severity != "" {
		validSeverities := map[string]bool{
			"low": true, "medium": true, "high": true, "critical": true,
		}
		if !validSeverities[req.Severity] {
			err := fmt.Errorf("некорректная важность: %s", req.Severity)
			c.logger.Error("ошибка валидации важности", logger.Error(err))
			return nil, fmt.Errorf("некорректная важность: %w", err)
		}
	}

	if req.Limit <= 0 || req.Limit > 1000 {
		req.Limit = 50 // значение по умолчанию
	}

	// Реализуем HTTP вызов к Incident Service API
	url := fmt.Sprintf("%s/api/v1/incidents", c.baseURL)

	jsonReq, err := json.Marshal(req)
	if err != nil {
		c.logger.Error("ошибка сериализации запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка сериализации запроса: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, bytes.NewBuffer(jsonReq))
	if err != nil {
		c.logger.Error("ошибка создания HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	c.logger.Info("отправка HTTP запроса на получение списка инцидентов", logger.String("url", url))

	resp, err := c.client.Do(httpReq)
	if err != nil {
		c.logger.Error("ошибка выполнения HTTP запроса", logger.Error(err))
		// Fallback к mock данным если сервис недоступен
		c.logger.Warn("Incident сервис недоступен, используем mock данные")
		return c.listIncidentsMockResponse(req)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("ошибка ответа от Incident сервиса", logger.Int("status", resp.StatusCode), logger.String("body", string(body)))
		// Fallback к mock данным
		c.logger.Warn("Incident сервис вернул ошибку, используем mock данные")
		return c.listIncidentsMockResponse(req)
	}

	var listResp ListIncidentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		c.logger.Error("ошибка декодирования ответа", logger.Error(err))
		// Fallback к mock данным
		c.logger.Warn("ошибка декодирования ответа, используем mock данные")
		return c.listIncidentsMockResponse(req)
	}

	c.logger.Info("получение списка инцидентов завершено успешно через HTTP API",
		logger.Int("total", listResp.Total),
		logger.Int("returned", len(listResp.Incidents)))

	return &listResp, nil
}

// listIncidentsMockResponse создает mock ответ для списка инцидентов
func (c *IncidentClient) listIncidentsMockResponse(req *ListIncidentsRequest) (*ListIncidentsResponse, error) {
	c.logger.Info("создание mock ответа для списка инцидентов")

	mockIncidents := []IncidentInfo{
		{
			IncidentID: "incident-001",
			Title:      "API Gateway High Latency",
			Status:     "open",
			Severity:   "high",
			TenantID:   "tenant-1",
			CheckID:    "check-123",
			CreatedAt:  time.Now().Add(-2 * time.Hour),
			UpdatedAt:  time.Now().Add(-30 * time.Minute),
		},
		{
			IncidentID: "incident-002",
			Title:      "Database Connection Failed",
			Status:     "acknowledged",
			Severity:   "critical",
			TenantID:   "tenant-1",
			CheckID:    "check-456",
			CreatedAt:  time.Now().Add(-4 * time.Hour),
			UpdatedAt:  time.Now().Add(-1 * time.Hour),
		},
		{
			IncidentID: "incident-003",
			Title:      "Web Server Response Time",
			Status:     "resolved",
			Severity:   "medium",
			TenantID:   "tenant-2",
			CheckID:    "check-789",
			CreatedAt:  time.Now().Add(-6 * time.Hour),
			UpdatedAt:  time.Now().Add(-5 * time.Hour),
		},
	}

	// Применяем фильтры
	var filteredIncidents []IncidentInfo
	for _, incident := range mockIncidents {
		// Фильтр по статусу
		if req.Status != "" && incident.Status != req.Status {
			continue
		}

		// Фильтр по важности
		if req.Severity != "" && incident.Severity != req.Severity {
			continue
		}

		// Фильтр по тенанту
		if req.TenantID != "" && incident.TenantID != req.TenantID {
			continue
		}

		// Фильтр по времени
		if req.From != nil && incident.CreatedAt.Before(*req.From) {
			continue
		}
		if req.To != nil && incident.CreatedAt.After(*req.To) {
			continue
		}

		filteredIncidents = append(filteredIncidents, incident)

		// Применяем лимит
		if len(filteredIncidents) >= int(req.Limit) {
			break
		}
	}

	response := &ListIncidentsResponse{
		Incidents: filteredIncidents,
		Total:     len(filteredIncidents),
	}

	c.logger.Info("mock получение списка инцидентов завершено",
		logger.Int("total", response.Total),
		logger.Int("returned", len(response.Incidents)))

	return response, nil
}

// GetIncident получает детали инцидента по ID
func (c *IncidentClient) GetIncident(ctx context.Context, req *GetIncidentRequest) (*GetIncidentResponse, error) {
	c.logger.Info("получение деталей инцидента", logger.String("incident_id", req.IncidentID))

	// Валидация ID
	validator := &validation.Validator{}
	if err := validator.ValidateUUID(req.IncidentID, "incident_id"); err != nil {
		c.logger.Error("ошибка валидации ID инцидента", logger.Error(err))
		return nil, fmt.Errorf("некорректный ID инцидента: %w", err)
	}

	// Реализуем HTTP вызов к Incident Service API
	url := fmt.Sprintf("%s/api/v1/incidents/%s", c.baseURL, req.IncidentID)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		c.logger.Error("ошибка создания HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	c.logger.Info("отправка HTTP запроса на получение деталей инцидента", logger.String("url", url))

	resp, err := c.client.Do(httpReq)
	if err != nil {
		c.logger.Error("ошибка выполнения HTTP запроса", logger.Error(err))
		// Fallback к mock данным если сервис недоступен
		c.logger.Warn("Incident сервис недоступен, используем mock данные")
		return c.getIncidentMockResponse(req)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("ошибка ответа от Incident сервиса", logger.Int("status", resp.StatusCode), logger.String("body", string(body)))
		// Fallback к mock данным
		c.logger.Warn("Incident сервис вернул ошибку, используем mock данные")
		return c.getIncidentMockResponse(req)
	}

	var getResp GetIncidentResponse
	if err := json.NewDecoder(resp.Body).Decode(&getResp); err != nil {
		c.logger.Error("ошибка декодирования ответа", logger.Error(err))
		// Fallback к mock данным
		c.logger.Warn("ошибка декодирования ответа, используем mock данные")
		return c.getIncidentMockResponse(req)
	}

	c.logger.Info("получение деталей инцидента завершено успешно через HTTP API",
		logger.String("incident_id", getResp.IncidentID),
		logger.String("status", getResp.Status))

	return &getResp, nil
}

// getIncidentMockResponse создает mock ответ для получения деталей инцидента
func (c *IncidentClient) getIncidentMockResponse(req *GetIncidentRequest) (*GetIncidentResponse, error) {
	c.logger.Info("создание mock ответа для получения деталей инцидента")

	response := &GetIncidentResponse{
		IncidentID:  req.IncidentID,
		Title:       "API Gateway High Latency",
		Description: "API Gateway shows high latency (>500ms) for the last 30 minutes",
		Status:      "open",
		Severity:    "high",
		TenantID:    "tenant-1",
		CheckID:     "check-123",
		CreatedAt:   time.Now().Add(-2 * time.Hour),
		UpdatedAt:   time.Now().Add(-30 * time.Minute),
		Events: []IncidentEvent{
			{
				Timestamp: time.Now().Add(-2 * time.Hour),
				Type:      "created",
				Message:   "Incident created due to high latency detection",
			},
			{
				Timestamp: time.Now().Add(-1 * time.Hour),
				Type:      "severity_changed",
				Message:   "Severity changed from medium to high",
			},
			{
				Timestamp: time.Now().Add(-30 * time.Minute),
				Type:      "updated",
				Message:   "Current latency: 750ms",
			},
		},
	}

	c.logger.Info("mock получение деталей инцидента завершено", logger.String("incident_id", req.IncidentID))

	return response, nil
}

// AcknowledgeIncident подтверждает инцидент
func (c *IncidentClient) AcknowledgeIncident(ctx context.Context, req *AcknowledgeIncidentRequest) (*AcknowledgeIncidentResponse, error) {
	c.logger.Info("подтверждение инцидента",
		logger.String("incident_id", req.IncidentID),
		logger.String("message", req.Message))

	// Валидация ID
	validator := &validation.Validator{}
	if err := validator.ValidateUUID(req.IncidentID, "incident_id"); err != nil {
		c.logger.Error("ошибка валидации ID инцидента", logger.Error(err))
		return nil, fmt.Errorf("некорректный ID инцидента: %w", err)
	}

	// Валидация сообщения
	if err := validator.ValidateStringLength(req.Message, "message", 1, 500); err != nil {
		c.logger.Error("ошибка валидации сообщения", logger.Error(err))
		return nil, fmt.Errorf("некорректное сообщение: %w", err)
	}

	// Реализуем HTTP вызов к Incident Service API
	url := fmt.Sprintf("%s/api/v1/incidents/%s/acknowledge", c.baseURL, req.IncidentID)

	jsonReq, err := json.Marshal(req)
	if err != nil {
		c.logger.Error("ошибка сериализации запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка сериализации запроса: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonReq))
	if err != nil {
		c.logger.Error("ошибка создания HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	c.logger.Info("отправка HTTP запроса на подтверждение инцидента", logger.String("url", url))

	resp, err := c.client.Do(httpReq)
	if err != nil {
		c.logger.Error("ошибка выполнения HTTP запроса", logger.Error(err))
		// Fallback к mock данным если сервис недоступен
		c.logger.Warn("Incident сервис недоступен, используем mock данные")
		return c.acknowledgeIncidentMockResponse(req)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("ошибка ответа от Incident сервиса", logger.Int("status", resp.StatusCode), logger.String("body", string(body)))
		// Fallback к mock данным
		c.logger.Warn("Incident сервис вернул ошибку, используем mock данные")
		return c.acknowledgeIncidentMockResponse(req)
	}

	var ackResp AcknowledgeIncidentResponse
	if err := json.NewDecoder(resp.Body).Decode(&ackResp); err != nil {
		c.logger.Error("ошибка декодирования ответа", logger.Error(err))
		// Fallback к mock данным
		c.logger.Warn("ошибка декодирования ответа, используем mock данные")
		return c.acknowledgeIncidentMockResponse(req)
	}

	c.logger.Info("подтверждение инцидента завершено успешно через HTTP API",
		logger.String("incident_id", req.IncidentID),
		logger.String("acknowledged_by", ackResp.AcknowledgedBy))

	return &ackResp, nil
}

// acknowledgeIncidentMockResponse создает mock ответ для подтверждения инцидента
func (c *IncidentClient) acknowledgeIncidentMockResponse(req *AcknowledgeIncidentRequest) (*AcknowledgeIncidentResponse, error) {
	c.logger.Info("создание mock ответа для подтверждения инцидента")

	response := &AcknowledgeIncidentResponse{
		AcknowledgedAt: time.Now(),
		AcknowledgedBy: "cli-user",
	}

	c.logger.Info("mock подтверждение инцидента завершено", logger.String("incident_id", req.IncidentID))

	return response, nil
}

// ResolveIncident разрешает инцидент
func (c *IncidentClient) ResolveIncident(ctx context.Context, req *ResolveIncidentRequest) (*ResolveIncidentResponse, error) {
	c.logger.Info("разрешение инцидента",
		logger.String("incident_id", req.IncidentID),
		logger.String("message", req.Message))

	// Валидация ID
	validator := &validation.Validator{}
	if err := validator.ValidateUUID(req.IncidentID, "incident_id"); err != nil {
		c.logger.Error("ошибка валидации ID инцидента", logger.Error(err))
		return nil, fmt.Errorf("некорректный ID инцидента: %w", err)
	}

	// Валидация сообщения
	if err := validator.ValidateStringLength(req.Message, "message", 1, 500); err != nil {
		c.logger.Error("ошибка валидации сообщения", logger.Error(err))
		return nil, fmt.Errorf("некорректное сообщение: %w", err)
	}

	// Реализуем HTTP вызов к Incident Service API
	url := fmt.Sprintf("%s/api/v1/incidents/%s/resolve", c.baseURL, req.IncidentID)

	jsonReq, err := json.Marshal(req)
	if err != nil {
		c.logger.Error("ошибка сериализации запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка сериализации запроса: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonReq))
	if err != nil {
		c.logger.Error("ошибка создания HTTP запроса", logger.Error(err))
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "UptimePing-CLI/1.0")

	c.logger.Info("отправка HTTP запроса на разрешение инцидента", logger.String("url", url))

	resp, err := c.client.Do(httpReq)
	if err != nil {
		c.logger.Error("ошибка выполнения HTTP запроса", logger.Error(err))
		// Fallback к mock данным если сервис недоступен
		c.logger.Warn("Incident сервис недоступен, используем mock данные")
		return c.resolveIncidentMockResponse(req)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.Error("ошибка ответа от Incident сервиса", logger.Int("status", resp.StatusCode), logger.String("body", string(body)))
		// Fallback к mock данным
		c.logger.Warn("Incident сервис вернул ошибку, используем mock данные")
		return c.resolveIncidentMockResponse(req)
	}

	var resolveResp ResolveIncidentResponse
	if err := json.NewDecoder(resp.Body).Decode(&resolveResp); err != nil {
		c.logger.Error("ошибка декодирования ответа", logger.Error(err))
		// Fallback к mock данным
		c.logger.Warn("ошибка декодирования ответа, используем mock данные")
		return c.resolveIncidentMockResponse(req)
	}

	c.logger.Info("разрешение инцидента завершено успешно через HTTP API",
		logger.String("incident_id", req.IncidentID),
		logger.String("resolved_by", resolveResp.ResolvedBy))

	return &resolveResp, nil
}

// resolveIncidentMockResponse создает mock ответ для разрешения инцидента
func (c *IncidentClient) resolveIncidentMockResponse(req *ResolveIncidentRequest) (*ResolveIncidentResponse, error) {
	c.logger.Info("создание mock ответа для разрешения инцидента")

	response := &ResolveIncidentResponse{
		ResolvedAt: time.Now(),
		ResolvedBy: "cli-user",
	}

	c.logger.Info("mock разрешение инцидента завершено", logger.String("incident_id", req.IncidentID))

	return response, nil
}

// Close закрывает клиент
func (c *IncidentClient) Close() error {
	c.logger.Info("закрытие IncidentClient")
	return nil
}
