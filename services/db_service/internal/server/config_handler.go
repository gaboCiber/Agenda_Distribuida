package server

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/agenda-distribuida/db-service/internal/repository"

	"github.com/gorilla/mux"
)

type ConfigHandler struct {
	repo *repository.ConfigRepository
}

func NewConfigHandler(repo *repository.ConfigRepository) *ConfigHandler {
	return &ConfigHandler{repo: repo}
}

func (h *ConfigHandler) CreateConfig(w http.ResponseWriter, r *http.Request) {
	var config repository.Config
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid request body"))
		return
	}

	err := h.repo.Create(r.Context(), config)
	if err != nil {
		log.Printf("failed to create config: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *ConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	config, err := h.repo.GetByName(r.Context(), name)
	if err != nil {
		log.Printf("failed to get config: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if config.Name == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

func (h *ConfigHandler) ListConfigs(w http.ResponseWriter, r *http.Request) {
	configs, err := h.repo.List(r.Context())
	if err != nil {
		log.Printf("failed to list configs: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(configs)
}

func (h *ConfigHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	var config repository.Config
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid request body"))
		return
	}
	config.Name = name

	err := h.repo.Update(r.Context(), config)
	if err != nil {
		log.Printf("failed to update config: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ConfigHandler) DeleteConfig(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	err := h.repo.Delete(r.Context(), name)
	if err != nil {
		log.Printf("failed to delete config: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ConfigHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/configs", h.CreateConfig).Methods("POST")
	r.HandleFunc("/configs", h.ListConfigs).Methods("GET")
	r.HandleFunc("/configs/{name}", h.GetConfig).Methods("GET")
	r.HandleFunc("/configs/{name}", h.UpdateConfig).Methods("PUT")
	r.HandleFunc("/configs/{name}", h.DeleteConfig).Methods("DELETE")
}
