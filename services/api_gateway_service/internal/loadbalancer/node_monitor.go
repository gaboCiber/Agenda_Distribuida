package loadbalancer

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

type NodeMonitor struct {
	serviceType   string
	nodes         []string
	activeNodes   []string
	lastChecked   time.Time
	checkInterval time.Duration
	timeout       time.Duration
	mutex         sync.RWMutex
	stopChan      chan struct{}
	logger        *zap.Logger
}

// NewNodeMonitor crea una nueva instancia de NodeMonitor
func NewNodeMonitor(serviceType string, nodes []string, logger *zap.Logger) *NodeMonitor {
	return &NodeMonitor{
		serviceType:   serviceType,
		nodes:         nodes,
		activeNodes:   make([]string, 0),
		checkInterval: 30 * time.Second,
		timeout:       2 * time.Second,
		stopChan:      make(chan struct{}),
		logger:        logger.With(zap.String("service", serviceType)),
	}
}

// Start inicia el monitoreo periódico de los nodos
func (nm *NodeMonitor) Start() {
	nm.logger.Info("Iniciando monitoreo de nodos",
		zap.Strings("nodos", nm.nodes),
		zap.Duration("intervalo", nm.checkInterval))

	// Hacer la primera verificación de forma síncrona para tener nodos disponibles inmediatamente
	nm.logger.Info("Realizando verificación inicial de nodos")
	nm.checkAllNodes()

	// Inicia las verificaciones periódicas
	go func() {
		ticker := time.NewTicker(nm.checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Verificación periódica
				nm.checkAllNodes()
			case <-nm.stopChan:
				nm.logger.Info("Deteniendo monitoreo de nodos")
				return
			}
		}
	}()
}

// Stop detiene el monitoreo de nodos
func (nm *NodeMonitor) Stop() {
	close(nm.stopChan)
}

// GetActiveNodes devuelve una copia de la lista de nodos activos
func (nm *NodeMonitor) GetActiveNodes() []string {
	nm.mutex.RLock()
	defer nm.mutex.RUnlock()

	nodes := make([]string, len(nm.activeNodes))
	copy(nodes, nm.activeNodes)
	return nodes
}

// checkAllNodes verifica el estado de todos los nodos y actualiza la lista de activos
func (nm *NodeMonitor) checkAllNodes() {
	nm.logger.Debug("Iniciando checkAllNodes")

	var wg sync.WaitGroup
	var nodeStatus sync.Map

	// Verificar cada nodo en paralelo
	for _, node := range nm.nodes {
		wg.Add(1)
		go func(nodeURL string) {
			defer wg.Done()

			isActive, err := nm.checkNode(nodeURL)
			if err != nil {
				nm.logger.Warn("Error al verificar nodo",
					zap.String("nodo", nodeURL),
					zap.Error(err))
				nodeStatus.Store(nodeURL, false)
				return
			}

			if isActive {
				nodeStatus.Store(nodeURL, true)
			} else {
				nodeStatus.Store(nodeURL, false)
			}
		}(node)
	}

	// Esperar a que todas las verificaciones terminen
	wg.Wait()

	// Actualizar la lista de nodos activos
	var active []string
	nodeStatus.Range(func(key, value interface{}) bool {
		if value.(bool) {
			active = append(active, key.(string))
		}
		return true
	})

	nm.logger.Debug("Verificación completada",
		zap.Int("encontrados_activos", len(active)),
		zap.Int("actuales_activos", len(nm.activeNodes)),
		zap.Strings("nodos_activos", active))

	// Actualizar los nodos activos con lock
	nm.mutex.Lock()
	if len(active) != len(nm.activeNodes) {
		nm.logger.Info("Estado de nodos actualizado",
			zap.Int("activos", len(active)),
			zap.Int("totales", len(nm.nodes)),
			zap.Strings("nodos_activos", active))
	}
	nm.activeNodes = active
	nm.mutex.Unlock()
	nm.lastChecked = time.Now()
}

// checkNode verifica si un nodo específico está activo
func (nm *NodeMonitor) checkNode(nodeURL string) (bool, error) {
	nm.logger.Debug("Verificando nodo", zap.String("nodo", nodeURL))
	
	client := &http.Client{
		Timeout: nm.timeout,
	}

	// Construir la URL del endpoint de salud
	url := fmt.Sprintf("http://%s/health", nodeURL)
	nm.logger.Debug("Intentando conectar a", zap.String("url", url))

	// Crear contexto con timeout
	ctx, cancel := context.WithTimeout(context.Background(), nm.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		nm.logger.Error("Error creando petición", zap.String("nodo", nodeURL), zap.Error(err))
		return false, fmt.Errorf("error creando petición: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		nm.logger.Error("Error realizando petición", zap.String("nodo", nodeURL), zap.Error(err))
		return false, fmt.Errorf("error realizando petición: %w", err)
	}
	defer resp.Body.Close()

	nm.logger.Debug("Respuesta recibida", zap.String("nodo", nodeURL), zap.Int("status_code", resp.StatusCode))

	// Considerar el nodo como activo si responde con 200 OK
	isActive := resp.StatusCode == http.StatusOK
	nm.logger.Debug("Nodo verificado", zap.String("nodo", nodeURL), zap.Bool("activo", isActive))
	
	return isActive, nil
}
