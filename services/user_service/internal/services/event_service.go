package services

import (
	"context"
	"fmt"

	"github.com/agenda-distribuida/user-service/internal/clients"
	"github.com/agenda-distribuida/user-service/internal/models"
	"go.uber.org/zap"
)

type EventService struct {
	dbClient *clients.DBServiceClient
	logger   *zap.Logger
}

func NewEventService(dbClient *clients.DBServiceClient, logger *zap.Logger) *EventService {
	return &EventService{
		dbClient: dbClient,
		logger:   logger.Named("event_service"),
	}
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
