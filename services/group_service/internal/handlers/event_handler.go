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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := newRedisClient.Ping(ctx).Err(); err != nil {
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

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		pubsub := h.redisClient.Subscribe(ctx, h.channel)
		ch := pubsub.Channel()

		h.logger.Info("Escuchando eventos de Redis",
			zap.String("channel", h.channel),
			zap.String("redis_url", h.currentRedisURL))

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

			case <-time.After(30 * time.Second):
				h.logger.Debug("Timeout de pubsub, verificando conexión...")
				if err := h.redisClient.Ping(ctx).Err(); err != nil {
					h.logger.Error("Redis client desconectado, forzando reconexión", zap.Error(err))
					keepRunning = false
				} else {
					newRedisURL, redisErr := h.eventService.UpdateRedisConnection(ctx, h.currentRedisURL)
					if redisErr == nil && newRedisURL != h.currentRedisURL {
						h.logger.Info("Redis primary ha cambiado durante timeout, reconectando...",
							zap.String("old", h.currentRedisURL),
							zap.String("new", newRedisURL))
						if reconnectErr := h.reconnectRedis(newRedisURL); reconnectErr != nil {
							h.logger.Error("Error reconectando", zap.Error(reconnectErr))
						}
						keepRunning = false
					}
				}

			case <-ctx.Done():
				pubsub.Close()
				return ctx.Err()
			}
		}

		if pubsub != nil {
			pubsub.Close()
		}

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
	if time.Since(h.lastRedisCheck) > 15*time.Second {
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
