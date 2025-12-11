package services

import (
	"context"
	"fmt"
	"time"

	"github.com/agenda-distribuida/user-service/internal/clients"
	"github.com/agenda-distribuida/user-service/internal/models"
	"go.uber.org/zap"
)

type EventService struct {
	dbClient *clients.DBServiceClient
	logger   *zap.Logger
}

// AgendaEvent representa un evento de agenda/calendario
type AgendaEvent struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Location    string    `json:"location"`
	UserID      string    `json:"user_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func NewEventService(dbClient *clients.DBServiceClient, logger *zap.Logger) *EventService {
	return &EventService{
		dbClient: dbClient,
		logger:   logger.Named("event_service"),
	}
}

// FindAndUpdateLeader busca y actualiza el líder del cluster Raft
func (s *EventService) FindAndUpdateLeader(ctx context.Context, raftNodes []string) error {
	return s.dbClient.FindAndUpdateLeader(ctx, raftNodes)
}

func (s *EventService) HandleCreateUser(ctx context.Context, event models.Event) (models.EventResponse, error) {
	s.logger.Info("Procesando evento de creación de usuario",
		zap.String("event_id", event.ID),
		zap.String("email", event.Data["email"].(string)))

	// Validar datos del evento
	email, ok := event.Data["email"].(string)
	if !ok || email == "" {
		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("email es requerido"),
		), nil
	}

	username, _ := event.Data["username"].(string)
	if username == "" {
		// Usar la parte antes del @ del email como nombre de usuario
		username = email
	}

	password, ok := event.Data["password"].(string)
	if !ok || password == "" {
		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("contraseña es requerida"),
		), nil
	}

	// Crear el usuario a través del db_service
	user, err := s.dbClient.CreateUser(ctx, email, password, username)

	if err != nil {
		s.logger.Error("Error al crear usuario",
			zap.String("email", email),
			zap.Error(err))

		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("error al crear usuario: %w", err),
		), nil
	}

	s.logger.Info("Usuario creado exitosamente",
		zap.String("email", email),
		zap.String("user_id", user.ID))

	return models.NewSuccessResponse(
		event.ID,
		event.Type,
		map[string]interface{}{
			"id":        user.ID,
			"email":     user.Email,
			"username":  user.Username,
			"is_active": user.IsActive,
		},
	), nil
}

// HandleGetUser maneja la obtención de un usuario por su ID
func (s *EventService) HandleGetUser(ctx context.Context, event models.Event) (models.EventResponse, error) {
	// Extraer el ID del usuario del evento
	userID, ok := event.Data["user_id"].(string)
	if !ok || userID == "" {
		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("user_id es requerido"),
		), nil
	}

	s.logger.Info("Procesando evento de obtención de usuario",
		zap.String("event_id", event.ID),
		zap.String("user_id", userID))

	// Obtener el usuario a través del db_service
	user, err := s.dbClient.GetUser(ctx, userID)
	if err != nil {
		s.logger.Error("Error al obtener usuario",
			zap.String("user_id", userID),
			zap.Error(err))

		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("error al obtener usuario: %w", err),
		), nil
	}

	s.logger.Debug("Usuario obtenido exitosamente",
		zap.String("user_id", userID))

	return models.NewSuccessResponse(
		event.ID,
		event.Type,
		map[string]interface{}{
			"id":         user.ID,
			"email":      user.Email,
			"username":   user.Username,
			"is_active":  user.IsActive,
			"created_at": user.CreatedAt,
			"updated_at": user.UpdatedAt,
		},
	), nil
}

// HandleLogin maneja la autenticación de un usuario
func (s *EventService) HandleLogin(ctx context.Context, event models.Event) (models.EventResponse, error) {
	// Extraer credenciales del evento
	email, ok := event.Data["email"].(string)
	if !ok || email == "" {
		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("email es requerido"),
		), nil
	}

	password, ok := event.Data["password"].(string)
	if !ok || password == "" {
		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("contraseña es requerida"),
		), nil
	}

	s.logger.Info("Procesando evento de inicio de sesión",
		zap.String("event_id", event.ID),
		zap.String("email", email))

	// Autenticar al usuario a través del db_service
	user, err := s.dbClient.Login(ctx, email, password)
	if err != nil || user == nil {
		s.logger.Error("Error en el inicio de sesión",
			zap.String("email", email),
			zap.Error(err),
			zap.Bool("user_not_found", user == nil))

		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("credenciales inválidas"),
		), nil
	}

	s.logger.Info("Inicio de sesión exitoso",
		zap.String("user_id", user.ID),
		zap.String("email", email))

	return models.NewSuccessResponse(
		event.ID,
		event.Type,
		map[string]interface{}{
			"id":        user.ID,
			"email":     user.Email,
			"username":  user.Username,
			"is_active": user.IsActive,
		},
	), nil
}

// HandleCreateAgendaEvent maneja la creación de un nuevo evento de agenda
func (s *EventService) HandleCreateAgendaEvent(ctx context.Context, event models.Event) (models.EventResponse, error) {
	// Extraer y validar los datos del evento
	title, ok := event.Data["title"].(string)
	if !ok || title == "" {
		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("el título es requerido"),
		), nil
	}

	description, _ := event.Data["description"].(string)
	location, _ := event.Data["location"].(string)
	userID, _ := event.Data["user_id"].(string)

	// Parsear fechas
	startTimeStr, ok := event.Data["start_time"].(string)
	if !ok || startTimeStr == "" {
		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("start_time es requerido"),
		), nil
	}

	startTime, err := time.Parse(time.RFC3339, startTimeStr)
	if err != nil {
		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("formato de start_time inválido, se esperaba formato RFC3339"),
		), nil
	}

	endTimeStr, ok := event.Data["end_time"].(string)
	if !ok || endTimeStr == "" {
		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("end_time es requerido"),
		), nil
	}

	endTime, err := time.Parse(time.RFC3339, endTimeStr)
	if err != nil {
		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("formato de end_time inválido, se esperaba formato RFC3339"),
		), nil
	}

	if endTime.Before(startTime) {
		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("end_time debe ser posterior a start_time"),
		), nil
	}

	// Crear el evento en la base de datos
	newEvent := &clients.AgendaEvent{
		Title:       title,
		Description: description,
		StartTime:   startTime,
		EndTime:     endTime,
		Location:    location,
		UserID:      userID,
	}

	createdEvent, err := s.dbClient.CreateAgendaEvent(ctx, newEvent)
	if err != nil {
		s.logger.Error("Error al crear el evento de agenda",
			zap.String("event_id", event.ID),
			zap.Error(err))

		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("error al crear el evento: %w", err),
		), nil
	}

	s.logger.Info("Evento de agenda creado exitosamente",
		zap.String("event_id", createdEvent.ID),
		zap.String("title", createdEvent.Title))

	return models.NewSuccessResponse(
		event.ID,
		event.Type,
		map[string]interface{}{
			"id":          createdEvent.ID,
			"title":       createdEvent.Title,
			"description": createdEvent.Description,
			"start_time":  createdEvent.StartTime.Format(time.RFC3339),
			"end_time":    createdEvent.EndTime.Format(time.RFC3339),
			"location":    createdEvent.Location,
			"user_id":     createdEvent.UserID,
			"created_at":  createdEvent.CreatedAt.Format(time.RFC3339),
		},
	), nil
}

// HandleGetAgendaEvent maneja la obtención de un evento de agenda por su ID
func (s *EventService) HandleGetAgendaEvent(ctx context.Context, event models.Event) (models.EventResponse, error) {
	eventID, ok := event.Data["event_id"].(string)
	if !ok || eventID == "" {
		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("event_id es requerido"),
		), nil
	}

	s.logger.Info("Obteniendo evento de agenda",
		zap.String("event_id", eventID))

	// Obtener el evento de la base de datos
	agendaEvent, err := s.dbClient.GetAgendaEvent(ctx, eventID)
	if err != nil {
		s.logger.Error("Error al obtener el evento de agenda",
			zap.String("event_id", eventID),
			zap.Error(err))

		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("error al obtener el evento: %w", err),
		), nil
	}

	return models.NewSuccessResponse(
		event.ID,
		event.Type,
		map[string]interface{}{
			"id":          agendaEvent.ID,
			"title":       agendaEvent.Title,
			"description": agendaEvent.Description,
			"start_time":  agendaEvent.StartTime.Format(time.RFC3339),
			"end_time":    agendaEvent.EndTime.Format(time.RFC3339),
			"location":    agendaEvent.Location,
			"user_id":     agendaEvent.UserID,
			"created_at":  agendaEvent.CreatedAt.Format(time.RFC3339),
			"updated_at":  agendaEvent.UpdatedAt.Format(time.RFC3339),
		},
	), nil
}

// HandleUpdateAgendaEvent maneja la actualización de un evento de agenda existente
func (s *EventService) HandleUpdateAgendaEvent(ctx context.Context, event models.Event) (models.EventResponse, error) {
	eventID, ok := event.Data["event_id"].(string)
	if !ok || eventID == "" {
		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("event_id es requerido"),
		), nil
	}

	// Obtener el evento actual para tener todos los campos
	currentEvent, err := s.dbClient.GetAgendaEvent(ctx, eventID)
	if err != nil {
		s.logger.Error("Error al obtener el evento actual",
			zap.String("event_id", eventID),
			zap.Error(err))
		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("error al obtener el evento: %w", err),
		), nil
	}

	// Crear un mapa con los campos actualizables
	updates := make(map[string]interface{})

	// Actualizar solo los campos que se proporcionaron en la solicitud
	if title, ok := event.Data["title"].(string); ok && title != "" {
		currentEvent.Title = title
	}
	updates["title"] = currentEvent.Title

	if description, ok := event.Data["description"].(string); ok {
		currentEvent.Description = description
	}
	updates["description"] = currentEvent.Description

	if location, ok := event.Data["location"].(string); ok {
		currentEvent.Location = location
	}
	updates["location"] = currentEvent.Location

	if startTimeStr, ok := event.Data["start_time"].(string); ok && startTimeStr != "" {
		startTime, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			return models.NewErrorResponse(
				event.ID,
				event.Type,
				fmt.Errorf("formato de start_time inválido, se esperaba formato RFC3339"),
			), nil
		}
		currentEvent.StartTime = startTime
	}
	updates["start_time"] = currentEvent.StartTime

	if endTimeStr, ok := event.Data["end_time"].(string); ok && endTimeStr != "" {
		endTime, err := time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			return models.NewErrorResponse(
				event.ID,
				event.Type,
				fmt.Errorf("formato de end_time inválido, se esperaba formato RFC3339"),
			), nil
		}
		currentEvent.EndTime = endTime
	}
	updates["end_time"] = currentEvent.EndTime

	// Asegurarse de que el user_id siempre esté presente
	if userID, ok := event.Data["user_id"].(string); ok && userID != "" {
		currentEvent.UserID = userID
	}
	updates["user_id"] = currentEvent.UserID

	s.logger.Info("Actualizando evento de agenda",
		zap.String("event_id", eventID),
		zap.Any("updates", updates))

	// Actualizar el evento en la base de datos
	updatedEvent, err := s.dbClient.UpdateAgendaEvent(ctx, eventID, updates)
	if err != nil {
		s.logger.Error("Error al actualizar el evento de agenda",
			zap.String("event_id", eventID),
			zap.Error(err))

		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("error al actualizar el evento: %w", err),
		), nil
	}

	return models.NewSuccessResponse(
		event.ID,
		event.Type,
		map[string]interface{}{
			"id":          updatedEvent.ID,
			"title":       updatedEvent.Title,
			"description": updatedEvent.Description,
			"start_time":  updatedEvent.StartTime.Format(time.RFC3339),
			"end_time":    updatedEvent.EndTime.Format(time.RFC3339),
			"location":    updatedEvent.Location,
			"user_id":     updatedEvent.UserID,
			"updated_at":  updatedEvent.UpdatedAt.Format(time.RFC3339),
		},
	), nil
}

// HandleDeleteAgendaEvent maneja la eliminación de un evento de agenda
func (s *EventService) HandleDeleteAgendaEvent(ctx context.Context, event models.Event) (models.EventResponse, error) {
	eventID, ok := event.Data["event_id"].(string)
	if !ok || eventID == "" {
		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("event_id es requerido"),
		), nil
	}

	s.logger.Info("Eliminando evento de agenda",
		zap.String("event_id", eventID))

	// Eliminar el evento de la base de datos
	err := s.dbClient.DeleteAgendaEvent(ctx, eventID)
	if err != nil {
		s.logger.Error("Error al eliminar el evento de agenda",
			zap.String("event_id", eventID),
			zap.Error(err))

		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("error al eliminar el evento: %w", err),
		), nil
	}

	return models.NewSuccessResponse(
		event.ID,
		event.Type,
		map[string]interface{}{
			"message":  "Evento eliminado correctamente",
			"event_id": eventID,
		},
	), nil
}

// HandleListAgendaEventsByUser maneja la obtención de todos los eventos de un usuario
func (s *EventService) HandleListAgendaEventsByUser(ctx context.Context, event models.Event) (models.EventResponse, error) {
	userID, ok := event.Data["user_id"].(string)
	if !ok || userID == "" {
		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("user_id es requerido"),
		), nil
	}

	// Obtener parámetros de paginación (opcionales)
	offset := 0
	limit := 100 // Valor por defecto

	if offsetVal, ok := event.Data["offset"].(float64); ok {
		offset = int(offsetVal)
		if offset < 0 {
			offset = 0
		}
	}

	if limitVal, ok := event.Data["limit"].(float64); ok {
		limit = int(limitVal)
		if limit <= 0 {
			limit = 100
		}
		// Limitar el máximo a 1000 para prevenir abusos
		if limit > 1000 {
			limit = 1000
		}
	}

	s.logger.Info("Listando eventos de agenda por usuario",
		zap.String("user_id", userID),
		zap.Int("offset", offset),
		zap.Int("limit", limit))

	// Obtener los eventos de la base de datos
	events, err := s.dbClient.ListAgendaEventsByUser(ctx, userID, offset, limit)
	if err != nil {
		s.logger.Error("Error al listar eventos de agenda",
			zap.String("user_id", userID),
			zap.Error(err))

		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("error al listar eventos: %w", err),
		), nil
	}

	// Convertir los eventos a un formato serializable
	eventsData := make([]map[string]interface{}, 0, len(events))
	for _, evt := range events {
		eventsData = append(eventsData, map[string]interface{}{
			"id":          evt.ID,
			"title":       evt.Title,
			"description": evt.Description,
			"start_time":  evt.StartTime.Format(time.RFC3339),
			"end_time":    evt.EndTime.Format(time.RFC3339),
			"user_id":     evt.UserID,
			"created_at":  evt.CreatedAt.Format(time.RFC3339),
			"updated_at":  evt.UpdatedAt.Format(time.RFC3339),
		})
	}

	return models.NewSuccessResponse(
		event.ID,
		event.Type,
		map[string]interface{}{
			"events": eventsData,
			"count":  len(eventsData),
		},
	), nil
}

func (s *EventService) HandleDeleteUser(ctx context.Context, event models.Event) (models.EventResponse, error) {
	// Extraer el email del evento
	userID, ok := event.Data["user_id"].(string)
	if !ok || userID == "" {
		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("user_id es requerido"),
		), nil
	}

	s.logger.Info("Procesando evento de eliminación de usuario",
		zap.String("event_id", event.ID),
		zap.String("user_id", userID))

	// Eliminar el usuario a través del db_service
	err := s.dbClient.DeleteUser(ctx, userID)
	if err != nil {
		s.logger.Error("Error al eliminar usuario",
			zap.String("user_id", userID),
			zap.Error(err))

		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("error al eliminar usuario: %w", err),
		), nil
	}

	s.logger.Info("Usuario eliminado exitosamente",
		zap.String("user_id", userID))

	return models.NewSuccessResponse(
		event.ID,
		event.Type,
		map[string]interface{}{
			"message": "Usuario eliminado correctamente",
			"user_id": userID,
		},
	), nil
}

// HandleUpdateUser maneja la actualización de un usuario existente
func (s *EventService) HandleUpdateUser(ctx context.Context, event models.Event) (models.EventResponse, error) {
	// Extraer el ID del usuario y los datos a actualizar
	userID, ok := event.Data["user_id"].(string)
	if !ok || userID == "" {
		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("user_id es requerido"),
		), nil
	}

	// Crear un mapa con solo los campos actualizables
	updates := make(map[string]interface{})
	if email, ok := event.Data["email"].(string); ok && email != "" {
		updates["email"] = email
	}
	if username, ok := event.Data["username"].(string); ok && username != "" {
		updates["username"] = username
	}
	if password, ok := event.Data["password"].(string); ok && password != "" {
		updates["password"] = password
	}
	if isActive, ok := event.Data["is_active"].(bool); ok {
		updates["is_active"] = isActive
	}

	if len(updates) == 0 {
		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("no se proporcionaron campos para actualizar"),
		), nil
	}

	s.logger.Info("Procesando actualización de usuario",
		zap.String("event_id", event.ID),
		zap.String("user_id", userID),
		zap.Any("updates", updates))

	// Actualizar el usuario a través del db_service
	user, err := s.dbClient.UpdateUser(ctx, userID, updates)
	if err != nil {
		s.logger.Error("Error al actualizar usuario",
			zap.String("user_id", userID),
			zap.Error(err))

		return models.NewErrorResponse(
			event.ID,
			event.Type,
			fmt.Errorf("error al actualizar el usuario: %w", err),
		), nil
	}

	s.logger.Info("Usuario actualizado exitosamente",
		zap.String("user_id", userID))

	return models.NewSuccessResponse(
		event.ID,
		event.Type,
		map[string]interface{}{
			"id":        user.ID,
			"email":     user.Email,
			"username":  user.Username,
			"is_active": user.IsActive,
		},
	), nil
}
