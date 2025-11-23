package main

import (
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/agenda-distribuida/db-service/internal/consensus"
	"github.com/agenda-distribuida/db-service/internal/logger"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	// --- Configuración del Clúster ---
	// Mapa de IDs de nodo a sus direcciones de red.
	peerAddresses := map[string]string{
		"node1": "localhost:8011",
		"node2": "localhost:8012",
		"node3": "localhost:8013",
	}

	// --- Obtener ID del Nodo ---
	if len(os.Args) < 2 {
		log.Fatalf("Uso: go run main.go [node1|node2|node3]")
	}
	nodeID := os.Args[1]

	if _, ok := peerAddresses[nodeID]; !ok {
		log.Fatalf("ID de nodo inválido. Debe ser uno de: %v", getKeys(peerAddresses))
	}

	// Inicializar logger
	// Usar el directorio de trabajo actual/logs para los archivos de log
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("No se pudo obtener el directorio de trabajo actual: %v", err)
	}
	logDir := filepath.Join(cwd, "logs")

	if err := logger.InitLogger(logDir, nodeID); err != nil {
		log.Fatalf("No se pudo inicializar el logger: %v", err)
	}

	logger.InfoLogger.Printf("Iniciando nodo Raft: %s", nodeID)
	logger.InfoLogger.Printf("Directorio de logs: %s", logDir)

	// --- Crear y Lanzar el Nodo Raft ---
	raftNode := consensus.NewRaftNode(nodeID, peerAddresses)
	raftNode.Start()

	// Si este es el nodo1, intentar proponer un comando después de un tiempo
	// para dar tiempo a que se elija un líder.
	if nodeID == "node1" {
		go func() {
			time.Sleep(5 * time.Second) // Esperar a que se establezca un líder.

			logger.InfoLogger.Println("Intentando proponer un comando...")
			success, _ := raftNode.Propose("SET x = 10")
			if success {
				logger.InfoLogger.Println("Comando propuesto exitosamente por el líder.")
			} else {
				logger.InfoLogger.Println("Fallo al proponer el comando (probablemente no soy el líder).")
			}
		}()
	}

	// Mantener el programa en ejecución indefinidamente.
	// En una aplicación real, aquí estaría la lógica de la máquina de estados.
	select {}
}

// Función de ayuda para obtener las claves de un mapa (para el mensaje de error).
func getKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
