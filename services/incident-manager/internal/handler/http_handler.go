package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/incident-manager/internal/api"
	"UptimePingPlatform/services/incident-manager/internal/domain"
	"UptimePingPlatform/services/incident-manager/internal/service"
)

// HTTPHandler обрабатывает HTTP запросы для Incident Manager
type HTTPHandler struct {
	logger        logger.Logger
	incidentService service.IncidentService
}

// NewHTTPHandler создает новый HTTP обработчик
func NewHTTPHandler(logger logger.Logger, incidentService service.IncidentService) *HTTPHandler {
	return &HTTPHandler{
		logger:        logger,
		incidentService: incidentService,
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
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleIncidentByID обрабатывает запросы к /api/v1/incidents/{id}
func (h *HTTPHandler) handleIncidentByID(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из URL
	id := extractIncidentID(r.URL.Path)
	if id == "" {
		http.Error(w, "Invalid incident ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getIncident(w, r, id)
	case http.MethodPost:
		// Проверяем, это подтверждение или решение
		if r.URL.Query().Get("action") == "acknowledge" {
			h.acknowledgeIncident(w, r, id)
		} else if r.URL.Query().Get("action") == "resolve" {
			h.resolveIncident(w, r, id)
		} else {
			http.Error(w, "Invalid action. Use ?action=acknowledge or ?action=resolve", http.StatusBadRequest)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listIncidents получает список инцидентов
func (h *HTTPHandler) listIncidents(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Processing list incidents request")

	// Получаем query параметры
	query := r.URL.Query()
	status := query.Get("status")
	severity := query.Get("severity")
	page, _ := strconv.Atoi(query.Get("page"))
	pageSize, _ := strconv.Atoi(query.Get("page_size"))

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	h.logger.Info("List incidents parameters",
		logger.String("status", status),
		logger.String("severity", severity),
		logger.Int("page", page),
		logger.Int("page_size", pageSize))

	// Создаем фильтр для сервиса
	var statusPtr *domain.IncidentStatus
	if statusStr := status; statusStr != "" {
		s := domain.IncidentStatus(statusStr)
		statusPtr = &s
	}
	
	var severityPtr *domain.IncidentSeverity
	if severityStr := severity; severityStr != "" {
		sev := domain.IncidentSeverity(severityStr)
		severityPtr = &sev
	}
	
	limit := pageSize
	offset := (page - 1) * pageSize
	
	filter := &domain.IncidentFilter{
		Status:   statusPtr,
		Severity: severityPtr,
		Limit:    limit,
		Offset:   offset,
	}

	// Вызываем реальный сервис
	domainIncidents, err := h.incidentService.GetIncidents(r.Context(), filter)
	if err != nil {
		h.logger.Error("Failed to get incidents", logger.Error(err))
		http.Error(w, "Failed to get incidents", http.StatusInternalServerError)
		return
	}

	// Конвертируем domain модели в API модели
	incidents := make([]api.Incident, len(domainIncidents))
	for i, domainIncident := range domainIncidents {
		incidents[i] = api.Incident{
			ID:          domainIncident.ID,
			Title:       domainIncident.ErrorMessage, // Используем ErrorMessage как Title
			Description: domainIncident.Metadata["description"].(string), // Если есть
			Status:      string(domainIncident.Status),
			Severity:    string(domainIncident.Severity),
			CreatedAt:   domainIncident.CreatedAt,
			UpdatedAt:   domainIncident.UpdatedAt,
		}
	}

	response := api.ListIncidentsResponse{
		Incidents: incidents,
		Total:     len(incidents),
		Page:      page,
		PageSize:  pageSize,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// getIncident получает инцидент по ID
func (h *HTTPHandler) getIncident(w http.ResponseWriter, r *http.Request, id string) {
	h.logger.Info("Processing get incident request", logger.String("id", id))

	// Вызываем реальный сервис
	domainIncident, err := h.incidentService.GetIncident(r.Context(), id)
	if err != nil {
		h.logger.Error("Failed to get incident", logger.Error(err))
		http.Error(w, "Incident not found", http.StatusNotFound)
		return
	}

	// Конвертируем domain модель в API модель
	incident := api.Incident{
		ID:          domainIncident.ID,
		Title:       domainIncident.ErrorMessage, // Используем ErrorMessage как Title
		Description: "", // Domain модель не имеет поля Description
		Status:      string(domainIncident.Status),
		Severity:    string(domainIncident.Severity),
		CreatedAt:   domainIncident.CreatedAt,
		UpdatedAt:   domainIncident.UpdatedAt,
	}

	response := api.GetIncidentResponse{
		Incident: &incident,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// acknowledgeIncident подтверждает инцидент
func (h *HTTPHandler) acknowledgeIncident(w http.ResponseWriter, r *http.Request, id string) {
	h.logger.Info("Processing acknowledge incident request", logger.String("id", id))

	var req api.AcknowledgeIncidentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode acknowledge request", logger.Error(err))
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	req.ID = id

	h.logger.Info("Acknowledging incident",
		logger.String("id", req.ID),
		logger.String("message", req.Message),
		logger.String("assignee", req.Assignee))

	// Вызываем реальный сервис
	err := h.incidentService.AcknowledgeIncident(r.Context(), id)
	if err != nil {
		h.logger.Error("Failed to acknowledge incident", logger.Error(err))
		http.Error(w, "Failed to acknowledge incident", http.StatusInternalServerError)
		return
	}

	// Получаем обновленный инцидент
	domainIncident, err := h.incidentService.GetIncident(r.Context(), id)
	if err != nil {
		h.logger.Error("Failed to get updated incident", logger.Error(err))
		http.Error(w, "Failed to get updated incident", http.StatusInternalServerError)
		return
	}

	// Конвертируем domain модель в API модель
	incident := api.Incident{
		ID:          domainIncident.ID,
		Title:       domainIncident.ErrorMessage, // Используем ErrorMessage как Title
		Description: "", // Domain модель не имеет поля Description
		Status:      string(domainIncident.Status),
		Severity:    string(domainIncident.Severity),
		CreatedAt:   domainIncident.CreatedAt,
		UpdatedAt:   domainIncident.UpdatedAt,
	}

	response := api.AcknowledgeIncidentResponse{
		Success:   true,
		Message:   "Incident acknowledged successfully",
		Incident:  &incident,
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// resolveIncident решает инцидент
func (h *HTTPHandler) resolveIncident(w http.ResponseWriter, r *http.Request, id string) {
	h.logger.Info("Processing resolve incident request", logger.String("id", id))

	var req api.ResolveIncidentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode resolve request", logger.Error(err))
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	req.ID = id

	h.logger.Info("Resolving incident",
		logger.String("id", req.ID),
		logger.String("message", req.Message),
		logger.String("resolution", req.Resolution))

	// Вызываем реальный сервис
	err := h.incidentService.ResolveIncident(r.Context(), id)
	if err != nil {
		h.logger.Error("Failed to resolve incident", logger.Error(err))
		http.Error(w, "Failed to resolve incident", http.StatusInternalServerError)
		return
	}

	// Получаем обновленный инцидент
	domainIncident, err := h.incidentService.GetIncident(r.Context(), id)
	if err != nil {
		h.logger.Error("Failed to get updated incident", logger.Error(err))
		http.Error(w, "Failed to get updated incident", http.StatusInternalServerError)
		return
	}

	// Конвертируем domain модель в API модель
	incident := api.Incident{
		ID:          domainIncident.ID,
		Title:       domainIncident.ErrorMessage, // Используем ErrorMessage как Title
		Description: "", // Domain модель не имеет поля Description
		Status:      string(domainIncident.Status),
		Severity:    string(domainIncident.Severity),
		CreatedAt:   domainIncident.CreatedAt,
		UpdatedAt:   domainIncident.UpdatedAt,
	}

	response := api.ResolveIncidentResponse{
		Success:   true,
		Message:   "Incident resolved successfully",
		Incident:  &incident,
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Вспомогательные функции

// extractIncidentID извлекает ID инцидента из URL
func extractIncidentID(path string) string {
	// URL формат: /api/v1/incidents/{id}
	parts := splitPath(path)
	if len(parts) >= 4 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "incidents" {
		return parts[3]
	}
	return ""
}

// splitPath разделяет URL путь на компоненты
func splitPath(path string) []string {
	if path == "" || path[0] != '/' {
		return []string{}
	}

	parts := []string{}
	current := ""
	for i, char := range path {
		if i == 0 {
			continue // Пропускаем первый /
		}

		if char == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}

	if current != "" {
		parts = append(parts, current)
	}

	return parts
}
