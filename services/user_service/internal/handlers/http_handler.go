package handlers

import (
	"net/http"

	"go.uber.org/zap"
)

// HTTPHandler maneja las peticiones HTTP del servicio
type HTTPHandler struct {
	logger *zap.Logger
}

// NewHTTPHandler crea una nueva instancia de HTTPHandler
func NewHTTPHandler(logger *zap.Logger) *HTTPHandler {
	return &HTTPHandler{
		logger: logger,
	}
}

// HealthCheck maneja las peticiones al endpoint de salud
func (h *HTTPHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "ok"}`))
}

// SetupRoutes configura las rutas HTTP
func (h *HTTPHandler) SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.HealthCheck)
}
