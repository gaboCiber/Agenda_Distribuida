package api

import (
	"net/http"

	"github.com/agenda-distribuida/group-service/internal/api/handlers"
	"github.com/gorilla/mux"
)

// setupRouter configures the application routes
func setupRouter(
	groupHandler *handlers.GroupHandler,
	memberHandler *handlers.MemberHandler,
	invitationHandler *handlers.InvitationHandler,
) *mux.Router {
	r := mux.NewRouter()

	// API versioning
	api := r.PathPrefix("/api/v1").Subrouter()

	// Health check endpoint
	api.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// Group routes
	api.HandleFunc("/groups", groupHandler.ListUserGroups).Methods("GET")
	api.HandleFunc("/groups", groupHandler.CreateGroup).Methods("POST")
	api.HandleFunc("/groups/{id}", groupHandler.GetGroup).Methods("GET")
	api.HandleFunc("/groups/{id}", groupHandler.UpdateGroup).Methods("PUT")
	api.HandleFunc("/groups/{id}", groupHandler.DeleteGroup).Methods("DELETE")

	// Group member routes
	api.HandleFunc("/groups/{id}/members", memberHandler.ListMembers).Methods("GET")
	api.HandleFunc("/groups/{id}/members", memberHandler.AddMember).Methods("POST")
	api.HandleFunc("/groups/{id}/members/{member_id}", memberHandler.RemoveMember).Methods("DELETE")
	api.HandleFunc("/groups/{id}/admins", memberHandler.GetGroupAdmins).Methods("GET")

	// Invitation routes
	api.HandleFunc("/invitations", invitationHandler.ListUserInvitations).Methods("GET")
	api.HandleFunc("/invitations/{invitation_id}", invitationHandler.GetInvitation).Methods("GET")
	api.HandleFunc("/groups/{id}/invitations", invitationHandler.CreateInvitation).Methods("POST")
	api.HandleFunc("/invitations/{invitation_id}/respond", invitationHandler.RespondToInvitation).Methods("POST")

	// Add request logging middleware
	r.Use(loggingMiddleware)

	// Add recovery middleware to handle panics
	r.Use(recoveryMiddleware)

	return r
}

// loggingMiddleware logs all HTTP requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)
		next.ServeHTTP(w, r)
	})
}

// recoveryMiddleware recovers from panics and returns a 500 error
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Recovered from panic: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
