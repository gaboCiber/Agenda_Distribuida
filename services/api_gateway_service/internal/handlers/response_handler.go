package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/agenda-distribuida/api-gateway-service/internal/services"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// ResponseHandler manages async responses from microservices
type ResponseHandler struct {
	mu              sync.RWMutex
	waiting         map[string]chan *UserEventResponse
	logger          *zap.Logger
	raftNodes       []string
	eventService    *services.EventService
	redisClient     *redis.Client
	pubsub          *redis.PubSub
	currentRedisURL string
	reconnectChan   chan struct{} // Canal para señalizar reconexión
	reconnecting    bool          // Flag para evitar reconexiones simultáneas
	lastRedisCheck  time.Time     // Última vez que se verificó Redis primary
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
func NewResponseHandler(redisClient *redis.Client, eventService *services.EventService, raftNodes []string, redisURL string, logger *zap.Logger) *ResponseHandler {
	return &ResponseHandler{
		waiting:         make(map[string]chan *UserEventResponse),
		logger:          logger.Named("response_handler"),
		raftNodes:       raftNodes,
		eventService:    eventService,
		redisClient:     redisClient,
		currentRedisURL: redisURL,
		reconnectChan:   make(chan struct{}),
		lastRedisCheck:  time.Now(), // Inicializar al momento de creación
	}
}

// reconnectRedis crea una nueva conexión Redis con la nueva URL
func (rh *ResponseHandler) reconnectRedis(newRedisURL string) error {
	// Evitar reconexiones simultáneas
	if rh.reconnecting {
		rh.logger.Debug("Reconexión ya en progreso, ignorando solicitud")
		return nil
	}

	rh.reconnecting = true
	defer func() { rh.reconnecting = false }()

	rh.logger.Info("Reconectando a nuevo Redis primary", zap.String("new_url", newRedisURL))

	// Cerrar pubsub actual si existe
	if rh.pubsub != nil {
		if err := rh.pubsub.Close(); err != nil {
			rh.logger.Error("Error cerrando pubsub actual", zap.Error(err))
		}
		rh.pubsub = nil
	}

	// Validar que tengamos un cliente actual
	if rh.redisClient == nil {
		rh.logger.Error("Redis client es nil, creando nuevo cliente")
	} else {
		// Cerrar conexión actual
		if err := rh.redisClient.Close(); err != nil {
			rh.logger.Error("Error cerrando conexión Redis actual", zap.Error(err))
		}
	}

	// Parsear nueva URL
	redisOpts, err := redis.ParseURL(newRedisURL)
	if err != nil {
		return fmt.Errorf("error parseando Redis URL: %w", err)
	}

	// Crear nuevo cliente
	newRedisClient := redis.NewClient(redisOpts)

	// Verificar conexión con timeout más corto para detectar rápidamente DNS issues
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := newRedisClient.Ping(ctx).Err(); err != nil {
		// Si falla la conexión, intentar obtener el primary actualizado
		rh.logger.Warn("Error verificando nueva conexión Redis, intentando obtener primary actualizado", zap.Error(err))

		// Intentar obtener el primary actualizado desde el servicio
		updatedCtx, updatedCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer updatedCancel()

		if updatedURL, updateErr := rh.eventService.UpdateRedisConnection(updatedCtx, newRedisURL); updateErr == nil && updatedURL != newRedisURL {
			rh.logger.Info("Se obtuvo un primary actualizado, intentando con nueva URL",
				zap.String("failed_url", newRedisURL),
				zap.String("updated_url", updatedURL))

			// Intentar reconectar con la URL actualizada
			return rh.reconnectRedis(updatedURL)
		}

		return fmt.Errorf("error verificando nueva conexión Redis: %w", err)
	}

	// Actualizar el cliente solo si la verificación fue exitosa
	rh.redisClient = newRedisClient
	rh.currentRedisURL = newRedisURL

	// Señalizar que necesitamos reconectar el pubsub
	select {
	case rh.reconnectChan <- struct{}{}:
		rh.logger.Debug("Señal de reconexión enviada")
	default:
		rh.logger.Debug("Canal de reconexión ya ocupado")
	}

	rh.logger.Info("Reconexión Redis exitosa", zap.String("new_url", newRedisURL))
	return nil
}

// StartResponseListener inicia el listener global de respuestas con soporte de reconexión
func (rh *ResponseHandler) StartResponseListener(ctx context.Context) error {
	// Verificar el Redis primario al iniciar y reconectar si es necesario
	rh.logger.Info("Verificando Redis primario al iniciar...")
	newRedisURL, err := rh.eventService.UpdateRedisConnection(ctx, rh.currentRedisURL)
	if err != nil {
		rh.logger.Warn("No se pudo verificar Redis primario al iniciar", zap.Error(err))
	} else if newRedisURL != rh.currentRedisURL {
		rh.logger.Info("Redis primario diferente al configurado, reconectando...",
			zap.String("old", rh.currentRedisURL),
			zap.String("new", newRedisURL))

		if reconnectErr := rh.reconnectRedis(newRedisURL); reconnectErr != nil {
			rh.logger.Error("Error reconectando a Redis primario al iniciar",
				zap.Error(reconnectErr),
				zap.String("new_url", newRedisURL))
		} else {
			rh.logger.Info("Reconexión inicial exitosa", zap.String("new_url", newRedisURL))
		}
	}

	// Bucle principal
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Verificar el Redis primario antes de suscribirse
		newRedisURL, err := rh.eventService.UpdateRedisConnection(ctx, rh.currentRedisURL)
		if err == nil && newRedisURL != rh.currentRedisURL {
			rh.logger.Info("Redis primary actualizado antes de suscripción, reconectando...",
				zap.String("old", rh.currentRedisURL),
				zap.String("new", newRedisURL))

			if reconnectErr := rh.reconnectRedis(newRedisURL); reconnectErr != nil {
				rh.logger.Error("Error reconectando antes de suscripción", zap.Error(reconnectErr))
			}
		}

		// Suscribirse a los canales de Redis
		responseChannels := []string{
			"users_events_response_1", "users_events_response_2", "users_events_response_3",
			"groups_events_response_1", "groups_events_response_2", "groups_events_response_3",
		}
		
		var ch <-chan *redis.Message
		
		// Reintentar suscripción con backoff exponencial
		maxRetries := 5
		backoff := 100 * time.Millisecond
		
		for retry := 0; retry < maxRetries; retry++ {
			rh.pubsub = rh.redisClient.Subscribe(ctx, responseChannels...)
			ch = rh.pubsub.Channel()
			
			// Verificar si la suscripción fue exitosa haciendo ping
			pingErr := rh.redisClient.Ping(ctx).Err()
			if pingErr == nil {
				break // Suscripción exitosa
			}
			
			// Cerrar pubsub fallido
			if rh.pubsub != nil {
				rh.pubsub.Close()
				rh.pubsub = nil
			}
			
			if retry < maxRetries-1 {
				rh.logger.Warn("Error al suscribirse a Redis, reintentando...",
					zap.Error(pingErr),
					zap.Int("retry", retry+1),
					zap.Duration("backoff", backoff))
				
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(backoff):
				}
				backoff *= 2 // Backoff exponencial
			} else {
				rh.logger.Error("No se pudo suscribirse a Redis después de varios intentos, esperando próximo reintento general",
					zap.Error(pingErr),
					zap.Int("max_retries", maxRetries))
				
				// Esperar antes de continuar al bucle principal para reintentar
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(5 * time.Second):
				}
			}
		}

		rh.logger.Info("Escuchando respuestas de Redis",
			zap.Strings("channels", []string{"users_events_response", "events_response", "groups_events_response", "group_events_response"}),
			zap.String("redis_url", rh.currentRedisURL))

		// Si no se pudo suscribir (ch es nil), esperar y reintentar el bucle principal
		if ch == nil {
			rh.logger.Warn("No se pudo suscribirse a Redis, esperando antes de reintentar...")
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
			continue // Volver al inicio del bucle principal
		}

		// Bucle de procesamiento de mensajes
		keepRunning := true
		for keepRunning {
			select {
			case msg := <-ch:
				if msg == nil {
					rh.logger.Warn("Conexión pubsub perdida, reintentando suscripción...")
					keepRunning = false
					continue
				}
				go rh.HandleResponse(msg.Channel, msg.Payload)

			case <-rh.reconnectChan:
				rh.logger.Info("Señal de reconexión recibida, cerrando pubsub actual")
				keepRunning = false

			case <-time.After(10 * time.Second): // Timeout para detectar conexiones inactivas
				rh.logger.Debug("Timeout de pubsub, verificando conexión...")

				// Primero verificar si el Redis primary ha cambiado
				newRedisURL, redisErr := rh.eventService.UpdateRedisConnection(ctx, rh.currentRedisURL)
				if redisErr == nil && newRedisURL != rh.currentRedisURL {
					rh.logger.Info("Redis primary ha cambiado durante timeout, reconectando...",
						zap.String("old", rh.currentRedisURL),
						zap.String("new", newRedisURL))
					if reconnectErr := rh.reconnectRedis(newRedisURL); reconnectErr != nil {
						rh.logger.Error("Error reconectando", zap.Error(reconnectErr))
					}
					keepRunning = false
				} else if err := rh.redisClient.Ping(ctx).Err(); err != nil {
					rh.logger.Error("Redis client desconectado, forzando reconexión", zap.Error(err))
					keepRunning = false
				}

			case <-ctx.Done():
				rh.pubsub.Close()
				return ctx.Err()
			}
		}

		// Cerrar pubsub actual antes de reintentar
		if rh.pubsub != nil {
			rh.pubsub.Close()
			rh.pubsub = nil
		}

		// Pausa antes de reintentar
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// GetRedisClient devuelve el cliente Redis actualizado
func (rh *ResponseHandler) GetRedisClient() *redis.Client {
	return rh.redisClient
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
	// Antes de procesar el mensaje, encontrar y actualizar el líder Raft
	ctx := context.Background()
	if err := rh.eventService.FindAndUpdateLeader(ctx, rh.raftNodes); err != nil {
		rh.logger.Warn("No se pudo encontrar líder Raft, usando baseURL actual", zap.Error(err))
	}

	// Verificar si el Redis primary ha cambiado (solo si ha pasado suficiente tiempo)
	if time.Since(rh.lastRedisCheck) > 5*time.Second {
		newRedisURL, redisErr := rh.eventService.UpdateRedisConnection(ctx, rh.currentRedisURL)
		rh.lastRedisCheck = time.Now()

		if redisErr != nil {
			rh.logger.Debug("No se pudo verificar Redis primary, usando conexión actual", zap.Error(redisErr))
		} else if newRedisURL != rh.currentRedisURL {
			rh.logger.Info("Redis primary ha cambiado, intentando reconexión",
				zap.String("old", rh.currentRedisURL),
				zap.String("new", newRedisURL))

			// Reconectar al nuevo Redis primary
			if reconnectErr := rh.reconnectRedis(newRedisURL); reconnectErr != nil {
				rh.logger.Error("Error reconectando a Redis primary",
					zap.Error(reconnectErr),
					zap.String("new_url", newRedisURL))
				// Continuar con la conexión actual por ahora
			}
		}
	}

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
