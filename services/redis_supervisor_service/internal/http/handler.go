package http

import (
	"encoding/json"
	"net/http"

	"redis_supervisor_service/internal/election"
)

// Handler provides HTTP endpoints for the supervisor
type Handler struct {
	elector *election.Elector
}

// NewHandler creates a new HTTP handler
func NewHandler(elector *election.Elector) *Handler {
	return &Handler{
		elector: elector,
	}
}

// LeaderResponse represents the response for the leader endpoint
type LeaderResponse struct {
	LeaderID    string `json:"leader_id"`
	IsLeader    bool   `json:"is_leader"`
	Epoch       uint64 `json:"epoch"`
	SupervisorID string `json:"supervisor_id"`
}

// LeaderHandler returns the current leader information
func (h *Handler) LeaderHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	leaderID, epoch := h.elector.CurrentLeader()
	isLeader := h.elector.IsLeader()

	response := LeaderResponse{
		LeaderID:     leaderID,
		IsLeader:     isLeader,
		Epoch:        epoch,
		SupervisorID: h.elector.GetID(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
