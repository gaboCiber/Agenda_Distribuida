package services

import (
	"context"
	"fmt"
	"time"

	"github.com/agenda-distribuida/group-service/internal/clients"
	"github.com/agenda-distribuida/group-service/internal/raft"
	"go.uber.org/zap"
)

// RegistryService maneja el registro y descubrimiento de servicios
type RegistryService struct {
	client         *clients.ServiceRegistryClient
	leaderDiscover *raft.LeaderDiscovery
	raftNodes      []string
	logger         *zap.Logger
}

// NewRegistryService crea una nueva instancia de RegistryService
func NewRegistryService(raftNodes []string, logger *zap.Logger) *RegistryService {
	return &RegistryService{
		client:         clients.NewServiceRegistryClient("", logger),
		leaderDiscover: raft.NewLeaderDiscovery(logger),
		raftNodes:      raftNodes,
		logger:         logger,
	}
}

// RegisterService registra un servicio en el registro de servicios
func (s *RegistryService) RegisterService(serviceName, address string) error {
	// Actualizar el líder antes de registrar
	_, err := s.leaderDiscover.FindAndUpdateLeader(context.Background(), s.raftNodes)
	if err != nil {
		return fmt.Errorf("no se pudo encontrar el líder para registrar el servicio: %v", err)
	}

	// Configurar la URL del líder en el cliente
	s.client.SetBaseURL(s.leaderDiscover.GetLeaderURL())

	// Registrar el servicio
	return s.client.RegisterService(serviceName, address)
}

// StartHeartbeats inicia el envío periódico de latidos para mantener el servicio activo
func (s *RegistryService) StartHeartbeats(serviceName, address string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		// Actualizar el líder antes de cada heartbeat
		_, err := s.leaderDiscover.FindAndUpdateLeader(context.Background(), s.raftNodes)
		if err != nil {
			s.logger.Error("Error actualizando el líder Raft",
				zap.Error(err),
				zap.Strings("raft_nodes", s.raftNodes))
			continue
		}

		// Configurar la URL del líder en el cliente
		s.client.SetBaseURL(s.leaderDiscover.GetLeaderURL())

		// Enviar el heartbeat
		if err := s.client.SendHeartbeat(serviceName); err != nil {
			s.logger.Error("Error enviando heartbeat, intentando registrar de nuevo",
				zap.String("service", serviceName),
				zap.Error(err))

			// Reintentar registro
			if err := s.RegisterService(serviceName, address); err != nil {
				s.logger.Error("Error en el reintento de registro",
					zap.String("service", serviceName),
					zap.Error(err))
			}
		} else {
			s.logger.Debug("Heartbeat enviado exitosamente",
				zap.String("service", serviceName),
				zap.String("leader", s.leaderDiscover.GetLeaderURL()))
		}
	}
}
