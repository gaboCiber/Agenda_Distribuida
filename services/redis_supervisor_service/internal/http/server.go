package http

import (
	"net/http"

	"redis_supervisor_service/internal/election"
)

// SetupRoutes configures the HTTP routes for the supervisor
func SetupRoutes(elector *election.Elector) *http.ServeMux {
	handler := NewHandler(elector)
	
	mux := http.NewServeMux()
	mux.HandleFunc("/leader", handler.LeaderHandler)
	
	return mux
}
