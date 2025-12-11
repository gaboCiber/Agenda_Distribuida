package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/agenda-distribuida/user-service/internal/models"
	"github.com/agenda-distribuida/user-service/internal/services"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

type EventHandler struct {
	redisClient  *redis.Client
	eventService *services.EventService
	logger       *zap.Logger
	channel      string
	raftNodes    []string
}

func NewEventHandler(
	redisClient *redis.Client,
	eventService *services.EventService,
	channel string,
	raftNodes []string,
	logger *zap.Logger,
) *EventHandler {
	return &EventHandler{
		redisClient:  redisClient,
		eventService: eventService,
		channel:      channel,
		raftNodes:    raftNodes,
		logger:       logger.Named("event_handler"),
	}
}

func (h *EventHandler) Start(ctx context.Context) error {
	// Suscribirse al canal de Redis
	pubsub := h.redisClient.Subscribe(ctx, h.channel)
	defer pubsub.Close()

	// Canal para recibir mensajes
	ch := pubsub.Channel()

	h.logger.Info("Escuchando eventos de Redis",
		zap.String("channel", h.channel))

	for {
		select {
		case msg := <-ch:
			go h.processMessage(ctx, msg)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (h *EventHandler) processMessage(ctx context.Context, msg *redis.Message) {
	// Antes de procesar el mensaje, encontrar y actualizar el líder
	if err := h.eventService.FindAndUpdateLeader(ctx, h.raftNodes); err != nil {
		h.logger.Warn("No se pudo encontrar líder, usando baseURL actual", zap.Error(err))
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
	var response models.EventResponse
	var err error

	switch event.Type {
	case "user.create":
		response, err = h.eventService.HandleCreateUser(ctx, event)
	case "user.get":
		response, err = h.eventService.HandleGetUser(ctx, event)
	case "user.login":
		response, err = h.eventService.HandleLogin(ctx, event)
	case "user.update":
		response, err = h.eventService.HandleUpdateUser(ctx, event)
	case "user.delete":
		response, err = h.eventService.HandleDeleteUser(ctx, event)
	case "agenda.event.create":
		response, err = h.eventService.HandleCreateAgendaEvent(ctx, event)
	case "agenda.event.get":
		response, err = h.eventService.HandleGetAgendaEvent(ctx, event)
	case "agenda.event.update":
		response, err = h.eventService.HandleUpdateAgendaEvent(ctx, event)
	case "agenda.event.delete":
		response, err = h.eventService.HandleDeleteAgendaEvent(ctx, event)
	case "agenda.event.list":
		response, err = h.eventService.HandleListAgendaEventsByUser(ctx, event)
	default:
		h.logger.Warn("Tipo de evento no soportado",
			zap.String("event_type", event.Type))
		// Publicar error de tipo no soportado si hay un canal de respuesta
		if event.Metadata != nil {
			if replyTo, ok := event.Metadata["reply_to"]; ok && replyTo != "" {
				errResp := models.NewErrorResponse(
					event.ID,
					event.Type,
					fmt.Errorf("tipo de evento no soportado: %s", event.Type),
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

	// Manejar errores del procesamiento
	if err != nil {
		h.logger.Error("Error al procesar evento",
			zap.Error(err),
			zap.String("event_type", event.Type),
			zap.String("event_id", event.ID))
		return
	}

	// Publicar la respuesta si se especificó un canal de respuesta
	if event.Metadata != nil {
		if replyTo, ok := event.Metadata["reply_to"]; ok && replyTo != "" {
			if err := h.publishResponse(ctx, replyTo, response); err != nil {
				h.logger.Error("Error al publicar respuesta",
					zap.Error(err),
					zap.String("reply_to", replyTo))
			}
		}
	}
}

func (h *EventHandler) publishResponse(ctx context.Context, channel string, response models.EventResponse) error {
	// Convertir la respuesta a JSON
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("error al serializar respuesta: %w", err)
	}

	// Publicar la respuesta
	result := h.redisClient.Publish(ctx, channel, string(responseJSON))
	if result.Err() != nil {
		return fmt.Errorf("error al publicar respuesta: %w", result.Err())
	}

	h.logger.Debug("Respuesta publicada",
		zap.String("channel", channel),
		zap.String("event_id", response.EventID))

	return nil
}
