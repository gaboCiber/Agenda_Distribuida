package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/agenda-distribuida/group-service/internal/models"
	"github.com/agenda-distribuida/group-service/internal/services"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

type EventHandler struct {
	redisClient      *redis.Client
	pubsub           *redis.PubSub
	eventService     *services.EventService
	logger           *zap.Logger
	channel          string
	raftNodes        []string
	currentRedisURL  string
	reconnectChan    chan struct{}
	reconnecting     bool
	lastRedisCheck   time.Time
}

func NewEventHandler(
	redisClient *redis.Client,
	eventService *services.EventService,
	channel string,
	raftNodes []string,
	redisURL string,
	logger *zap.Logger,
) *EventHandler {
	return &EventHandler{
		redisClient:     redisClient,
		eventService:    eventService,
		channel:         channel,
		raftNodes:       raftNodes,
		currentRedisURL: redisURL,
		logger:          logger.Named("event_handler"),
		reconnectChan:   make(chan struct{}),
		lastRedisCheck:  time.Now(),
	}
}

// reconnectRedis crea una nueva conexión Redis con la nueva URL
func (h *EventHandler) reconnectRedis(newRedisURL string) error {
	if h.reconnecting {
		h.logger.Debug("Reconexión ya en progreso")
		return nil
	}
	h.reconnecting = true
	defer func() { h.reconnecting = false }()

	h.logger.Info("Reconectando a nuevo Redis primary", zap.String("new_url", newRedisURL))

	// Cerrar pubsub actual si existe
	if h.pubsub != nil {
		if err := h.pubsub.Close(); err != nil {
			h.logger.Error("Error cerrando pubsub actual", zap.Error(err))
		}
		h.pubsub = nil
	}

	if h.redisClient != nil {
		if err := h.redisClient.Close(); err != nil {
			h.logger.Error("Error cerrando conexión Redis actual", zap.Error(err))
		}
	}

	redisOpts, err := redis.ParseURL(newRedisURL)
	if err != nil {
		return fmt.Errorf("error parseando Redis URL: %w", err)
	}

	newRedisClient := redis.NewClient(redisOpts)

	// Verificar conexión con timeout más corto para detectar rápidamente DNS issues
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := newRedisClient.Ping(ctx).Err(); err != nil {
		// Si falla la conexión, intentar obtener el primary actualizado
		h.logger.Warn("Error verificando nueva conexión Redis, intentando obtener primary actualizado", zap.Error(err))
		
		// Intentar obtener el primary actualizado desde el servicio
		updatedCtx, updatedCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer updatedCancel()
		
		if updatedURL, updateErr := h.eventService.UpdateRedisConnection(updatedCtx, newRedisURL); updateErr == nil && updatedURL != newRedisURL {
			h.logger.Info("Se obtuvo un primary actualizado, intentando con nueva URL",
				zap.String("failed_url", newRedisURL),
				zap.String("updated_url", updatedURL))
			
			// Intentar reconectar con la URL actualizada
			return h.reconnectRedis(updatedURL)
		}
		
		return fmt.Errorf("error verificando nueva conexión Redis: %w", err)
	}

	h.redisClient = newRedisClient
	h.currentRedisURL = newRedisURL

	select {
	case h.reconnectChan <- struct{}{}:
		h.logger.Debug("Señal de reconexión enviada")
	default:
		h.logger.Debug("Canal de reconexión ya ocupado")
	}

	h.logger.Info("Reconexión Redis exitosa", zap.String("new_url", newRedisURL))
	return nil
}

func (h *EventHandler) Start(ctx context.Context) error {
	// Verificar el Redis primario al iniciar y reconectar si es necesario
	h.logger.Info("Verificando Redis primario al iniciar...")
	newRedisURL, err := h.eventService.UpdateRedisConnection(ctx, h.currentRedisURL)
	if err != nil {
		h.logger.Warn("No se pudo verificar Redis primario al iniciar", zap.Error(err))
	} else if newRedisURL != h.currentRedisURL {
		h.logger.Info("Redis primario diferente al configurado, reconectando...",
			zap.String("old", h.currentRedisURL),
			zap.String("new", newRedisURL))

		if reconnectErr := h.reconnectRedis(newRedisURL); reconnectErr != nil {
			h.logger.Error("Error reconectando a Redis primario al iniciar",
				zap.Error(reconnectErr),
				zap.String("new_url", newRedisURL))
		} else {
			h.logger.Info("Reconexión inicial exitosa", zap.String("new_url", newRedisURL))
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
		newRedisURL, err := h.eventService.UpdateRedisConnection(ctx, h.currentRedisURL)
		if err == nil && newRedisURL != h.currentRedisURL {
			h.logger.Info("Redis primary actualizado antes de suscripción, reconectando...",
				zap.String("old", h.currentRedisURL),
				zap.String("new", newRedisURL))
			
			if reconnectErr := h.reconnectRedis(newRedisURL); reconnectErr != nil {
				h.logger.Error("Error reconectando antes de suscripción", zap.Error(reconnectErr))
			}
		}

		// Suscribirse al canal de Redis
		var ch <-chan *redis.Message
		
		// Reintentar suscripción con backoff exponencial
		maxRetries := 5
		backoff := 100 * time.Millisecond
		
		for retry := 0; retry < maxRetries; retry++ {
			h.pubsub = h.redisClient.Subscribe(ctx, h.channel)
			ch = h.pubsub.Channel()
			
			// Verificar si la suscripción fue exitosa haciendo ping
			pingErr := h.redisClient.Ping(ctx).Err()
			if pingErr == nil {
				break // Suscripción exitosa
			}
			
			// Cerrar pubsub fallido
			if h.pubsub != nil {
				h.pubsub.Close()
				h.pubsub = nil
			}
			
			if retry < maxRetries-1 {
				h.logger.Warn("Error al suscribirse a Redis, reintentando...",
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
				h.logger.Error("No se pudo suscribirse a Redis después de varios intentos, esperando próximo reintento general",
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

		h.logger.Info("Escuchando eventos de Redis",
			zap.String("channel", h.channel),
			zap.String("redis_url", h.currentRedisURL))

		// Si no se pudo suscribir (ch es nil), esperar y reintentar el bucle principal
		if ch == nil {
			h.logger.Warn("No se pudo suscribirse a Redis, esperando antes de reintentar...")
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
					h.logger.Warn("Conexión pubsub perdida, reintentando suscripción...")
					keepRunning = false
					continue
				}
				go h.processMessage(ctx, msg)

			case <-h.reconnectChan:
				h.logger.Info("Señal de reconexión recibida, cerrando pubsub actual")
				keepRunning = false

			case <-time.After(10 * time.Second): // Timeout para detectar conexiones inactivas
				h.logger.Debug("Timeout de pubsub, verificando conexión...")
				
				// Primero verificar si el Redis primary ha cambiado
				newRedisURL, redisErr := h.eventService.UpdateRedisConnection(ctx, h.currentRedisURL)
				if redisErr == nil && newRedisURL != h.currentRedisURL {
					h.logger.Info("Redis primary ha cambiado durante timeout, reconectando...",
						zap.String("old", h.currentRedisURL),
						zap.String("new", newRedisURL))
					if reconnectErr := h.reconnectRedis(newRedisURL); reconnectErr != nil {
						h.logger.Error("Error reconectando", zap.Error(reconnectErr))
					}
					keepRunning = false
				} else if err := h.redisClient.Ping(ctx).Err(); err != nil {
					h.logger.Error("Redis client desconectado, forzando reconexión", zap.Error(err))
					keepRunning = false
				}

			case <-ctx.Done():
				h.pubsub.Close()
				return ctx.Err()
			}
		}

		// Cerrar pubsub actual antes de reintentar
		if h.pubsub != nil {
			h.pubsub.Close()
			h.pubsub = nil
		}

		// Pausa antes de reintentar
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (h *EventHandler) processMessage(ctx context.Context, msg *redis.Message) {
	// Antes de procesar el mensaje, encontrar y actualizar el líder Raft
	if err := h.eventService.FindAndUpdateLeader(ctx, h.raftNodes); err != nil {
		h.logger.Warn("No se pudo encontrar líder Raft, usando baseURL actual", zap.Error(err))
	}

	// Verificar si el Redis primary ha cambiado (solo si ha pasado suficiente tiempo)
	if time.Since(h.lastRedisCheck) > 5*time.Second {
		newRedisURL, redisErr := h.eventService.UpdateRedisConnection(ctx, h.currentRedisURL)
		h.lastRedisCheck = time.Now()

		if redisErr == nil && newRedisURL != h.currentRedisURL {
			h.logger.Info("Redis primary ha cambiado, reconectando...",
				zap.String("old", h.currentRedisURL),
				zap.String("new", newRedisURL))
			if err := h.reconnectRedis(newRedisURL); err != nil {
				h.logger.Error("Error en reconexión a Redis", zap.Error(err))
			}
		}
	}

	// Registrar la recepción del mensaje
	h.logger.Debug("Mensaje recibido de Redis",
		zap.String("channel", msg.Channel),
		zap.String("payload", msg.Payload))

	// Parsear el evento
	var event models.Event
	if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
		h.logger.Error("Error al decodificar evento",
			zap.Error(err),
			zap.String("payload", msg.Payload))
		return
	}

	// Procesar el evento según su tipo
	var response *models.EventResponse
	var err error

	response, err = h.eventService.ProcessGroupEvent(ctx, event)

	// Manejar errores del procesamiento
	if err != nil {
		h.logger.Error("Error al procesar evento",
			zap.Error(err),
			zap.String("event_type", event.Type),
			zap.String("event_id", event.ID))

		// Enviar respuesta de error si hay un canal de respuesta
		if event.Metadata != nil {
			if replyTo, ok := event.Metadata["reply_to"]; ok && replyTo != "" {
				errResp := models.NewErrorResponse(
					event.ID,
					event.Type,
					err,
				)
				if pubErr := h.publishResponse(ctx, replyTo, errResp); pubErr != nil {
					h.logger.Error("Error al publicar respuesta de error",
						zap.Error(pubErr),
						zap.String("reply_to", replyTo))
				}
			}
		}
		return
	}

	// Publicar la respuesta si se especificó un canal de respuesta
	if event.Metadata != nil {
		if replyTo, ok := event.Metadata["reply_to"]; ok && replyTo != "" {
			if err := h.publishResponse(ctx, replyTo, *response); err != nil {
				h.logger.Error("Error al publicar respuesta",
					zap.Error(err),
					zap.String("reply_to", replyTo))
			}
		}
	}
}

// publishResponse sends a response back to the specified Redis channel
func (h *EventHandler) publishResponse(ctx context.Context, channel string, response models.EventResponse) error {
	// Convert the response to JSON
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("error al serializar respuesta: %w", err)
	}

	// Publish the response
	result := h.redisClient.Publish(ctx, channel, string(responseJSON))
	if result.Err() != nil {
		return fmt.Errorf("error al publicar respuesta: %w", result.Err())
	}

	h.logger.Debug("Respuesta publicada",
		zap.String("channel", channel),
		zap.String("event_id", response.EventID))

	return nil
}
