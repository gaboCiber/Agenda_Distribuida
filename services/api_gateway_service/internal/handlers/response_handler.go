package handlers

import (
	"encoding/json"
	"sync"

	"go.uber.org/zap"
)

// ResponseHandler manages async responses from microservices
type ResponseHandler struct {
	mu      sync.RWMutex
	waiting map[string]chan *UserEventResponse
	logger  *zap.Logger
}

// UserEventResponse represents the response from user service
type UserEventResponse struct {
	EventID string      `json:"event_id"`
	Type    string      `json:"type"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// NewResponseHandler creates a new response handler
func NewResponseHandler(logger *zap.Logger) *ResponseHandler {
	return &ResponseHandler{
		waiting: make(map[string]chan *UserEventResponse),
		logger:  logger.Named("response_handler"),
	}
}

// WaitForResponse creates a channel to wait for a response with the given event ID
func (rh *ResponseHandler) WaitForResponse(eventID string) chan *UserEventResponse {
	rh.mu.Lock()
	defer rh.mu.Unlock()

	ch := make(chan *UserEventResponse, 1)
	rh.waiting[eventID] = ch

	rh.logger.Debug("Created response channel",
		zap.String("event_id", eventID),
		zap.Int("total_waiting", len(rh.waiting)))

	return ch
}

// HandleResponse processes an incoming response from Redis
func (rh *ResponseHandler) HandleResponse(channel, payload string) {
	rh.logger.Info("🎯🎯🎯 RESPONSE_HANDLER ACTIVADO",
		zap.String("channel", channel),
		zap.String("payload", payload),
		zap.Int("payload_length", len(payload)))

	var response UserEventResponse
	if err := json.Unmarshal([]byte(payload), &response); err != nil {
		rh.logger.Error("❌ ERROR al deserializar respuesta",
			zap.Error(err),
			zap.String("payload", payload))
		return
	}

	rh.logger.Info("📦 Respuesta parseada correctamente",
		zap.String("event_id", response.EventID),
		zap.String("type", response.Type),
		zap.Bool("success", response.Success),
		zap.Any("data", response.Data),
		zap.String("error", response.Error))

	rh.mu.Lock()
	ch, exists := rh.waiting[response.EventID]
	if exists {
		delete(rh.waiting, response.EventID)
		rh.mu.Unlock()

		rh.logger.Info("✅ Encontró canal esperando, entregando respuesta",
			zap.String("event_id", response.EventID),
			zap.Bool("success", response.Success),
			zap.Int("remaining_waiting", len(rh.waiting)))

		// Send response to waiting channel (non-blocking)
		select {
		case ch <- &response:
			rh.logger.Debug("✅ Response delivered successfully",
				zap.String("event_id", response.EventID))
		default:
			rh.logger.Warn("⚠️ Response channel was full, dropping response",
				zap.String("event_id", response.EventID))
		}
	} else {
		rh.mu.Unlock()
		rh.logger.Warn("⚠️ No waiting channel for response",
			zap.String("event_id", response.EventID),
			zap.Int("total_waiting", len(rh.waiting)))
	}
}

// Cleanup removes expired waiting channels
func (rh *ResponseHandler) Cleanup() {
	rh.mu.Lock()
	defer rh.mu.Unlock()

	rh.logger.Info("🧹 Cleaning up response handler",
		zap.Int("channels_before", len(rh.waiting)))

	// In a real implementation, you might want to clean up old channels
	// For now, we'll rely on channels being cleaned up when responses arrive
}
