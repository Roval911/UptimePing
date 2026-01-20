package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestSimpleHealthChecker_Check проверяет работу проверки здоровья
func TestSimpleHealthChecker_Check(t *testing.T) {
	checker := NewSimpleHealthChecker("v1.0.0")
	status := checker.Check()
	
	if status == nil {
		t.Fatal("Expected status, got nil")
	}
	
	if status.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got %s", status.Status)
	}
	
	if status.Timestamp.IsZero() {
		t.Error("Expected timestamp, got zero")
	}
	
	if status.Version != "v1.0.0" {
		t.Errorf("Expected version 'v1.0.0', got %s", status.Version)
	}
}

// TestHandler проверяет HTTP обработчик
func TestHandler(t *testing.T) {
	checker := NewSimpleHealthChecker("v1.0.0")
	handler := Handler(checker)
	
	// Создаем тестовый запрос
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	
	// Выполняем запрос
	handler(w, req)
	
	// Проверяем ответ
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}
	
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got %s", w.Header().Get("Content-Type"))
	}
	
	// Проверяем тело ответа
	var response HealthStatus
	err := json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	
	if response.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got %s", response.Status)
	}
	
	if response.Version != "v1.0.0" {
		t.Errorf("Expected version 'v1.0.0', got %s", response.Version)
	}
}

// TestReadyHandler проверяет ready handler
func TestReadyHandler(t *testing.T) {
	handler := ReadyHandler()
	
	// Создаем тестовый запрос
	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()
	
	// Выполняем запрос
	handler(w, req)
	
	// Проверяем ответ
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}
	
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got %s", w.Header().Get("Content-Type"))
	}
	
	// Проверяем тело ответа
	var response map[string]string
	err := json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	
	if response["status"] != "ready" {
		t.Errorf("Expected status 'ready', got %s", response["status"])
	}
}

// TestLiveHandler проверяет live handler
func TestLiveHandler(t *testing.T) {
	handler := LiveHandler()
	
	// Создаем тестовый запрос
	req := httptest.NewRequest("GET", "/live", nil)
	w := httptest.NewRecorder()
	
	// Выполняем запрос
	handler(w, req)
	
	// Проверяем ответ
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}
	
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got %s", w.Header().Get("Content-Type"))
	}
	
	// Проверяем тело ответа
	var response map[string]string
	err := json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	
	if response["status"] != "alive" {
		t.Errorf("Expected status 'alive', got %s", response["status"])
	}
}