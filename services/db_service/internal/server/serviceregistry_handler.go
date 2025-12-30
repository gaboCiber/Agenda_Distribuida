package server

import (
	"encoding/json"
	"net/http"

	"github.com/agenda-distribuida/db-service/internal/repository"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

type ServiceRegistryHandler struct {
	repo   repository.ServiceRegistryRepository
	logger *zap.Logger
}

func NewServiceRegistryHandler(repo repository.ServiceRegistryRepository, logger *zap.Logger) *ServiceRegistryHandler {
	return &ServiceRegistryHandler{
		repo:   repo,
		logger: logger,
	}
}

func (h *ServiceRegistryHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/services", h.listServices).Methods("GET")
	router.HandleFunc("/services/{serviceName}", h.getService).Methods("GET")
	router.HandleFunc("/services/{serviceName}", h.registerService).Methods("POST")
	router.HandleFunc("/services/{serviceName}", h.deregisterService).Methods("DELETE")
	router.HandleFunc("/services/{serviceName}/heartbeat", h.heartbeat).Methods("POST")
}

func (h *ServiceRegistryHandler) listServices(w http.ResponseWriter, r *http.Request) {
	services, err := h.repo.ListServices(r.Context())
	if err != nil {
		h.logger.Error("Failed to list services", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(services)
}

type registerRequest struct {
	Address string `json:"address"`
}

func (h *ServiceRegistryHandler) registerService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serviceName := vars["serviceName"]

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Address == "" {
		http.Error(w, "Address is required", http.StatusBadRequest)
		return
	}

	// Register the service with a 30-second TTL
	if err := h.repo.Register(r.Context(), serviceName, req.Address); err != nil {
		h.logger.Error("Failed to register service",
			zap.String("service", serviceName),
			zap.String("address", req.Address),
			zap.Error(err))
		http.Error(w, "Failed to register service", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *ServiceRegistryHandler) deregisterService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serviceName := vars["serviceName"]

	if err := h.repo.Deregister(r.Context(), serviceName); err != nil {
		h.logger.Error("Failed to deregister service",
			zap.String("service", serviceName),
			zap.Error(err))
		http.Error(w, "Failed to deregister service", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ServiceRegistryHandler) getService(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serviceName := vars["serviceName"]

	address, err := h.repo.GetService(r.Context(), serviceName)
	if err != nil {
		h.logger.Error("Failed to get service",
			zap.String("service", serviceName),
			zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if address == "" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"service_name": serviceName,
		"address":      address,
	})
}

func (h *ServiceRegistryHandler) heartbeat(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serviceName := vars["serviceName"]

	if err := h.repo.UpdateLastSeen(r.Context(), serviceName); err != nil {
		h.logger.Error("Failed to update service heartbeat",
			zap.String("service", serviceName),
			zap.Error(err))
		http.Error(w, "Failed to update service heartbeat", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
