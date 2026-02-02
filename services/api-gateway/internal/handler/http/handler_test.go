package http

import (
	"UptimePingPlatform/pkg/logger"
	"UptimePingPlatform/services/api-gateway/internal/middleware"
	"github.com/gorilla/mux"
	"net/http"
)

// TestHandler - простой тестовый handler
type TestHandler struct {
	mux *mux.Router
}

func NewTestHandler() *TestHandler {
	h := &TestHandler{
		mux: mux.NewRouter(),
	}
	h.setupRoutes()
	return h
}

func (h *TestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *TestHandler) setupRoutes() {
	// Health check роуты
	h.mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("healthy"))
	})

	// САМЫЙ ПРОСТОЙ ENDPOINT ДЛЯ ТЕСТИРОВАНИЯ
	h.mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	})
}
