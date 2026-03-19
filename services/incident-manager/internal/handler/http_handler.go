package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"UptimePingPlatform/pkg/errors"
	pkglogger "UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/pkg/validation"
	"UptimePingPlatform/services/incident-manager/internal/domain"
	"UptimePingPlatform/services/incident-manager/internal/service"
)

// HTTPHandler обрабатывает HTTP запросы для Incident Manager
type HTTPHandler struct {
	logger          pkglogger.Logger
	incidentService service.IncidentService
	validator       *validation.Validator
}

// NewHTTPHandler создает новый HTTP обработчик
func NewHTTPHandler(logger pkglogger.Logger, incidentService service.IncidentService) *HTTPHandler {
	return &HTTPHandler{
		logger:          logger,
		incidentService: incidentService,
		validator:       &validation.Validator{},
	}
}

// RegisterRoutes регистрирует HTTP маршруты
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	// API маршруты для инцидентов
	mux.HandleFunc("/api/v1/incidents", h.handleIncidents)
	mux.HandleFunc("/api/v1/incidents/", h.handleIncidentByID)
}

// handleIncidents обрабатывает запросы к /api/v1/incidents
func (h *HTTPHandler) handleIncidents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listIncidents(w, r)
	case http.MethodPost:
		h.createIncident(w, r)
	default:
		h.writeError(w, errors.New(errors.ErrValidation, "method not allowed"), http.StatusMethodNotAllowed)
	}
}

// handleIncidentByID обрабатывает запросы к /api/v1/incidents/{id}
func (h *HTTPHandler) handleIncidentByID(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из URL
	id := extractIncidentID(r.URL.Path)
	if id == "" {
		h.writeError(w, errors.New(errors.ErrValidation, "invalid incident ID"), http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getIncident(w, r, id)
	case http.MethodPut:
		h.updateIncident(w, r, id)
	case http.MethodDelete:
		h.deleteIncident(w, r, id)
	default:
		h.writeError(w, errors.New(errors.ErrValidation, "method not allowed"), http.StatusMethodNotAllowed)
	}
}

// listIncidents получает список инцидентов
func (h *HTTPHandler) listIncidents(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	
	// Получаем параметры фильтрации
	statusStr := r.URL.Query().Get("status")
	
	var status *domain.IncidentStatus
	if statusStr != "" {
		s := domain.IncidentStatus(statusStr)
		status = &s
	}
	
	filter := &domain.IncidentFilter{
		Status: status,
	}

	incidents, err := h.incidentService.GetIncidents(ctx, filter)
	if err != nil {
		h.writeError(w, err, http.StatusInternalServerError)
		return
	}

	h.writeResponse(w, map[string]interface{}{
		"success": true,
		"incidents": incidents,
		"total":    len(incidents),
		"message":  "Incidents retrieved successfully",
	})
}

// createIncident создает новый инцидент
func (h *HTTPHandler) createIncident(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title       string                 `json:"title"`
		Description string                 `json:"description"`
		Severity    string                 `json:"severity"`
		CheckID     string                 `json:"check_id,omitempty"`
		Metadata    map[string]interface{} `json:"metadata,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, errors.New(errors.ErrValidation, "invalid request body"), http.StatusBadRequest)
		return
	}

	// Валидация обязательных полей
	if err := h.validator.ValidateRequiredFields(map[string]interface{}{
		"title":       req.Title,
		"description": req.Description,
		"severity":    req.Severity,
	}, map[string]string{
		"title":       "required",
		"description": "required",
		"severity":    "required",
	}); err != nil {
		h.writeError(w, err, http.StatusBadRequest)
		return
	}

	// Валидация severity
	if err := h.validator.ValidateEnum(req.Severity, []string{"low", "medium", "high", "critical"}, "severity"); err != nil {
		h.writeError(w, errors.New(errors.ErrValidation, "invalid severity value"), http.StatusBadRequest)
		return
	}

	// Создаем инцидент
	incident := domain.NewIncident(
		req.CheckID,
		domain.IncidentSeverity(req.Severity),
		req.Title,
		req.Description,
	)

	// Сохраняем инцидент через сервис
	ctx := context.Background()
	if err := h.incidentService.CreateIncident(ctx, incident); err != nil {
		h.writeError(w, err, http.StatusInternalServerError)
		return
	}

	h.logger.Info("Incident created via API",
		pkglogger.String("incident_id", incident.ID),
		pkglogger.String("title", req.Title),
		pkglogger.String("severity", req.Severity))

	h.writeResponse(w, map[string]interface{}{
		"success":  true,
		"incident": incident,
		"message":  "Incident created successfully",
	})
}

// getIncident получает инцидент по ID
func (h *HTTPHandler) getIncident(w http.ResponseWriter, r *http.Request, id string) {
	ctx := context.Background()

	incident, err := h.incidentService.GetIncident(ctx, id)
	if err != nil {
		h.writeError(w, err, http.StatusNotFound)
		return
	}

	h.writeResponse(w, map[string]interface{}{
		"success":  true,
		"incident": incident,
		"message":  "Incident retrieved successfully",
	})
}

// updateIncident обновляет инцидент
func (h *HTTPHandler) updateIncident(w http.ResponseWriter, r *http.Request, id string) {
	ctx := context.Background()

	var req struct {
		Title       string `json:"title,omitempty"`
		Description string `json:"description,omitempty"`
		Severity    string `json:"severity,omitempty"`
		Status      string `json:"status,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, errors.New(errors.ErrValidation, "invalid request body"), http.StatusBadRequest)
		return
	}

	// Получаем существующий инцидент
	incident, err := h.incidentService.GetIncident(ctx, id)
	if err != nil {
		h.writeError(w, err, http.StatusNotFound)
		return
	}

	// Обновляем поля
	if req.Title != "" {
		incident.Title = req.Title
	}
	if req.Description != "" {
		incident.Description = req.Description
	}
	if req.Severity != "" {
		if err := h.validator.ValidateEnum(req.Severity, []string{"low", "medium", "high", "critical"}, "severity"); err != nil {
			h.writeError(w, errors.New(errors.ErrValidation, "invalid severity value"), http.StatusBadRequest)
			return
		}
		incident.Severity = domain.IncidentSeverity(req.Severity)
	}
	if req.Status != "" {
		if req.Status == "acknowledged" {
			incident.Acknowledge()
		} else if req.Status == "resolved" {
			incident.Resolve()
		}
	}

	// Сохраняем обновленный инцидент
	if err := h.incidentService.UpdateIncident(ctx, incident); err != nil {
		h.writeError(w, err, http.StatusInternalServerError)
		return
	}

	h.logger.Info("Incident updated via API",
		pkglogger.String("incident_id", id),
		pkglogger.String("status", req.Status))

	h.writeResponse(w, map[string]interface{}{
		"success":  true,
		"incident": incident,
		"message":  "Incident updated successfully",
	})
}

// deleteIncident удаляет инцидент (мягкое удаление - статус resolved)
func (h *HTTPHandler) deleteIncident(w http.ResponseWriter, r *http.Request, id string) {
	ctx := context.Background()

	// Для демонстрации просто разрешаем инцидент
	if err := h.incidentService.ResolveIncident(ctx, id); err != nil {
		h.writeError(w, err, http.StatusNotFound)
		return
	}

	h.logger.Info("Incident resolved via API",
		pkglogger.String("incident_id", id))

	h.writeResponse(w, map[string]interface{}{
		"success": true,
		"message": "Incident resolved successfully",
	})
}

// extractIncidentID извлекает ID инцидента из URL пути
func extractIncidentID(path string) string {
	parts := strings.Split(path, "/")
	// URL формат: /api/v1/incidents/{id}
	// parts будет: ["", "api", "v1", "incidents", "{id}"]
	if len(parts) >= 5 && parts[4] != "" {
		return parts[4]
	}
	return ""
}

// writeResponse пишет JSON ответ
func (h *HTTPHandler) writeResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

// writeError пишет ошибку в формате JSON
func (h *HTTPHandler) writeError(w http.ResponseWriter, err error, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	response := map[string]interface{}{
		"success": false,
		"error":   err.Error(),
		"message": "Request failed",
	}

	// Добавляем детали если есть
	if customErr, ok := err.(*errors.Error); ok {
		if customErr.Details != "" {
			response["details"] = customErr.Details
		}
	}

	json.NewEncoder(w).Encode(response)
}
