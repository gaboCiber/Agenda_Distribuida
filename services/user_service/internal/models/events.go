package models

import (
	"time"
)

// Event representa un evento recibido a través de Redis
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Metadata  map[string]string      `json:"metadata,omitempty"`
}

// EventResponse representa la respuesta a un evento
type EventResponse struct {
	EventID string      `json:"event_id"`
	Type    string      `json:"type"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// UserEvent es un evento específico para operaciones de usuario
type UserEvent struct {
	Email    string `json:"email"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// NewErrorResponse crea una nueva respuesta de error
func NewErrorResponse(eventID, eventType string, err error) EventResponse {
	return EventResponse{
		EventID: eventID,
		Type:    eventType,
		Success: false,
		Error:   err.Error(),
	}
}

// NewSuccessResponse crea una nueva respuesta exitosa
func NewSuccessResponse(eventID, eventType string, data interface{}) EventResponse {
	return EventResponse{
		EventID: eventID,
		Type:    eventType,
		Success: true,
		Data:    data,
	}
}
