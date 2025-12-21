package loadbalancer

import (
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"

	"go.uber.org/zap"
)

// LoadBalancer distribuye la carga entre múltiples nodos de servicio
type LoadBalancer struct {
	userMonitor  *NodeMonitor
	groupMonitor *NodeMonitor
	userCounter  int64
	groupCounter int64
	logger       *zap.Logger
}

// NewLoadBalancer crea una nueva instancia de LoadBalancer
func NewLoadBalancer(userNodes, groupNodes []string, logger *zap.Logger) *LoadBalancer {
	if logger == nil {
		logger = zap.NewNop()
	}

	lb := &LoadBalancer{
		userMonitor:  NewNodeMonitor("user", userNodes, logger),
		groupMonitor: NewNodeMonitor("group", groupNodes, logger),
		logger:       logger,
	}

	// Iniciar monitoreo de nodos
	lb.userMonitor.Start()
	lb.groupMonitor.Start()

	return lb
}

// parseNodeChannels convierte las URLs de nodos a nombres de canales de Redis
// Esta función ya no es necesaria ya que ahora usamos NodeMonitor
// Se mantiene por compatibilidad
func parseNodeChannels(nodes []string, prefix string) []string {
	channels := make([]string, 0, len(nodes))
	
	for i, node := range nodes {
		// Extraer el número del servicio del nombre
		serviceNum := i + 1 // Por defecto, asigna números secuenciales
		if num := extractServiceNumber(node); num > 0 {
			serviceNum = num
		}
		channels = append(channels, fmt.Sprintf("%s_%d", prefix, serviceNum))
	}
	
	return channels
}

// extractServiceNumber extrae el número del nombre del servicio o la URL
// Ejemplos:
//   - "agenda-user-service-1" -> 1
//   - "user-service-2:8080" -> 2
//   - "192.168.1.10:8080" -> 8080 (si no se puede extraer un número, se usa el puerto)
func extractServiceNumber(nodeURL string) int {
	// Primero intentar extraer un número del final del hostname
	parts := strings.Split(nodeURL, "-")
	if len(parts) > 1 {
		lastPart := parts[len(parts)-1]
		// Remover puerto si existe
		if colonIndex := strings.Index(lastPart, ":"); colonIndex != -1 {
			lastPart = lastPart[:colonIndex]
		}
		if num, err := strconv.Atoi(lastPart); err == nil {
			return num
		}
	}

	// Si no se pudo extraer un número del hostname, intentar con el puerto
	if colonIndex := strings.LastIndex(nodeURL, ":"); colonIndex != -1 && colonIndex < len(nodeURL)-1 {
		portStr := nodeURL[colonIndex+1:]
		if port, err := strconv.Atoi(portStr); err == nil {
			return port
		}
	}

	// Si no se pudo extraer ningún número, devolver 1 como valor por defecto
	return 1
}

// SelectUserNode selecciona el siguiente nodo de usuario usando round-robin
func (lb *LoadBalancer) SelectUserNode() string {
	activeNodes := lb.userMonitor.GetActiveNodes()
	if len(activeNodes) == 0 {
		lb.logger.Warn("No hay nodos de usuario activos, usando canal por defecto")
		return "user_events_1" // fallback
	}

	// Usar round-robin para seleccionar el siguiente nodo
	index := (atomic.AddInt64(&lb.userCounter, 1) - 1) % int64(len(activeNodes))
	
	// Convertir la URL del nodo a un canal de Redis
	nodeNumber := extractServiceNumber(activeNodes[index])
	return fmt.Sprintf("user_events_%d", nodeNumber)
}

// SelectGroupNode selecciona el siguiente nodo de grupo usando round-robin
func (lb *LoadBalancer) SelectGroupNode() string {
	activeNodes := lb.groupMonitor.GetActiveNodes()
	if len(activeNodes) == 0 {
		lb.logger.Warn("No hay nodos de grupo activos, usando canal por defecto")
		return "group_events_1" // fallback
	}

	// Usar round-robin para seleccionar el siguiente nodo
	index := (atomic.AddInt64(&lb.groupCounter, 1) - 1) % int64(len(activeNodes))
	
	// Convertir la URL del nodo a un canal de Redis
	nodeNumber := extractServiceNumber(activeNodes[index])
	return fmt.Sprintf("group_events_%d", nodeNumber)
}

// GetUserChannels devuelve todos los canales de usuario disponibles
func (lb *LoadBalancer) GetUserChannels() []string {
	activeNodes := lb.userMonitor.GetActiveNodes()
	channels := make([]string, 0, len(activeNodes))
	
	for _, node := range activeNodes {
		nodeNumber := extractServiceNumber(node)
		channels = append(channels, fmt.Sprintf("user_events_%d", nodeNumber))
	}
	
	return channels
}

// GetGroupChannels devuelve todos los canales de grupo disponibles
func (lb *LoadBalancer) GetGroupChannels() []string {
	activeNodes := lb.groupMonitor.GetActiveNodes()
	channels := make([]string, 0, len(activeNodes))
	
	for _, node := range activeNodes {
		nodeNumber := extractServiceNumber(node)
		channels = append(channels, fmt.Sprintf("group_events_%d", nodeNumber))
	}
	
	return channels
}

// GetAllResponseChannels devuelve todos los canales de respuesta activos
func (lb *LoadBalancer) GetAllResponseChannels() []string {
	userChannels := lb.GetUserChannels()
	groupChannels := lb.GetGroupChannels()
	
	responseChannels := make([]string, 0, len(userChannels)+len(groupChannels))
	
	// Agregar canales de respuesta de usuarios
	for _, ch := range userChannels {
		// Convertir de "user_events_X" a "users_events_response_X"
		responseCh := strings.Replace(ch, "user_events_", "users_events_response_", 1)
		responseChannels = append(responseChannels, responseCh)
	}
	
	// Agregar canales de respuesta de grupos
	for _, ch := range groupChannels {
		// Convertir de "group_events_X" a "groups_events_response_X"
		responseCh := strings.Replace(ch, "group_events_", "groups_events_response_", 1)
		responseChannels = append(responseChannels, responseCh)
	}
	
	lb.logger.Debug("Canales de respuesta actualizados",
		zap.Int("usuarios", len(userChannels)),
		zap.Int("grupos", len(groupChannels)))
	
	return responseChannels
}

// Stop detiene el monitoreo de nodos
func (lb *LoadBalancer) Stop() {
	if lb.userMonitor != nil {
		lb.userMonitor.Stop()
	}
	if lb.groupMonitor != nil {
		lb.groupMonitor.Stop()
	}
}
