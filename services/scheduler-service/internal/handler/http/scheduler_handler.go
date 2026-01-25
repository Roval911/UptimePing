package http

import (
	"encoding/json"
	"fmt"
	"net/http"

	"UptimePingPlatform/pkg/errors"
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/scheduler-service/internal/usecase"
)

// SchedulerHandler обрабатывает HTTP запросы для управления планировщиком
type SchedulerHandler struct {
	schedulerUseCase *usecase.SchedulerUseCase
	logger           logger.Logger
}

// NewSchedulerHandler создает новый экземпляр SchedulerHandler
func NewSchedulerHandler(schedulerUseCase *usecase.SchedulerUseCase, logger logger.Logger) *SchedulerHandler {
	return &SchedulerHandler{
		schedulerUseCase: schedulerUseCase,
		logger:           logger,
	}
}

// Start запускает планировщик
func (h *SchedulerHandler) Start(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, errors.New(errors.ErrValidation, "Method not allowed").
			WithDetails("Only POST method is allowed").
			WithContext(r.Context()))
		return
	}

	h.logger.Info("Starting scheduler via HTTP request", logger.CtxField(r.Context()))

	if err := h.schedulerUseCase.Start(r.Context()); err != nil {
		h.logger.Error("Failed to start scheduler",
			logger.Error(err),
			logger.CtxField(r.Context()),
		)
		h.writeError(w, errors.Wrap(err, errors.ErrInternal, "Failed to start scheduler").
			WithContext(r.Context()))
		return
	}

	response := map[string]interface{}{
		"status":  "started",
		"message": "Scheduler started successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Stop останавливает планировщик
func (h *SchedulerHandler) Stop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, errors.New(errors.ErrValidation, "Method not allowed").
			WithDetails("Only POST method is allowed").
			WithContext(r.Context()))
		return
	}

	h.logger.Info("Stopping scheduler via HTTP request", logger.CtxField(r.Context()))

	if err := h.schedulerUseCase.Stop(r.Context()); err != nil {
		h.logger.Error("Failed to stop scheduler",
			logger.Error(err),
			logger.CtxField(r.Context()),
		)
		h.writeError(w, errors.Wrap(err, errors.ErrInternal, "Failed to stop scheduler").
			WithContext(r.Context()))
		return
	}

	response := map[string]interface{}{
		"status":  "stopped",
		"message": "Scheduler stopped successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// ExecuteTask выполняет конкретную задачу
func (h *SchedulerHandler) ExecuteTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, errors.New(errors.ErrValidation, "Method not allowed").
			WithDetails("Only POST method is allowed").
			WithContext(r.Context()))
		return
	}

	var request struct {
		CheckID string `json:"check_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.writeError(w, errors.Wrap(err, errors.ErrValidation, "Invalid request body").
			WithContext(r.Context()))
		return
	}

	if request.CheckID == "" {
		h.writeError(w, errors.New(errors.ErrValidation, "check_id is required").
			WithContext(r.Context()))
		return
	}

	h.logger.Info("Executing task via HTTP request",
		logger.String("check_id", request.CheckID),
		logger.CtxField(r.Context()),
	)

	if err := h.schedulerUseCase.ExecuteTask(r.Context(), request.CheckID); err != nil {
		h.logger.Error("Failed to execute task",
			logger.String("check_id", request.CheckID),
			logger.Error(err),
			logger.CtxField(r.Context()),
		)
		h.writeError(w, errors.Wrap(err, errors.ErrInternal, "Failed to execute task").
			WithDetails(fmt.Sprintf("check_id: %s", request.CheckID)).
			WithContext(r.Context()))
		return
	}

	response := map[string]interface{}{
		"status":   "executed",
		"check_id": request.CheckID,
		"message":  "Task executed successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Stats возвращает статистику планировщика
func (h *SchedulerHandler) Stats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, errors.New(errors.ErrValidation, "Method not allowed").
			WithDetails("Only GET method is allowed").
			WithContext(r.Context()))
		return
	}

	stats := h.schedulerUseCase.GetStats()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"data":   stats,
	})
}

// writeError записывает ошибку в ответ используя pkg/errors
func (h *SchedulerHandler) writeError(w http.ResponseWriter, err error) {
	if customErr, ok := err.(*errors.Error); ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(customErr.HTTPStatus())

		response := map[string]interface{}{
			"error": map[string]interface{}{
				"code":    string(customErr.Code),
				"message": customErr.GetUserMessage(),
				"details": customErr.Details,
			},
		}

		json.NewEncoder(w).Encode(response)
	} else {
		// Создаем внутреннюю ошибку для неизвестных ошибок
		customErr := errors.New(errors.ErrInternal, "Internal Server Error")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(customErr.HTTPStatus())
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    string(customErr.Code),
				"message": customErr.GetUserMessage(),
				"details": customErr.Details,
			},
		})
	}
}
