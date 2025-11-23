package consensus

import (
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/agenda-distribuida/db-service/internal/logger"
)

// RaftState define los posibles estados de un nodo Raft.
type RaftState int

const (
	Follower RaftState = iota
	Candidate
	Leader
)

// Constantes de tiempo
const (
	electionTimeoutMin time.Duration = 150 * time.Millisecond
	electionTimeoutMax time.Duration = 300 * time.Millisecond
	heartbeatInterval  time.Duration = 50 * time.Millisecond // Heartbeat interval should be less than election timeout
)

// LogEntry representa una entrada en el log de Raft.
// Contendrá el comando a ejecutar por la máquina de estados.
type LogEntry struct {
	Term    int         // El término en el que se recibió la entrada.
	Command interface{} // El comando para la máquina de estados (ej. una consulta SQL).
}

// RaftNode es la estructura principal que representa un nodo en el clúster de Raft.
type RaftNode struct {
	mu sync.Mutex // Mutex para proteger el acceso concurrente al estado del nodo.

	// --- Estado Persistente (debe guardarse en almacenamiento estable) ---
	currentTerm int        // Último término que el servidor ha visto.
	votedFor    string     // ID del candidato que recibió el voto en el término actual.
	log         []LogEntry // Entradas del log.

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

	// --- Logging ---
	logger *log.Logger // Logger personalizado para el nodo

	// Canales para la comunicación interna y el manejo de temporizadores.
	electionTimer     *time.Timer
	electionTimeout   time.Duration
	appendEntriesChan chan struct{} // Canal para resetear el temporizador al recibir AppendEntries.
	winElectionChan   chan bool     // Canal para señalar que la elección se ha ganado.
	heartbeatCount    int           // Contador para controlar la frecuencia de logs de heartbeat
}

// NewRaftNode crea e inicializa un nuevo nodo Raft.
func NewRaftNode(id string, peerAddress map[string]string) *RaftNode {
	// Inicializar el logger
	if err := logger.InitLogger("logs", id); err != nil {
		log.Fatalf("No se pudo inicializar el logger: %v", err)
	}

	rn := &RaftNode{
		id:                id,
		peerAddress:       peerAddress,
		state:             Follower,
		currentTerm:       0,
		votedFor:          "",
		log:               make([]LogEntry, 1), // Log ficticio en índice 0 para simplificar.
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
	}
	return rn
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
	logger.InfoLogger.Printf("[Nodo %s] INFO: Iniciando con %d peers: %v", rn.id, len(peers), strings.Join(peers, ", "))

	// Iniciar el servidor RPC en una gorutina.
	go rn.startRPCServer(rn.peerAddress[rn.id])

	// Esperar a que el servidor RPC esté listo.
	<-rn.serverReady
	logger.InfoLogger.Printf("[Nodo %s] INFO: Servidor RPC listo en %s.", rn.id, rn.peerAddress[rn.id])

	// Iniciar la gorutina que aplica logs a la máquina de estados.
	go rn.applyLogs()

	// Iniciar el bucle de estado principal del nodo.
	go rn.run()
}

// Propose es usado por el cliente para proponer un nuevo comando.
// Solo el líder puede procesar esta solicitud.
func (rn *RaftNode) Propose(command interface{}) (bool, int) {
	rn.mu.Lock()
	defer rn.mu.Unlock()

	if rn.state != Leader {
		return false, -1
	}

	entry := LogEntry{
		Term:    rn.currentTerm,
		Command: command,
	}
	rn.log = append(rn.log, entry)
	logger.InfoLogger.Printf("[Líder %s] INFO: Comando propuesto. Nuevo tamaño del log: %d", rn.id, len(rn.log))

	// No esperamos a que se replique, simplemente lo añadimos y el siguiente
	// heartbeat se encargará de enviarlo.
	return true, len(rn.log) - 1
}

// applyLogs es una gorutina que aplica logs comprometidos a la máquina de estados.
func (rn *RaftNode) applyLogs() {
	for range rn.applyChan {
		rn.mu.Lock()
		// Copiamos los índices para no mantener el lock durante la aplicación.
		lastApplied := rn.lastApplied
		commitIndex := rn.commitIndex
		entriesToApply := make([]LogEntry, 0)

		if commitIndex > lastApplied {
			entriesToApply = rn.log[lastApplied+1 : commitIndex+1]
		}
		rn.mu.Unlock()

		for i, entry := range entriesToApply {
			// Aquí es donde se aplicaría el comando a la máquina de estados real.
			// Por ahora, solo lo logueamos.
			logger.InfoLogger.Printf("[Nodo %s] Aplicando log %d: Comando='%v'", rn.id, lastApplied+1+i, entry.Command)
		}

		rn.mu.Lock()
		// Actualizamos lastApplied solo después de aplicar los logs.
		rn.lastApplied = commitIndex
		rn.mu.Unlock()
	}
}

// run es el bucle principal que gestiona el estado del nodo.
func (rn *RaftNode) run() {
	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()

	for {
		rn.mu.Lock()
		state := rn.state
		rn.mu.Unlock()

		switch state {
		case Follower:
			select {
			case <-rn.appendEntriesChan:
				rn.resetElectionTimer()
			case <-rn.electionTimer.C:
				rn.mu.Lock()
				logger.InfoLogger.Printf("[Nodo %s] INFO: Tiempo de espera agotado. Convirtiéndose en CANDIDATO para el término %d", rn.id, rn.currentTerm+1)
				rn.state = Candidate
				rn.mu.Unlock()
			}

		case Candidate:
			rn.mu.Lock()
			rn.startElection()
			rn.mu.Unlock()

			select {
			case <-rn.appendEntriesChan:
				rn.mu.Lock()
				if rn.state == Candidate {
					logger.InfoLogger.Printf("[Nodo %s] INFO: Descubierto nuevo líder. Volviendo a Follower.", rn.id)
					rn.state = Follower
				}
				rn.mu.Unlock()
				rn.resetElectionTimer()
			case <-rn.winElectionChan:
				logger.InfoLogger.Printf("[Nodo %s] INFO: Transición a Líder.", rn.id)
				rn.mu.Lock()
				rn.state = Leader
				rn.initializeLeaderState()
				rn.mu.Unlock()
			case <-rn.electionTimer.C:
				logger.InfoLogger.Printf("[Nodo %s] INFO: Elección fallida (timeout). Reiniciando.", rn.id)
			}

		case Leader:
			select {
			case <-heartbeatTicker.C:
				rn.sendHeartbeats()
			}
		}
	}
}

// randomElectionTimeout genera una duración aleatoria para el temporizador de elección.
func randomElectionTimeout() time.Duration {
	return electionTimeoutMin + time.Duration(rand.Int63n(int64(electionTimeoutMax-electionTimeoutMin)))
}

// resetElectionTimer reinicia el temporizador de elección del nodo.
func (rn *RaftNode) resetElectionTimer() {
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
	logger.InfoLogger.Printf("Nodo %s: Temporizador de elección reseteado a %s", rn.id, rn.electionTimeout)
}

// startElection inicia el proceso de elección para un nodo candidato.
func (rn *RaftNode) startElection() {
	// Incrementar el término actual.
	rn.currentTerm++
	// Votar por sí mismo.
	rn.votedFor = rn.id
	// Resetear el temporizador de elección.
	rn.resetElectionTimer()

	logger.InfoLogger.Printf("Nodo %s: Iniciando elección para el término %d", rn.id, rn.currentTerm)

	votes := 1 // Voto por sí mismo.

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

			logger.InfoLogger.Printf("Nodo %s: Enviando RequestVote a %s", rn.id, peerId)
			if err := rn.sendRPC(peerId, "RequestVote", &args, &reply); err != nil {
				logger.InfoLogger.Printf("Nodo %s: Error al enviar RequestVote a %s: %v", rn.id, peerId, err)
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
				logger.InfoLogger.Printf("Nodo %s: Término obsoleto. Volviendo a Follower.", rn.id)
				rn.currentTerm = reply.Term
				rn.state = Follower
				rn.votedFor = ""
				return
			}

			if reply.VoteGranted {
				votes++
				logger.InfoLogger.Printf("Nodo %s: Voto recibido de %s. Total de votos: %d", rn.id, peerId, votes)
				// Comprobar si hemos ganado la elección (mayoría).
				if votes > len(rn.peerAddress)/2 {
					logger.InfoLogger.Printf("Nodo %s: Elección ganada. Señalizando para convertirse en Líder.", rn.id)
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
	if rn.heartbeatCount%10 == 0 {
		logger.InfoLogger.Printf("Nodo %s: Enviando heartbeats/logs a seguidores... (término %d)", rn.id, term)
	}
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
		for _, peerID := range rn.peerAddress {
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
func (rn *RaftNode) becomeFollower(term int) {
	rn.state = Follower
	rn.currentTerm = term
	rn.votedFor = ""
	logger.InfoLogger.Printf("Nodo %s: Convertido a SEGUIDOR para el término %d", rn.id, term)
	rn.resetElectionTimer()
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
