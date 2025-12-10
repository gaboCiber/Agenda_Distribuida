package consensus

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/agenda-distribuida/db-service/internal/logger"
	"github.com/agenda-distribuida/db-service/internal/models"
	"github.com/agenda-distribuida/db-service/internal/repository"
	"github.com/google/uuid"
)

func init() {
	// Register DBCommand type for gob encoding/decoding
	gob.Register(DBCommand{})
	gob.Register(RaftStatus{})
}

// RaftState define los posibles estados de un nodo Raft.
type RaftState int

const (
	Follower RaftState = iota
	Candidate
	Leader
)

func (s RaftState) String() string {
	switch s {
	case Follower:
		return "Follower"
	case Candidate:
		return "Candidate"
	case Leader:
		return "Leader"
	default:
		return "Unknown"
	}
}

// RaftStatus holds a snapshot of a Raft node's internal state for introspection.
type RaftStatus struct {
	ID          string `json:"id"`
	State       string `json:"state"`
	Term        int    `json:"term"`
	CommitIndex int    `json:"commit_index"`
	LastApplied int    `json:"last_applied"`
	LeaderID    string `json:"leader_id"`
}

// Constantes de tiempo
const (
	electionTimeoutMin time.Duration = 3 * time.Second
	electionTimeoutMax time.Duration = 6 * time.Second
	heartbeatInterval  time.Duration = time.Second // Heartbeat interval should be less than election timeout
)

// DBCommand representa un comando de repositorio a ser replicado por Raft.
type DBCommand struct {
	Repository string // El nombre del repositorio a usar (ej. "UserRepository").
	Method     string // El método a llamar (ej. "Create").
	Payload    []byte // Los argumentos del método, serializados (ej. en JSON).
}

// LogEntry representa una entrada en el log de Raft.
// Contendrá el comando a ejecutar por la máquina de estados.
type LogEntry struct {
	Term    int       // El término en el que se recibió la entrada.
	Command DBCommand // El comando para la máquina de estados (ej. una consulta SQL).
}

// RaftNode es la estructura principal que representa un nodo en el clúster de Raft.
type RaftNode struct {
	mu sync.Mutex // Mutex para proteger el acceso concurrente al estado del nodo.

	// --- Estado Persistente (debe guardarse en almacenamiento estable) ---
	currentTerm int          // Último término que el servidor ha visto.
	votedFor    string       // ID del candidato que recibió el voto en el término actual.
	log         []LogEntry   // Entradas del log.
	stateDB     *RaftStateDB // Manejador de la base de datos para el estado persistente.

	// --- Estado Volátil (se pierde en reinicios) ---
	state       RaftState // Estado actual del nodo (Follower, Candidate, o Leader).
	commitIndex int       // Índice de la entrada de log más alta que se sabe que está comprometida.
	lastApplied int       // Índice de la entrada de log más alta aplicada a la máquina de estados.

	// --- Estado Volátil (solo para Líderes) ---
	nextIndex  map[string]int // Para cada servidor, índice de la próxima entrada de log a enviar.
	matchIndex map[string]int // Para cada servidor, índice de la entrada de log más alta que se sabe replicada.

	// --- Configuración y Comunicación ---
	id          string            // ID único de este nodo.
	peerAddress map[string]string // Mapa de IDs de peers a sus direcciones de red.
	serverReady chan bool         // Canal para señalar cuando el servidor está listo.
	applyChan   chan struct{}     // Canal para señalar que hay logs para aplicar.

	// --- Base de datos de la aplicación ---
	repositories map[string]interface{} // Mapa de repositorios base para aplicar comandos.

	// Canales para la comunicación interna y el manejo de temporizadores.
	electionTimer     *time.Timer
	electionTimeout   time.Duration
	appendEntriesChan chan struct{} // Canal para resetear el temporizador al recibir AppendEntries.
	winElectionChan   chan bool     // Canal para señalar que la elección se ha ganado.
	heartbeatCount    int           // Contador para controlar la frecuencia de logs de heartbeat
	voteCount         int32         // Contador atómico de votos recibidos en la elección actual

	// --- Para la linealizabilidad ---
	pendingCommands map[int]chan error // Mapa de índice de log a canal para notificar la aplicación del comando.

	// --- Estado del líder (para consultas externas) ---
	leaderID string // ID del líder actual (vacío si no se conoce o no es líder).
}

// NewRaftNode crea e inicializa un nuevo nodo Raft.
func NewRaftNode(id string, peerAddress map[string]string, baseDir string, repos map[string]interface{}) *RaftNode {
	// Inicializar el logger
	if err := logger.InitLogger("logs", id); err != nil {
		log.Fatalf("No se pudo inicializar el logger: %v", err)
	}

	// Inicializar la base de datos de persistencia
	dbDir := filepath.Join(baseDir, id)
	stateDB, err := NewRaftStateDB(dbDir)
	if err != nil {
		log.Fatalf("No se pudo inicializar la base de datos de Raft: %v", err)
	}

	// Cargar el estado persistente desde la base de datos.
	currentTerm, votedFor, logEntries, err := stateDB.LoadState()
	if err != nil {
		log.Fatalf("No se pudo cargar el estado de Raft: %v", err)
	}

	// Si el log está vacío (primer arranque), añadir la entrada ficticia.
	if len(logEntries) == 0 {
		logEntries = []LogEntry{{Term: 0, Command: DBCommand{}}}
	}

	rn := &RaftNode{
		id:                id,
		peerAddress:       peerAddress,
		state:             Follower,
		currentTerm:       currentTerm,
		votedFor:          votedFor,
		log:               logEntries,
		stateDB:           stateDB,
		commitIndex:       0,
		lastApplied:       0,
		nextIndex:         make(map[string]int),
		matchIndex:        make(map[string]int),
		appendEntriesChan: make(chan struct{}, 1),
		winElectionChan:   make(chan bool, 1),
		serverReady:       make(chan bool, 1), // Buffered channel to prevent deadlocks
		applyChan:         make(chan struct{}, 1),
		electionTimeout:   randomElectionTimeout(),
		electionTimer:     time.NewTimer(randomElectionTimeout()),
		heartbeatCount:    0,
		repositories:      repos,
		pendingCommands:   make(map[int]chan error),
		leaderID:          "", // Inicialmente no conocemos al líder.
	}
	return rn
}

// DEBE llamarse solo cuando el mutex ya está adquirido.
func (rn *RaftNode) persist() {
	if err := rn.stateDB.SaveState(rn.currentTerm, rn.votedFor, rn.log); err != nil {
		log.Fatalf("Error al persistir el estado de Raft: %v", err)
	}
	logger.InfoLogger.Printf("[Nodo %s]: Estado persistido. Término: %d, VotadoPor: %s, Tamaño del log: %d", rn.id, rn.currentTerm, rn.votedFor, len(rn.log))
}

// Start inicia el nodo Raft, incluyendo su servidor RPC y el bucle principal.
func (rn *RaftNode) Start() {
	// Mostrar información de los peers conocidos
	peers := make([]string, 0, len(rn.peerAddress)-1)
	for peerID, addr := range rn.peerAddress {
		if peerID != rn.id {
			peers = append(peers, fmt.Sprintf("%s (%s)", peerID, addr))
		}
	}
	logger.InfoLogger.Printf("[Nodo %s]: Iniciando con %d peers: %v", rn.id, len(peers), strings.Join(peers, ", "))

	// Iniciar el servidor RPC en una gorutina.
	go rn.startRPCServer(rn.peerAddress[rn.id])

	// Esperar a que el servidor RPC esté listo.
	<-rn.serverReady
	logger.InfoLogger.Printf("[Nodo %s]: Servidor RPC listo en %s.", rn.id, rn.peerAddress[rn.id])

	// Iniciar la gorutina que aplica logs a la máquina de estados.
	go rn.applyLogs()

	// Iniciar el bucle de estado principal del nodo.
	go rn.run()
}

// Propose es usado por el cliente para proponer un nuevo comando.
// Solo el líder puede procesar esta solicitud.
// Devuelve un canal que se cerrará cuando el comando sea aplicado a la máquina de estados.
func (rn *RaftNode) Propose(command DBCommand) (<-chan error, error) {
	rn.mu.Lock()
	defer rn.mu.Unlock()

	if rn.state != Leader {
		return nil, fmt.Errorf("no es el líder")
	}

	entry := LogEntry{
		Term:    rn.currentTerm,
		Command: command,
	}
	rn.log = append(rn.log, entry)
	newLogIndex := len(rn.log) - 1
	rn.persist()

	// Crear un canal para notificar la aplicación del comando.
	applyCh := make(chan error, 1)
	rn.pendingCommands[newLogIndex] = applyCh

	logger.InfoLogger.Printf("[Líder %s]: Comando propuesto en índice %d. Nuevo tamaño del log: %d", rn.id, newLogIndex, len(rn.log))

	// No esperamos a que se replique, simplemente lo añadimos y el siguiente
	// heartbeat se encargará de enviarlo.
	return applyCh, nil
}

// applyLogs es una gorutina que aplica logs comprometidos a la máquina de estados.
func (rn *RaftNode) applyLogs() {
	for range rn.applyChan {
		rn.mu.Lock()
		lastApplied := rn.lastApplied
		commitIndex := rn.commitIndex
		entriesToApply := make([]LogEntry, 0)

		if commitIndex > lastApplied {
			entriesToApply = rn.log[lastApplied+1 : commitIndex+1]
		}
		rn.mu.Unlock()

		for i, entry := range entriesToApply {
			idx := lastApplied + 1 + i
			var applyErr error

			// Dispatch the command to the correct repository and method.
			if entry.Command.Repository != "" {
				applyErr = rn.dispatchCommand(entry.Command)
				if applyErr != nil {
					logger.ErrorLogger.Printf("[Nodo %s] ERROR al aplicar log %d: %v", rn.id, idx, applyErr)
				}
			}

			rn.mu.Lock()
			if ch, ok := rn.pendingCommands[idx]; ok {
				ch <- applyErr
				close(ch)
				delete(rn.pendingCommands, idx)
			}
			rn.mu.Unlock()
		}

		rn.mu.Lock()
		rn.lastApplied = commitIndex
		rn.mu.Unlock()
	}
}

// dispatchCommand routes a command to the appropriate repository.
func (rn *RaftNode) dispatchCommand(cmd DBCommand) error {
	repo, ok := rn.repositories[cmd.Repository]
	if !ok {
		return fmt.Errorf("repositorio desconocido: %s", cmd.Repository)
	}

	switch cmd.Repository {
	case "UserRepository":
		userRepo := repo.(repository.UserRepository)
		switch cmd.Method {
		case "Create":
			var user models.User
			if err := json.Unmarshal(cmd.Payload, &user); err != nil {
				return fmt.Errorf("error al deserializar payload para UserRepository.Create: %w", err)
			}
			return userRepo.Create(context.Background(), &user)

		case "Update":
			type updatePayload struct {
				ID        uuid.UUID                 `json:"id"`
				UpdateReq *models.UpdateUserRequest `json:"update_req"`
			}
			var payload updatePayload
			if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
				return fmt.Errorf("error al deserializar payload para UserRepository.Update: %w", err)
			}
			_, err := userRepo.Update(context.Background(), payload.ID, payload.UpdateReq)
			return err

		case "Delete":
			var userID uuid.UUID
			if err := json.Unmarshal(cmd.Payload, &userID); err != nil {
				return fmt.Errorf("error al deserializar payload para UserRepository.Delete: %w", err)
			}
			return userRepo.Delete(context.Background(), userID)

		default:
			return fmt.Errorf("método desconocido para UserRepository: %s", cmd.Method)
		}
	case "EventRepository":
		eventRepo, ok := rn.repositories["EventRepository"].(repository.EventRepository)
		if !ok {
			return fmt.Errorf("EventRepository no encontrado en el mapa de repositorios")
		}

		switch cmd.Method {
		case "Create":
			// Handle the new payload structure with leader-generated ID
			type createPayload struct {
				ID          uuid.UUID `json:"id"`
				Title       string    `json:"title"`
				Description string    `json:"description"`
				StartTime   time.Time `json:"start_time"`
				EndTime     time.Time `json:"end_time"`
				UserID      uuid.UUID `json:"user_id"`
			}
			var payload createPayload
			if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
				return fmt.Errorf("error al deserializar payload para EventRepository.Create: %w", err)
			}

			// Create Event object from payload
			event := &models.Event{
				ID:          payload.ID,
				Title:       payload.Title,
				Description: payload.Description,
				StartTime:   payload.StartTime,
				EndTime:     payload.EndTime,
				UserID:      payload.UserID,
			}
			return eventRepo.Create(context.Background(), event)

		case "Update":
			type updatePayload struct {
				ID        uuid.UUID            `json:"id"`
				UpdateReq *models.EventRequest `json:"update_req"`
			}
			var payload updatePayload
			if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
				return fmt.Errorf("error al deserializar payload para EventRepository.Update: %w", err)
			}
			_, err := eventRepo.Update(context.Background(), payload.ID, payload.UpdateReq)
			return err

		case "Delete":
			var eventID uuid.UUID
			if err := json.Unmarshal(cmd.Payload, &eventID); err != nil {
				return fmt.Errorf("error al deserializar payload para EventRepository.Delete: %w", err)
			}
			return eventRepo.Delete(context.Background(), eventID)

		default:
			return fmt.Errorf("método desconocido para EventRepository: %s", cmd.Method)
		}
	case "GroupRepository":
		groupRepo, ok := rn.repositories["GroupRepository"].(repository.GroupRepository)
		if !ok {
			return fmt.Errorf("GroupRepository no encontrado en el mapa de repositorios")
		}

		switch cmd.Method {
		case "Create":
			// Handle the new payload structure with leader-generated ID
			type createPayload struct {
				ID             uuid.UUID  `json:"id"`
				Name           string     `json:"name"`
				Description    string     `json:"description"`
				CreatedBy      uuid.UUID  `json:"created_by"`
				IsHierarchical bool       `json:"is_hierarchical"`
				ParentGroupID  *uuid.UUID `json:"parent_group_id"`
			}
			var payload createPayload
			if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
				return fmt.Errorf("error al deserializar payload para GroupRepository.Create: %w", err)
			}

			// Create Group object from payload
			group := &models.Group{
				ID:             payload.ID,
				Name:           payload.Name,
				Description:    &payload.Description,
				CreatedBy:      payload.CreatedBy,
				IsHierarchical: payload.IsHierarchical,
				ParentGroupID:  payload.ParentGroupID,
			}
			return groupRepo.Create(context.Background(), group)

		case "Update":
			var group models.Group
			if err := json.Unmarshal(cmd.Payload, &group); err != nil {
				return fmt.Errorf("error al deserializar payload para GroupRepository.Update: %w", err)
			}
			return groupRepo.Update(context.Background(), &group)

		case "Delete":
			var groupID uuid.UUID
			if err := json.Unmarshal(cmd.Payload, &groupID); err != nil {
				return fmt.Errorf("error al deserializar payload para GroupRepository.Delete: %w", err)
			}
			return groupRepo.Delete(context.Background(), groupID)

		case "AddMember":
			// Handle the new payload structure with leader-generated ID
			type addMemberPayload struct {
				ID          uuid.UUID `json:"id"`
				GroupID     uuid.UUID `json:"group_id"`
				UserID      uuid.UUID `json:"user_id"`
				Role        string    `json:"role"`
				IsInherited bool      `json:"is_inherited"`
				JoinedAt    time.Time `json:"joined_at"`
			}
			var payload addMemberPayload
			if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
				return fmt.Errorf("error al deserializar payload para GroupRepository.AddMember: %w", err)
			}

			// Create GroupMember object from payload
			member := &models.GroupMember{
				ID:          payload.ID,
				GroupID:     payload.GroupID,
				UserID:      payload.UserID,
				Role:        payload.Role,
				IsInherited: payload.IsInherited,
				JoinedAt:    payload.JoinedAt,
			}
			return groupRepo.AddMember(context.Background(), member)

		case "UpdateGroupMember":
			type updateMemberPayload struct {
				GroupID uuid.UUID `json:"group_id"`
				UserID  uuid.UUID `json:"user_id"`
				Role    string    `json:"role"`
			}
			var payload updateMemberPayload
			if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
				return fmt.Errorf("error al deserializar payload para GroupRepository.UpdateGroupMember: %w", err)
			}
			return groupRepo.UpdateGroupMember(context.Background(), payload.GroupID, payload.UserID, payload.Role)

		case "RemoveMember":
			type removeMemberPayload struct {
				GroupID uuid.UUID `json:"group_id"`
				UserID  uuid.UUID `json:"user_id"`
			}
			var payload removeMemberPayload
			if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
				return fmt.Errorf("error al deserializar payload para GroupRepository.RemoveMember: %w", err)
			}
			return groupRepo.RemoveMember(context.Background(), payload.GroupID, payload.UserID)

		default:
			return fmt.Errorf("método desconocido para GroupRepository: %s", cmd.Method)
		}
	case "GroupEventRepository":
		groupEventRepo, ok := rn.repositories["GroupEventRepository"].(repository.GroupEventRepository)
		if !ok {
			return fmt.Errorf("GroupEventRepository no encontrado en el mapa de repositorios")
		}

		switch cmd.Method {
		case "AddGroupEvent":
			var groupEvent models.GroupEvent
			if err := json.Unmarshal(cmd.Payload, &groupEvent); err != nil {
				return fmt.Errorf("error al deserializar payload para GroupEventRepository.AddGroupEvent: %w", err)
			}
			return groupEventRepo.AddGroupEvent(context.Background(), &groupEvent)

		case "RemoveGroupEvent":
			type removeGroupEventPayload struct {
				GroupID uuid.UUID `json:"group_id"`
				EventID uuid.UUID `json:"event_id"`
			}
			var payload removeGroupEventPayload
			if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
				return fmt.Errorf("error al deserializar payload para GroupEventRepository.RemoveGroupEvent: %w", err)
			}
			return groupEventRepo.RemoveGroupEvent(context.Background(), payload.GroupID, payload.EventID)

		case "RemoveEventFromAllGroups":
			var eventID uuid.UUID
			if err := json.Unmarshal(cmd.Payload, &eventID); err != nil {
				return fmt.Errorf("error al deserializar payload para GroupEventRepository.RemoveEventFromAllGroups: %w", err)
			}
			return groupEventRepo.RemoveEventFromAllGroups(context.Background(), eventID)

		case "UpdateGroupEvent":
			type updateGroupEventPayload struct {
				GroupID        uuid.UUID          `json:"group_id"`
				EventID        uuid.UUID          `json:"event_id"`
				Status         models.EventStatus `json:"status"`
				IsHierarchical bool               `json:"is_hierarchical"`
			}
			var payload updateGroupEventPayload
			if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
				return fmt.Errorf("error al deserializar payload para GroupEventRepository.UpdateGroupEvent: %w", err)
			}
			_, err := groupEventRepo.UpdateGroupEvent(context.Background(), payload.GroupID, payload.EventID, payload.Status, payload.IsHierarchical)
			return err

		case "AddEventStatus":
			var status models.GroupEventStatus
			if err := json.Unmarshal(cmd.Payload, &status); err != nil {
				return fmt.Errorf("error al deserializar payload para GroupEventRepository.AddEventStatus: %w", err)
			}
			return groupEventRepo.AddEventStatus(context.Background(), &status)

		case "UpdateEventStatus":
			type updateEventStatusPayload struct {
				EventID uuid.UUID          `json:"event_id"`
				UserID  uuid.UUID          `json:"user_id"`
				Status  models.EventStatus `json:"status"`
			}
			var payload updateEventStatusPayload
			if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
				return fmt.Errorf("error al deserializar payload para GroupEventRepository.UpdateEventStatus: %w", err)
			}
			return groupEventRepo.UpdateEventStatus(context.Background(), payload.EventID, payload.UserID, payload.Status)

		case "CreateInvitation":
			var invitation models.GroupInvitation
			if err := json.Unmarshal(cmd.Payload, &invitation); err != nil {
				return fmt.Errorf("error al deserializar payload para GroupEventRepository.CreateInvitation: %w", err)
			}
			return groupEventRepo.CreateInvitation(context.Background(), &invitation)

		case "UpdateInvitation":
			type updateInvitationPayload struct {
				ID     uuid.UUID `json:"id"`
				Status string    `json:"status"`
			}
			var payload updateInvitationPayload
			if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
				return fmt.Errorf("error al deserializar payload para GroupEventRepository.UpdateInvitation: %w", err)
			}
			return groupEventRepo.UpdateInvitation(context.Background(), payload.ID, payload.Status)

		case "DeleteUserInvitations":
			var userID uuid.UUID
			if err := json.Unmarshal(cmd.Payload, &userID); err != nil {
				return fmt.Errorf("error al deserializar payload para GroupEventRepository.DeleteUserInvitations: %w", err)
			}
			return groupEventRepo.DeleteUserInvitations(context.Background(), userID)

		case "DeleteUserInvitation":
			var invitationID uuid.UUID
			if err := json.Unmarshal(cmd.Payload, &invitationID); err != nil {
				return fmt.Errorf("error al deserializar payload para GroupEventRepository.DeleteUserInvitation: %w", err)
			}
			return groupEventRepo.DeleteUserInvitation(context.Background(), invitationID)

		default:
			return fmt.Errorf("método desconocido para GroupEventRepository: %s", cmd.Method)
		}
	case "ConfigRepository":
		configRepo, ok := rn.repositories["ConfigRepository"].(*repository.ConfigRepository)
		if !ok {
			return fmt.Errorf("ConfigRepository no encontrado en el mapa de repositorios")
		}

		switch cmd.Method {
		case "Create":
			var config repository.Config
			if err := json.Unmarshal(cmd.Payload, &config); err != nil {
				return fmt.Errorf("error al deserializar payload para ConfigRepository.Create: %w", err)
			}
			return configRepo.Create(context.Background(), config)

		case "Update":
			var config repository.Config
			if err := json.Unmarshal(cmd.Payload, &config); err != nil {
				return fmt.Errorf("error al deserializar payload para ConfigRepository.Update: %w", err)
			}
			return configRepo.Update(context.Background(), config)

		case "Delete":
			var name string
			if err := json.Unmarshal(cmd.Payload, &name); err != nil {
				return fmt.Errorf("error al deserializar payload para ConfigRepository.Delete: %w", err)
			}
			return configRepo.Delete(context.Background(), name)

		default:
			return fmt.Errorf("método desconocido para ConfigRepository: %s", cmd.Method)
		}
	default:
		return fmt.Errorf("lógica de despacho no implementada para el repositorio: %s", cmd.Repository)
	}
}

// run es el bucle principal que gestiona el estado del nodo.
func (rn *RaftNode) run() {
	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()

	for {
		rn.mu.Lock()
		state := rn.state
		// Actualizar el leaderID si somos el líder
		if state == Leader {
			rn.leaderID = rn.id
		} else {
			// Si no somos el líder, intentamos descubrir quién es.
			// Esto es una simplificación; en un Raft real, el líder se comunica a los seguidores.
			// La lógica de AppendEntries ya actualiza el leaderID en el seguidor.
			// Si no somos el líder y no hemos recibido un heartbeat, el leaderID podría estar desactualizado.
			// Para una implementación más robusta, el líder debería enviar su ID en los heartbeats.
			// Por ahora, si no somos el líder, y no hemos recibido un voto reciente, asumimos que no conocemos al líder.
			if rn.state == Follower && rn.votedFor != "" {
				// Si somos follower y hemos votado por alguien, asumimos que es el líder (simplificación)
				rn.leaderID = rn.votedFor
			} else {
				rn.leaderID = ""
			}
		}
		rn.mu.Unlock()

		switch state {
		case Follower:
			select {
			case <-rn.appendEntriesChan:
				// Drenar el canal para evitar procesar múltiples mensajes
				for {
					select {
					case <-rn.appendEntriesChan:
						// Continuar drenando
					default:
						// Canal vacío, salir
						goto doneDraining
					}
				}
			doneDraining:
				logger.InfoLogger.Printf("[Nodo %s]: FOLLOWER del lider %s en el término %d", rn.id, rn.votedFor, rn.currentTerm+1)
				rn.resetElectionTimer()
			case <-rn.electionTimer.C:
				rn.mu.Lock()
				logger.InfoLogger.Printf("[Nodo %s]: Tiempo de espera agotado. Convirtiéndose en CANDIDATO para el término %d", rn.id, rn.currentTerm+1)
				rn.state = Candidate
				rn.mu.Unlock()
			}

		case Candidate:
			rn.mu.Lock()
			rn.startElection()
			rn.mu.Unlock()

			select {
			case <-rn.appendEntriesChan:
				// Drenar el canal
				for {
					select {
					case <-rn.appendEntriesChan:
					default:
						goto doneDrainingCandidate
					}
				}
			doneDrainingCandidate:
				rn.mu.Lock()
				if rn.state == Candidate {
					logger.InfoLogger.Printf("[Nodo %s]: Descubierto nuevo líder. Volviendo a Follower.", rn.id)
					rn.state = Follower
				}
				rn.mu.Unlock()
				rn.resetElectionTimer()
			case <-rn.winElectionChan:
				logger.InfoLogger.Printf("[Nodo %s]: Transición a Líder.", rn.id)
				rn.mu.Lock()
				rn.state = Leader
				rn.initializeLeaderState()
				rn.mu.Unlock()
			case <-rn.electionTimer.C:
				logger.InfoLogger.Printf("[Nodo %s]: Elección fallida (timeout). Reiniciando.", rn.id)
			}

		case Leader:
			<-heartbeatTicker.C
			rn.sendHeartbeats()
		}
	}
}

// randomElectionTimeout genera una duración aleatoria para el temporizador de elección.
func randomElectionTimeout() time.Duration {
	return electionTimeoutMin + time.Duration(rand.Int63n(int64(electionTimeoutMax-electionTimeoutMin)))
}

// resetElectionTimer reinicia el temporizador de elección del nodo.
// Thread-safe: adquiere el mutex internamente.
func (rn *RaftNode) resetElectionTimer() {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	rn.resetElectionTimerUnlocked()
}

// resetElectionTimerUnlocked reinicia el temporizador sin adquirir el mutex.
// DEBE llamarse solo cuando el mutex ya está adquirido.
func (rn *RaftNode) resetElectionTimerUnlocked() {
	if rn.electionTimer != nil {
		if !rn.electionTimer.Stop() {
			// Intenta drenar el canal si stop() devuelve false.
			select {
			case <-rn.electionTimer.C:
			default:
			}
		}
	}
	rn.electionTimeout = randomElectionTimeout()
	rn.electionTimer = time.NewTimer(rn.electionTimeout)
	logger.InfoLogger.Printf("[Nodo %s]: Temporizador de elección reseteado a %s", rn.id, rn.electionTimeout)
}

// startElection inicia el proceso de elección para un nodo candidato.
func (rn *RaftNode) startElection() {
	// Incrementar el término actual.
	rn.currentTerm++
	// Votar por sí mismo.
	rn.votedFor = rn.id
	// Persistir el nuevo término y el voto antes de enviar RPCs.
	rn.persist()
	// Resetear el temporizador de elección (ya tenemos el mutex).
	rn.resetElectionTimerUnlocked()
	// Inicializar contador de votos a 1 (voto por sí mismo)
	atomic.StoreInt32(&rn.voteCount, 1)

	logger.InfoLogger.Printf("[Nodo %s]: Iniciando elección para el término %d", rn.id, rn.currentTerm)

	// Enviar RPCs RequestVote a todos los demás nodos en paralelo.
	for peerId := range rn.peerAddress {
		if peerId == rn.id {
			continue
		}

		go func(peerId string) {
			rn.mu.Lock()
			// Verificar que seguimos siendo candidatos para el mismo término.
			if rn.state != Candidate {
				rn.mu.Unlock()
				return
			}
			args := RequestVoteArgs{
				Term:         rn.currentTerm,
				CandidateID:  rn.id,
				LastLogIndex: len(rn.log) - 1,
				LastLogTerm:  rn.log[len(rn.log)-1].Term,
			}
			rn.mu.Unlock()
			var reply RequestVoteReply

			logger.InfoLogger.Printf("[Nodo %s]: Enviando RequestVote a %s", rn.id, peerId)
			if err := rn.sendRPC(peerId, "RequestVote", &args, &reply); err != nil {
				logger.ErrorLogger.Printf("[Nodo %s]: Error al enviar RequestVote a %s: %v", rn.id, peerId, err)
				return
			}

			rn.mu.Lock()
			defer rn.mu.Unlock()

			// Verificar de nuevo el estado y el término por si cambiaron mientras esperábamos la respuesta.
			if rn.state != Candidate || rn.currentTerm != args.Term {
				return
			}

			if reply.Term > rn.currentTerm {
				// Descubrimos un término más alto, nos convertimos en Follower.
				logger.InfoLogger.Printf("[Nodo %s]: Término obsoleto. Volviendo a Follower.", rn.id)
				rn.currentTerm = reply.Term
				rn.state = Follower
				rn.votedFor = ""
				return
			}

			if reply.VoteGranted {
				// Incrementar contador atómico de votos
				newVoteCount := atomic.AddInt32(&rn.voteCount, 1)
				totalPeers := len(rn.peerAddress)
				majority := totalPeers/2 + 1

				logger.InfoLogger.Printf("[Nodo %s]: Voto recibido de %s. Total de votos: %d (mayoría necesaria: %d)",
					rn.id, peerId, newVoteCount, majority)

				// Verificar si tenemos mayoría
				if int(newVoteCount) >= majority {
					logger.InfoLogger.Printf("[Nodo %s]: Elección ganada. Señalizando para convertirse en Líder.", rn.id)
					select {
					case rn.winElectionChan <- true:
					default:
						// El canal ya está lleno, alguien más ya señaló la victoria.
					}
				}
			}
		}(peerId)
	}
}

// sendHeartbeats envía RPCs AppendEntries (posiblemente con logs) a todos los seguidores.
func (rn *RaftNode) sendHeartbeats() {
	rn.mu.Lock()
	if rn.state != Leader {
		rn.mu.Unlock()
		return
	}

	term := rn.currentTerm
	// Solo mostramos el log de heartbeats cada 10 envíos
	logger.InfoLogger.Printf("[Nodo %s]: Enviando heartbeats/logs a seguidores... (término %d)", rn.id, term)
	rn.heartbeatCount++
	rn.mu.Unlock()

	for peerId := range rn.peerAddress {
		if peerId == rn.id {
			continue
		}

		go func(peerId string) {
			rn.mu.Lock()
			// Lógica de consistencia: obtener el prevLogIndex y prevLogTerm
			nextIdx := rn.nextIndex[peerId]
			if nextIdx <= 0 {
				nextIdx = 1
			}
			prevLogIndex := nextIdx - 1
			prevLogTerm := rn.log[prevLogIndex].Term

			// Incluir entradas si hay nuevas para enviar a este peer.
			var entries []LogEntry
			if len(rn.log) > nextIdx {
				entries = rn.log[nextIdx:]
			}

			args := AppendEntriesArgs{
				Term:         term,
				LeaderID:     rn.id,
				PrevLogIndex: prevLogIndex,
				PrevLogTerm:  prevLogTerm,
				Entries:      entries,
				LeaderCommit: rn.commitIndex,
			}
			rn.mu.Unlock()

			var reply AppendEntriesReply
			if err := rn.sendRPC(peerId, "AppendEntries", &args, &reply); err != nil {
				// El error ya se loguea dentro de sendRPC
				return
			}

			rn.mu.Lock()
			defer rn.mu.Unlock()

			if reply.Term > rn.currentTerm {
				rn.becomeFollower(reply.Term)
				return
			}

			if rn.state == Leader && args.Term == rn.currentTerm {
				if reply.Success {
					// El seguidor aceptó las entradas.
					newNextIndex := args.PrevLogIndex + len(args.Entries) + 1
					rn.nextIndex[peerId] = newNextIndex
					rn.matchIndex[peerId] = newNextIndex - 1
					rn.updateCommitIndex() // Se intenta actualizar el commitIndex
				} else {
					// El seguidor rechazó por inconsistencia, retrocedemos nextIndex y reintentamos.
					rn.nextIndex[peerId]--
					if rn.nextIndex[peerId] < 1 {
						rn.nextIndex[peerId] = 1
					}
				}
			}
		}(peerId)
	}
}

// updateCommitIndex se ejecuta en el líder para avanzar el commitIndex.
func (rn *RaftNode) updateCommitIndex() {
	// El commitIndex debe ser al menos el valor que ya tiene.
	// Iteramos desde el final del log hacia atrás.
	for N := len(rn.log) - 1; N > rn.commitIndex; N-- {
		// Solo podemos comprometer logs de nuestro propio término.
		if rn.log[N].Term != rn.currentTerm {
			continue
		}

		// Contamos cuántos nodos han replicado hasta el índice N.
		matchCount := 1 // Nos contamos a nosotros mismos (el líder).
		for peerID := range rn.peerAddress {
			if peerID == rn.id {
				continue // Ya contamos al líder
			}
			if rn.matchIndex[peerID] >= N {
				matchCount++
			}
		}

		// Si una mayoría lo ha replicado, comprometemos el índice.
		if matchCount > len(rn.peerAddress)/2 {
			logger.InfoLogger.Printf("[Líder %s] INFO: Avanzando commitIndex a %d", rn.id, N)
			rn.commitIndex = N
			// Señalamos a la gorutina de aplicación que hay trabajo que hacer.
			select {
			case rn.applyChan <- struct{}{}:
			default: // No bloquear si el canal ya está lleno.
			}
			break // Salimos del bucle una vez que encontramos el N más alto.
		}
	}
}

// becomeFollower actualiza el estado del nodo a seguidor.

// DEBE llamarse cuando el mutex ya está adquirido.

func (rn *RaftNode) becomeFollower(term int) {
	rn.state = Follower
	rn.currentTerm = term
	rn.votedFor = ""
	rn.leaderID = "" // Resetear el líder conocido al convertirse en seguidor.
	logger.InfoLogger.Printf("[Nodo %s]: Convertido a SEGUIDOR para el término %d", rn.id, term)
	rn.resetElectionTimerUnlocked()
}

// initializeLeaderState inicializa el estado específico del líder.
func (rn *RaftNode) initializeLeaderState() {
	// Inicializar nextIndex y matchIndex para cada peer
	lastLogIndex := len(rn.log) - 1

	for peerID := range rn.peerAddress {
		if peerID == rn.id {
			continue // No nos enviamos RPCs a nosotros mismos
		}

		rn.nextIndex[peerID] = lastLogIndex + 1
		rn.matchIndex[peerID] = 0

	}

}

// --- Estructuras para las llamadas RPC ---

// IsLeader devuelve verdadero si el nodo actual es el líder.
func (rn *RaftNode) IsLeader() bool {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	return rn.state == Leader
}

// GetLeaderID devuelve el ID del líder actual. Si el nodo actual es el líder, devuelve su propio ID.
func (rn *RaftNode) GetLeaderID() string {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	return rn.leaderID
}

// GetLeaderAddress devuelve la dirección de red del líder actual.
func (rn *RaftNode) GetLeaderAddress() string {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	if rn.leaderID == "" {
		return "" // No hay líder conocido.
	}
	return rn.peerAddress[rn.leaderID]

}

// GetStatus returns a snapshot of the node's current status.
func (rn *RaftNode) GetStatus() RaftStatus {
	rn.mu.Lock()
	defer rn.mu.Unlock()

	return RaftStatus{
		ID:          rn.id,
		State:       rn.state.String(),
		Term:        rn.currentTerm,
		CommitIndex: rn.commitIndex,
		LastApplied: rn.lastApplied,
		LeaderID:    rn.leaderID,
	}
}

// Close cleans up resources used by the RaftNode
func (rn *RaftNode) Close() error {
	rn.mu.Lock()
	defer rn.mu.Unlock()

	// Close the stateDB if it's not nil
	if rn.stateDB != nil {
		return rn.stateDB.Close()
	}
	return nil
}

// RequestVoteArgs son los argumentos para la RPC RequestVote.
type RequestVoteArgs struct {
	Term         int    // Término del candidato.
	CandidateID  string // ID del candidato que solicita el voto.
	LastLogIndex int    // Índice del último log del candidato.
	LastLogTerm  int    // Término del último log del candidato.
}

// RequestVoteReply es la respuesta de la RPC RequestVote.
type RequestVoteReply struct {
	Term        int  // Término actual del votante, para que el candidato se actualice si es necesario.
	VoteGranted bool // Verdadero si el candidato recibió el voto.
}

// AppendEntriesArgs son los argumentos para la RPC AppendEntries (usada para replicación y heartbeats).
type AppendEntriesArgs struct {
	Term         int        // Término del líder.
	LeaderID     string     // ID del líder.
	PrevLogIndex int        // Índice del log inmediatamente anterior a las nuevas entradas.
	PrevLogTerm  int        // Término de la entrada en PrevLogIndex.
	Entries      []LogEntry // Entradas del log a almacenar (vacío para heartbeats).
	LeaderCommit int        // Índice del último log comprometido por el líder.
}

// AppendEntriesReply es la respuesta de la RPC AppendEntries.

type AppendEntriesReply struct {
	Term    int  // Término actual del seguidor, para que el líder se actualice si es necesario.
	Success bool // Verdadero si el seguidor contiene una entrada que coincide con PrevLogIndex y PrevLogTerm.
}
