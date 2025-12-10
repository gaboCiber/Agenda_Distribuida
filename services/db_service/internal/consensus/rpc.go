package consensus

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"strings"
	"time"

	"github.com/agenda-distribuida/db-service/internal/logger"
)

// --- Métodos RPC del Servidor ---

// RequestVote es el manejador RPC para que un candidato solicite un voto.
func (rn *RaftNode) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) error {
	rn.mu.Lock()
	defer rn.mu.Unlock()

	// 1. Responder falso si el término del candidato es menor que el nuestro.
	if args.Term < rn.currentTerm {
		reply.Term = rn.currentTerm
		reply.VoteGranted = false
		return nil
	}

	// 2. Si el término del candidato es mayor, nos convertimos en Follower.
	if args.Term > rn.currentTerm {
		rn.state = Follower
		rn.currentTerm = args.Term
		rn.votedFor = ""
		rn.persist()
	}

	reply.Term = rn.currentTerm

	// 3. Comprobar si ya hemos votado en este término.
	if rn.votedFor != "" && rn.votedFor != args.CandidateID {
		reply.VoteGranted = false
		return nil
	}

	// 4. Comprobar que el log del candidato esté al menos tan actualizado como el nuestro.
	lastLogTerm := rn.log[len(rn.log)-1].Term
	lastLogIndex := len(rn.log) - 1
	if args.LastLogTerm < lastLogTerm || (args.LastLogTerm == lastLogTerm && args.LastLogIndex < lastLogIndex) {
		reply.VoteGranted = false
		return nil
	}

	// Si todas las comprobaciones pasan, otorgamos el voto.
	rn.votedFor = args.CandidateID
	reply.VoteGranted = true
	rn.persist()
	log.Printf("[Nodo %s] Voto otorgado a %s para el término %d", rn.id, args.CandidateID, rn.currentTerm)

	// Al otorgar un voto, también reiniciamos nuestro propio temporizador de elección.
	rn.appendEntriesChan <- struct{}{}

	return nil
}

// AppendEntries maneja las solicitudes de AppendEntries (heartbeats y entradas de log).
func (rn *RaftNode) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) error {
	rn.mu.Lock()
	defer rn.mu.Unlock()

	// 1. Si el término del líder es menor que el nuestro, rechazar.
	if args.Term < rn.currentTerm {
		reply.Term = rn.currentTerm
		reply.Success = false
		return nil
	}

	// Si el término del RPC es mayor, nos convertimos en seguidor.
	if args.Term > rn.currentTerm {
		rn.becomeFollower(args.Term)
		rn.persist()
	}

	// Si el término del líder es igual o mayor que el nuestro, actualizamos el líder conocido.
	if args.Term >= rn.currentTerm {
		rn.leaderID = args.LeaderID
	}

	// En cualquier caso, si recibimos un AppendEntries de un líder legítimo
	// (con un término igual o mayor), reiniciamos nuestro temporizador.
	// Esto también nos convierte en seguidor si éramos candidatos.
	if rn.state == Candidate {
		rn.state = Follower
	}
	// Notificar al bucle principal que hemos recibido un heartbeat/RPC válido.
	select {
	case rn.appendEntriesChan <- struct{}{}:
	default:
	}

	reply.Term = rn.currentTerm

	// 2. Comprobar consistencia del log. Si la entrada en prevLogIndex no existe o
	// su término no coincide, rechazamos.
	if args.PrevLogIndex >= len(rn.log) || rn.log[args.PrevLogIndex].Term != args.PrevLogTerm {
		log.Printf("[Nodo %s] Rechazando AppendEntries por inconsistencia en el log. Nuestro log len: %d, PrevLogIndex: %d",
			rn.id, len(rn.log), args.PrevLogIndex)
		reply.Success = false
		return nil
	}

	// 3. Si hay nuevas entradas, verificar conflictos y añadirlas.
	// Si prevLogIndex existe y coincide, añadir nuevas entradas
	if len(args.Entries) > 0 {
		// Verificar si hay conflictos
		conflictIndex := -1
		for i, entry := range args.Entries {
			index := args.PrevLogIndex + 1 + i
			if index < len(rn.log) && rn.log[index].Term != entry.Term {
				conflictIndex = index
				break
			}
		}

		if conflictIndex != -1 {
			log.Printf("[Nodo %s] Conflicto de log detectado en el índice %d. Truncando log.", rn.id, conflictIndex)
			rn.log = rn.log[:conflictIndex]
			// Calcular el offset correcto en args.Entries
			entryOffset := conflictIndex - (args.PrevLogIndex + 1)
			if entryOffset >= 0 && entryOffset < len(args.Entries) {
				rn.log = append(rn.log, args.Entries[entryOffset:]...)
			}
		} else {
			// No hay conflictos, simplemente añadir las nuevas entradas
			if args.PrevLogIndex+1 < len(rn.log) {
				// Ya existen algunas entradas, reemplazarlas
				rn.log = rn.log[:args.PrevLogIndex+1]
			}
			rn.log = append(rn.log, args.Entries...)
		}
		rn.persist()
	}

	// 4. Si hay nuevas entradas que no están en el log, añadirlas.
	// (Esta lógica está implícita en el paso 3)

	// 5. Si el commitIndex del líder es mayor que el nuestro, actualizamos el nuestro.
	if args.LeaderCommit > rn.commitIndex {
		oldCommitIndex := rn.commitIndex
		lastNewEntryIndex := args.PrevLogIndex + len(args.Entries)
		rn.commitIndex = min(args.LeaderCommit, lastNewEntryIndex)
		
		// Notificar a applyChan si commitIndex cambió
		if rn.commitIndex > oldCommitIndex {
			select {
			case rn.applyChan <- struct{}{}:
			default:
				// No bloquear si el canal ya está lleno
			}
		}
	}

	if len(args.Entries) > 0 {
		log.Printf("[Nodo %s] Log replicado exitosamente hasta el índice %d", rn.id, len(rn.log)-1)
	}

	reply.Success = true
	return nil
}

// --- Infraestructura RPC ---

// startRPCServer inicia el servidor RPC para el nodo.
func (rn *RaftNode) startRPCServer(address string) {
	// Crear un nuevo servidor RPC
	server := rpc.NewServer()

	// Registrar manualmente los métodos RPC
	if err := server.RegisterName("RaftNode", rn); err != nil {
		log.Fatalf("[Nodo %s] Error al registrar el servicio RPC: %v", rn.id, err)
	}

	// Registrar los métodos manualmente para asegurar que estén disponibles
	server.HandleHTTP(rpc.DefaultRPCPath, rpc.DefaultDebugPath)

	// Configurar el mux HTTP
	mux := http.NewServeMux()
	mux.Handle(rpc.DefaultRPCPath, server)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Write([]byte("Raft Node RPC Server"))
			return
		}
		http.NotFound(w, r)
	})

	// Resolver la dirección para asegurarnos de que sea válida
	tcpAddr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		log.Fatalf("[Nodo %s] Error al resolver la dirección %s: %v", rn.id, address, err)
	}

	// Crear el listener
	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		logger.ErrorLogger.Printf("[Nodo %s] Error al iniciar el servidor RPC en %s: %v", rn.id, address, err)
	}

	// Obtener la dirección real en la que estamos escuchando (en caso de que se use el puerto 0)
	actualAddr := listener.Addr().(*net.TCPAddr)
	actualAddress := fmt.Sprintf("%s:%d", "localhost", actualAddr.Port)
	logger.InfoLogger.Printf("Nodo %s: Servidor RPC escuchando en %s", rn.id, listener.Addr())

	// Actualizar la dirección del peer con el puerto real asignado
	rn.mu.Lock()
	rn.peerAddress[rn.id] = actualAddress
	rn.mu.Unlock()

	// Crear el servidor HTTP
	httpServer := &http.Server{
		Addr:    actualAddress,
		Handler: mux,
	}

	// Señalar que estamos listos para aceptar conexiones
	rn.serverReady <- true

	// Iniciar el servidor en una goroutine
	go func() {
		logger.InfoLogger.Printf("[Nodo %s] Iniciando servidor HTTP en %s", rn.id, actualAddress)
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.ErrorLogger.Printf("Nodo %s: Error al iniciar el servidor RPC: %v", rn.id, err)
		}
	}()
}

// sendRPC realiza una llamada RPC a otro nodo con reintentos.
func (rn *RaftNode) sendRPC(peerId string, method string, args interface{}, reply interface{}) error {
	rn.mu.Lock()
	peerAddress, ok := rn.peerAddress[peerId]
	rn.mu.Unlock()
	
	if !ok {
		return fmt.Errorf("dirección de peer desconocida para %s", peerId)
	}

	var lastErr error
	maxRetries := 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Skip delay for first attempt
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*100) * time.Millisecond) // Exponential backoff
		}

		// Create a new client for each attempt
		client, err := rpc.DialHTTP("tcp", peerAddress)
		if err != nil {
			lastErr = fmt.Errorf("error al conectar con el peer %s (intento %d/%d): %w",
				peerAddress, attempt+1, maxRetries, err)
			logger.ErrorLogger.Printf("[Nodo %s] %v", rn.id, lastErr)
			continue
		}

		// Create a channel to handle the RPC call with a timeout
		done := make(chan error, 1)
		go func() {
			// El nombre del método debe ser "RaftNode.NombreDelMétodo"
			err := client.Call("RaftNode."+method, args, reply)
			done <- err
		}()

		// Set a timeout for the RPC call
		timeout := time.After(2 * time.Second)
		select {
		case err := <-done:
			client.Close()
			if err == nil {
				return nil // Success
			}
			
			lastErr = fmt.Errorf("error al llamar al método RaftNode.%s en %s (intento %d/%d): %w",
				method, peerAddress, attempt+1, maxRetries, err)
			logger.ErrorLogger.Printf("[Nodo %s] %v", rn.id, lastErr)

			// If the error is not a connection error, don't retry
			if !isNetworkError(err) {
				return lastErr
			}

		case <-timeout:
			client.Close()
			lastErr = fmt.Errorf("tiempo de espera agotado para el método RaftNode.%s en %s (intento %d/%d)",
				method, peerAddress, attempt+1, maxRetries)
			logger.ErrorLogger.Printf("[Nodo %s] %v", rn.id, lastErr)
		}
	}

	return lastErr
}

// isNetworkError checks if the error is a network-related error that might be worth retrying
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	// Check for network-related errors
	return strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "no route to host") ||
		strings.Contains(err.Error(), "network is unreachable")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
