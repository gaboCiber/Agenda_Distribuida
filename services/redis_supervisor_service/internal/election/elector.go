package election

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type state int32

const (
	stateFollower state = iota
	stateCandidate
	stateLeader
)

type coordinatorInfo struct {
	leaderID string
	epoch    uint64
}

// Elector coordinates leader selection among redis-supervisor instances.
type Elector struct {
	id      string
	address string

	peers        map[string]string
	sortedPeerID []string

	server *grpc.Server
	lis    net.Listener

	clients   map[string]*grpc.ClientConn
	clientsMu sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc

	mu            sync.RWMutex
	currentState  state
	leaderID      string
	leaderEpoch   uint64
	lastHeartbeat time.Time

	primaryAddr   atomic.Value
	leadershipCh  chan bool
	coordinatorCh chan coordinatorInfo

	heartbeatInterval time.Duration
	heartbeatTimeout  time.Duration
	rpcTimeout        time.Duration

	electionActive int32

	logger *log.Logger

	lastLeaderAnnouncement time.Time
}

// NewElector creates a new Elector instance.
func NewElector(id, addr string, peers map[string]string, logger *log.Logger) (*Elector, error) {
	if id == "" {
		return nil, errors.New("elector requires non-empty id")
	}
	if addr == "" {
		return nil, errors.New("elector requires non-empty address")
	}

	sorted := make([]string, 0, len(peers)+1)
	sorted = append(sorted, id)
	for peerID := range peers {
		sorted = append(sorted, peerID)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return compareIDs(sorted[i], sorted[j]) < 0
	})

	ele := &Elector{
		id:                     id,
		address:                addr,
		peers:                  peers,
		sortedPeerID:           sorted,
		clients:                make(map[string]*grpc.ClientConn),
		currentState:           stateFollower,
		leaderID:               "",
		leaderEpoch:            0,
		lastHeartbeat:          time.Now(),
		heartbeatInterval:      2 * time.Second,
		heartbeatTimeout:       6 * time.Second,
		rpcTimeout:             900 * time.Millisecond,
		leadershipCh:           make(chan bool, 1),
		coordinatorCh:          make(chan coordinatorInfo, 1),
		logger:                 logger,
		lastLeaderAnnouncement: time.Now(),
	}

	if ele.logger == nil {
		ele.logger = log.Default()
	}

	return ele, nil
}

// Start initializes the gRPC server and background loops.
func (e *Elector) Start(parent context.Context) error {
	if parent == nil {
		parent = context.Background()
	}
	e.ctx, e.cancel = context.WithCancel(parent)

	lis, err := net.Listen("tcp", e.address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", e.address, err)
	}
	e.lis = lis

	e.server = grpc.NewServer()
	RegisterElectionServiceServer(e.server, e)

	go func() {
		if err := e.server.Serve(lis); err != nil {
			e.logger.Printf("gRPC server error: %v", err)
		}
	}()

	// Wait a bit for gRPC server to be ready
	time.Sleep(500 * time.Millisecond)

	go e.heartbeatLoop()
	go e.monitorLoop()

	// Add random delay to prevent simultaneous elections
	delay := time.Duration(rand.Intn(2000)) * time.Millisecond
	e.logger.Printf("[%s] starting election with random delay %v", e.id, delay)
	time.Sleep(delay)

	go e.startElection("initial startup")

	return nil
}

// Stop stops the elector and closes background resources.
func (e *Elector) Stop() {
	if e.cancel != nil {
		e.cancel()
	}
	if e.server != nil {
		e.server.GracefulStop()
	}
	if e.lis != nil {
		_ = e.lis.Close()
	}
	e.clientsMu.Lock()
	for id, conn := range e.clients {
		_ = conn.Close()
		delete(e.clients, id)
	}
	e.clientsMu.Unlock()
}

// IsLeader reports whether this instance currently acts as leader.
func (e *Elector) IsLeader() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.currentState == stateLeader
}

// LeadershipEvents emits true when this node becomes leader and false when it steps down.
func (e *Elector) LeadershipEvents() <-chan bool {
	return e.leadershipCh
}

// CurrentLeader returns the ID and epoch of the known leader.
func (e *Elector) CurrentLeader() (string, uint64) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.leaderID, e.leaderEpoch
}

// UpdatePrimary sets the Redis primary address for heartbeats/coordinator messages.
func (e *Elector) UpdatePrimary(addr string) {
	e.primaryAddr.Store(addr)
}

// WaitForLeadership blocks until this node becomes leader or ctx is cancelled.
func (e *Elector) WaitForLeadership(ctx context.Context) error {
	if e.IsLeader() {
		return nil
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case val := <-e.leadershipCh:
			if val {
				return nil
			}
		}
	}
}

func (e *Elector) initialElection() {
	// Give peers time to boot so the first election sees them.
	timer := time.NewTimer(500 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-e.ctx.Done():
		return
	}
	e.startElection("initial startup")
}

func (e *Elector) monitorLoop() {
	ticker := time.NewTicker(e.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			if e.IsLeader() {
				continue
			}

			e.mu.RLock()
			last := e.lastHeartbeat
			currentLeader := e.leaderID
			e.mu.RUnlock()

			timeSinceLastHeartbeat := time.Since(last)
			e.logger.Printf("[%s] time since last heartbeat: %v, current leader: %s", e.id, timeSinceLastHeartbeat, currentLeader)

			if timeSinceLastHeartbeat > e.heartbeatTimeout {
				e.logger.Printf("[%s] leader heartbeat timeout (last: %v ago, leader: %s)", e.id, timeSinceLastHeartbeat, currentLeader)
				e.startElection("leader heartbeat timeout")
			}
		}
	}
}

func (e *Elector) heartbeatLoop() {
	ticker := time.NewTicker(e.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			if !e.IsLeader() {
				continue
			}
			e.broadcast(MessageType_HEARTBEAT)
		}
	}
}

func (e *Elector) startElection(reason string) {
	if !atomic.CompareAndSwapInt32(&e.electionActive, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&e.electionActive, 0)

	e.logger.Printf("[%s] starting election: %s", e.id, reason)

	e.mu.Lock()
	// If we're already a leader, don't start new election
	if e.currentState == stateLeader {
		e.logger.Printf("[%s] already leader, not starting election", e.id)
		e.mu.Unlock()
		return
	}
	
	e.currentState = stateCandidate
	e.leaderID = ""
	e.mu.Unlock()

	higherPeers := e.higherPriorityPeers()
	e.logger.Printf("[%s] higher priority peers: %v", e.id, higherPeers)
	
	// According to bully algorithm: send election messages to higher priority peers
	// If no one responds, become leader
	responded := false
	
	for _, peerID := range higherPeers {
		addr := e.peers[peerID]
		e.logger.Printf("[%s] checking connectivity to higher priority peer %s at %s", e.id, peerID, addr)
		
		// First check connectivity with retry
		isConnected := false
		maxRetries := 3
		for retry := 0; retry < maxRetries; retry++ {
			if retry > 0 {
				e.logger.Printf("[%s] retry %d for peer %s", e.id, retry+1, peerID)
				time.Sleep(100 * time.Millisecond)
			}
			
			if e.checkPeerConnectivity(peerID, addr) {
				isConnected = true
				break
			}
		}
		
		if isConnected {
			e.logger.Printf("[%s] higher priority peer %s is responsive, waiting for coordinator", e.id, peerID)
			responded = true
			break // At least one higher priority peer is alive, wait for them
		} else {
			e.logger.Printf("[%s] higher priority peer %s is not responsive after %d attempts", e.id, peerID, maxRetries)
		}
	}

	e.logger.Printf("[%s] election result: responded=%v", e.id, responded)
	
	if !responded {
		// No higher priority peers responded, become leader according to bully algorithm
		e.logger.Printf("[%s] no higher priority peers responded, becoming leader", e.id)
		e.becomeLeader()
		return
	}

	// Wait for coordinator message from higher priority peer
	waitTimer := time.NewTimer(e.heartbeatTimeout)
	defer waitTimer.Stop()

	e.logger.Printf("[%s] waiting for coordinator message from higher priority peer", e.id)
	for {
		select {
		case <-e.ctx.Done():
			return
		case info := <-e.coordinatorCh:
			e.mu.Lock()
			e.currentState = stateFollower
			e.leaderID = info.leaderID
			e.leaderEpoch = info.epoch
			e.lastHeartbeat = time.Now()
			e.mu.Unlock()
			e.logger.Printf("[%s] accepted leader %s with epoch %d", e.id, info.leaderID, info.epoch)
			e.notifyLeadership(false)
			return
		case <-waitTimer.C:
			e.logger.Printf("[%s] election timeout waiting for coordinator, becoming leader", e.id)
			e.becomeLeader()
			return
		}
	}
}

func (e *Elector) becomeLeader() {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	// Double-check that we should still become leader
	if e.currentState != stateCandidate {
		e.logger.Printf("[%s] no longer candidate, not becoming leader", e.id)
		return
	}
	
	prevLeader := e.leaderID
	prevEpoch := e.leaderEpoch

	e.currentState = stateLeader
	e.leaderID = e.id
	e.lastHeartbeat = time.Now()

	newEpoch := uint64(time.Now().UnixNano())
	if newEpoch <= prevEpoch {
		newEpoch = prevEpoch + 1
	}
	e.leaderEpoch = newEpoch

	e.logger.Printf("[%s] became leader (prevLeader=%s, newEpoch=%d)", e.id, prevLeader, newEpoch)

	e.notifyLeadership(true)
	go e.broadcast(MessageType_COORDINATOR)
}

func messageTypeToString(mt MessageType) string {
	switch mt {
	case MessageType_UNKNOWN:
		return "UNKNOWN"
	case MessageType_ELECTION:
		return "ELECTION"
	case MessageType_OK:
		return "OK"
	case MessageType_COORDINATOR:
		return "COORDINATOR"
	case MessageType_HEARTBEAT:
		return "HEARTBEAT"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", mt)
	}
}

func (e *Elector) broadcast(msgType MessageType) {
	leaderID, epoch := e.CurrentLeader()
	primaryAddr, _ := e.primaryAddr.Load().(string)

	e.logger.Printf("[%s] broadcasting message type %d to all peers (leader=%s, epoch=%d)", e.id, msgType, leaderID, epoch)

	for peerID, addr := range e.peers {
		e.logger.Printf("[%s] sending %s message to %s at %s", e.id, messageTypeToString(msgType), peerID, addr)
		ctx, cancel := context.WithTimeout(e.ctx, e.rpcTimeout)
		msg := &ElectionMessage{
			Type:        msgType,
			SenderId:    e.id,
			Epoch:       epoch,
			LeaderId:    leaderID,
			PrimaryAddr: primaryAddr,
		}
		_, err := e.sendMessage(ctx, peerID, addr, msg)
		cancel()
		if err != nil {
			e.logger.Printf("[%s] broadcast to %s failed: %v", e.id, peerID, err)
		}
	}
}

func (e *Elector) higherPriorityPeers() []string {
	result := make([]string, 0)
	for _, peerID := range e.sortedPeerID {
		if peerID == e.id {
			continue
		}
		if compareIDs(peerID, e.id) > 0 {
			if _, ok := e.peers[peerID]; ok {
				result = append(result, peerID)
			}
		}
	}
	return result
}

func (e *Elector) lowerPriorityPeers() []string {
	result := make([]string, 0)
	for _, peerID := range e.sortedPeerID {
		if peerID == e.id {
			continue
		}
		if compareIDs(peerID, e.id) < 0 {
			if _, ok := e.peers[peerID]; ok {
				result = append(result, peerID)
			}
		}
	}
	return result
}

// SendMessage handles incoming gRPC requests from peers.
func (e *Elector) SendMessage(ctx context.Context, msg *ElectionMessage) (*ElectionMessage, error) {
	e.logger.Printf("[%s] SendMessage called with message: %+v", e.id, msg)
	
	if msg == nil {
		e.logger.Printf("[%s] received nil message", e.id)
		return nil, errors.New("nil message")
	}

	switch msg.Type {
	case MessageType_ELECTION:
		e.logger.Printf("[%s] processing ELECTION message from %s", e.id, msg.SenderId)
		e.handleElectionMessage(msg)
		return &ElectionMessage{Type: MessageType_OK, SenderId: e.id}, nil
	case MessageType_COORDINATOR:
		e.logger.Printf("[%s] processing COORDINATOR message from %s", e.id, msg.SenderId)
		e.handleCoordinatorMessage(msg)
		return &ElectionMessage{Type: MessageType_OK, SenderId: e.id}, nil
	case MessageType_HEARTBEAT:
		e.logger.Printf("[%s] processing HEARTBEAT message from %s", e.id, msg.SenderId)
		e.handleHeartbeatMessage(msg)
		return &ElectionMessage{Type: MessageType_OK, SenderId: e.id}, nil
	default:
		e.logger.Printf("[%s] received unknown message type %d from %s", e.id, msg.Type, msg.SenderId)
		return &ElectionMessage{Type: MessageType_UNKNOWN, SenderId: e.id}, nil
	}
}

func (e *Elector) handleElectionMessage(msg *ElectionMessage) {
	e.logger.Printf("[%s] received election message from %s", e.id, msg.SenderId)
	
	// If we have higher priority (higher ID), we should respond OK and start our own election
	if compareIDs(e.id, msg.SenderId) > 0 {
		e.logger.Printf("[%s] has higher priority than %s, responding OK and starting election", e.id, msg.SenderId)
		// Start our own election to compete for leadership
		go e.startElection("received election from lower priority peer")
		// The OK response is automatically handled by SendMessage function
		return
	} else {
		// We have lower priority, we should become follower and wait for coordinator
		e.logger.Printf("[%s] has lower priority than %s, becoming follower", e.id, msg.SenderId)
		e.mu.Lock()
		if e.currentState == stateCandidate {
			e.currentState = stateFollower
			e.leaderID = msg.SenderId
			e.lastHeartbeat = time.Now()
		}
		e.mu.Unlock()
		// The OK response is automatically handled by SendMessage function
		return
	}
}

func (e *Elector) handleCoordinatorMessage(msg *ElectionMessage) {
	e.logger.Printf("[%s] received COORDINATOR message from %s (leader=%s, epoch=%d)", e.id, msg.SenderId, msg.LeaderId, msg.Epoch)

	if msg.LeaderId == "" {
		e.logger.Printf("[%s] received COORDINATOR with empty leader ID, ignoring", e.id)
		return
	}

	e.mu.Lock()
	accept := e.shouldAcceptLeaderLocked(msg.LeaderId, msg.Epoch)
	if accept {
		e.logger.Printf("[%s] accepting leader %s with epoch %d", e.id, msg.LeaderId, msg.Epoch)
		if e.currentState == stateLeader && msg.LeaderId != e.id {
			e.logger.Printf("[%s] stepping down as leader in favor of %s", e.id, msg.LeaderId)
			go e.notifyLeadership(false)
		}
		e.currentState = stateFollower
		e.leaderID = msg.LeaderId
		e.leaderEpoch = msg.Epoch
		e.lastHeartbeat = time.Now()
		e.lastLeaderAnnouncement = time.Now()
	} else {
		e.logger.Printf("[%s] rejecting leader %s with epoch %d (current: %s/%d)", e.id, msg.LeaderId, msg.Epoch, e.leaderID, e.leaderEpoch)
	}
	e.mu.Unlock()

	if accept {
		select {
		case e.coordinatorCh <- coordinatorInfo{leaderID: msg.LeaderId, epoch: msg.Epoch}:
		default:
		}
	}
}

func (e *Elector) handleHeartbeatMessage(msg *ElectionMessage) {
	if msg.LeaderId == "" {
		e.logger.Printf("[%s] received HEARTBEAT with empty leader ID, ignoring", e.id)
		return
	}

	e.mu.Lock()
	accept := e.shouldAcceptLeaderLocked(msg.LeaderId, msg.Epoch)
	if accept {
		e.logger.Printf("[%s] accepting heartbeat from leader %s with epoch %d", e.id, msg.LeaderId, msg.Epoch)
		if e.currentState == stateLeader && msg.LeaderId != e.id {
			e.logger.Printf("[%s] stepping down as leader due to heartbeat from %s", e.id, msg.LeaderId)
			go e.notifyLeadership(false)
		}
		e.currentState = stateFollower
		e.leaderID = msg.LeaderId
		e.leaderEpoch = msg.Epoch
		e.lastHeartbeat = time.Now()
	} else {
		e.logger.Printf("[%s] rejecting heartbeat from %s with epoch %d (current: %s/%d)", e.id, msg.LeaderId, msg.Epoch, e.leaderID, e.leaderEpoch)
	}
	e.mu.Unlock()
}

func (e *Elector) shouldAcceptLeaderLocked(candidateID string, candidateEpoch uint64) bool {
	if candidateID == "" {
		return false
	}

	if candidateEpoch > e.leaderEpoch {
		return true
	}
	if candidateEpoch < e.leaderEpoch {
		return false
	}

	// Epoch tie: prefer higher ID to avoid oscillations, unless we already accept same leader.
	if e.leaderID == candidateID {
		return true
	}
	return compareIDs(candidateID, e.leaderID) > 0
}

func (e *Elector) notifyLeadership(becameLeader bool) {
	select {
	case e.leadershipCh <- becameLeader:
	default:
	}
}

func (e *Elector) checkPeerConnectivity(peerID, addr string) bool {
	e.logger.Printf("[%s] checking connectivity to %s at %s", e.id, peerID, addr)
	
	ctx, cancel := context.WithTimeout(e.ctx, e.rpcTimeout)
	defer cancel()
	
	msg := &ElectionMessage{Type: MessageType_ELECTION, SenderId: e.id}
	resp, err := e.sendMessage(ctx, peerID, addr, msg)
	
	if err != nil {
		e.logger.Printf("[%s] connectivity check to %s failed: %v", e.id, peerID, err)
		return false
	}
	
	if resp != nil && resp.Type == MessageType_OK && resp.SenderId != "" {
		e.logger.Printf("[%s] connectivity check to %s passed", e.id, peerID)
		return true
	}
	
	e.logger.Printf("[%s] connectivity check to %s failed: invalid response %+v", e.id, peerID, resp)
	return false
}

func (e *Elector) sendMessage(ctx context.Context, peerID, addr string, msg *ElectionMessage) (*ElectionMessage, error) {
	e.logger.Printf("[%s] attempting to send message to %s at %s", e.id, peerID, addr)
	e.logger.Printf("[%s] sending message: %+v", e.id, msg)
	
	conn, err := e.getOrDialClient(peerID, addr)
	if err != nil {
		e.logger.Printf("[%s] failed to get connection to %s: %v", e.id, peerID, err)
		return nil, err
	}
	
	client := NewElectionServiceClient(conn)
	resp, err := client.SendMessage(ctx, msg)
	if err != nil {
		e.logger.Printf("[%s] gRPC call to %s failed: %v", e.id, peerID, err)
		return nil, err
	}
	
	e.logger.Printf("[%s] successfully sent message to %s, received: %+v", e.id, peerID, resp)
	return resp, nil
}

func (e *Elector) getOrDialClient(peerID, addr string) (*grpc.ClientConn, error) {
	e.clientsMu.Lock()
	defer e.clientsMu.Unlock()

	if conn, ok := e.clients[peerID]; ok {
		return conn, nil
	}

	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to dial peer %s (%s): %w", peerID, addr, err)
	}
	e.clients[peerID] = conn
	return conn, nil
}

// compareIDs attempts numeric comparison first; falls back to lexical order.
func compareIDs(a, b string) int {
	return strings.Compare(a, b)
}

// GetID returns the ID of this elector instance
func (e *Elector) GetID() string {
	return e.id
}
