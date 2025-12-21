package services

import (
	"context"
	"strings"

	"github.com/agenda-distribuida/api-gateway-service/internal/clients"
	"go.uber.org/zap"
)

type EventService struct {
	dbClient *clients.DBClient
	logger   *zap.Logger
}

func NewEventService(dbClient *clients.DBClient, logger *zap.Logger) *EventService {
	return &EventService{
		dbClient: dbClient,
		logger:   logger.Named("event_service"),
	}
}

// FindAndUpdateLeader busca y actualiza el líder del cluster Raft
func (s *EventService) FindAndUpdateLeader(ctx context.Context, raftNodes []string) error {
	return s.dbClient.FindAndUpdateLeader(ctx, raftNodes)
}

// UpdateRedisConnection actualiza la conexión Redis si el primary ha cambiado
func (s *EventService) UpdateRedisConnection(ctx context.Context, currentRedisURL string) (string, error) {
	// Obtener el Redis primary actual desde el DB service
	primary, err := s.dbClient.GetRedisPrimary(ctx)
	if err != nil {
		s.logger.Warn("No se pudo obtener el Redis primary", zap.Error(err))
		return currentRedisURL, err
	}

	// Asegurar que la URL tenga el esquema redis://
	if primary != "" && !strings.HasPrefix(primary, "redis://") {
		primary = "redis://" + primary
	}

	// Si el primary es diferente al actual, necesitamos reconectar
	if primary != currentRedisURL {
		s.logger.Info("Redis primary ha cambiado",
			zap.String("old", currentRedisURL),
			zap.String("new", primary))
		return primary, nil
	}

	return currentRedisURL, nil
}
