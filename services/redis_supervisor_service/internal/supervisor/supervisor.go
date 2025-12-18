package supervisor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"redis_supervisor_service/internal/clients"
	"redis_supervisor_service/internal/config"
	"redis_supervisor_service/internal/election"
)

// Supervisor contains the core logic for monitoring and failover.
type Supervisor struct {
	config      *config.Config
	redisClient *clients.RedisClient
	dbClient    *clients.DBClient
	elector     *election.Elector

	stateMu         sync.RWMutex
	currentPrimary  string
	currentReplicas []string
	redisNodes      []string

	leaderCancelMu sync.Mutex
	leaderCancel   context.CancelFunc
	leaderWg       sync.WaitGroup
}

// New creates a new Supervisor
func New(cfg *config.Config, redisClient *clients.RedisClient, dbClient *clients.DBClient, elector *election.Elector) *Supervisor {
	return &Supervisor{
		config:      cfg,
		redisClient: redisClient,
		dbClient:    dbClient,
		elector:     elector,
		redisNodes:  cfg.RedisAddrs,
	}
}

// Run starts the main monitoring loops.
func (s *Supervisor) Run(ctx context.Context) {
	log.Println("Supervisor is starting, waiting for leadership...")
	for {
		select {
		case <-ctx.Done():
			s.stopLeaderLoops()
			return
		case isLeader := <-s.elector.LeadershipEvents():
			if isLeader {
				log.Println("Became leader, starting monitoring loops.")
				s.startLeaderLoops(ctx)
				// Immediately check if we need to initiate failover
				go s.immediateLeaderCheck(ctx)
			} else {
				log.Println("Lost leadership, stopping monitoring loops.")
				s.stopLeaderLoops()
			}
		}
	}
}

func (s *Supervisor) startLeaderLoops(ctx context.Context) {
	s.leaderCancelMu.Lock()
	defer s.leaderCancelMu.Unlock()

	// Create a new cancellable context for leader-specific tasks
	leaderCtx, cancel := context.WithCancel(ctx)
	s.leaderCancel = cancel

	s.leaderWg.Add(1)
	go func() {
		defer s.leaderWg.Done()
		// Initial attempt to find the primary
		for {
			err := s.findInitialPrimary()
			if err == nil {
				log.Printf("Initial primary found: %s. Replicas: %v.", s.currentPrimary, s.currentReplicas)
				s.synchronizeDB()
				s.elector.UpdatePrimary(s.currentPrimary)
				break
			}
			log.Printf("Failed to find initial primary: %v. Retrying in 5 seconds...", err)
			select {
			case <-time.After(5 * time.Second):
			case <-leaderCtx.Done():
				log.Println("Stopping initial primary search due to leadership loss.")
				return
			}
		}

		// Start the monitoring loops for primary and replicas
		s.leaderWg.Add(3)
		go s.monitorPrimaryLoop(leaderCtx)
		go s.clusterHealthCheckLoop(leaderCtx)
		go s.monitorAllNodesLoop(leaderCtx)
	}()
}

func (s *Supervisor) stopLeaderLoops() {
	s.leaderCancelMu.Lock()
	if s.leaderCancel != nil {
		s.leaderCancel()
		s.leaderCancel = nil
	}
	s.leaderCancelMu.Unlock()

	// Wait for all leader-related goroutines to finish
	s.leaderWg.Wait()
	log.Println("All leader loops have been stopped.")
}

func (s *Supervisor) synchronizeDB() {
	s.stateMu.RLock()
	primary := s.currentPrimary
	s.stateMu.RUnlock()

	if primary == "" {
		return
	}

	log.Println("Synchronizing DB service with current primary...")
	err := s.dbClient.SetRedisPrimary(primary)
	if err != nil {
		log.Printf("CRITICAL: Failed to synchronize DB service with primary %s: %v", primary, err)
	} else {
		log.Printf("DB service synchronized with primary %s.", primary)
	}
}

// findInitialPrimary queries all configured redis nodes to determine the primary and replica.
func (s *Supervisor) findInitialPrimary() error {
	log.Println("Searching for initial Redis primary among:", s.redisNodes)
	var foundPrimary string
	var foundReplicas []string

	for _, addr := range s.redisNodes {
		role, err := s.redisClient.GetRole(addr)
		if err != nil {
			log.Printf("Could not get role for %s: %v", addr, err)
			continue
		}

		if role == "master" {
			if foundPrimary != "" {
				return fmt.Errorf("split-brain detected: multiple primaries found (%s and %s)", foundPrimary, addr)
			}
			foundPrimary = addr
		} else {
			foundReplicas = append(foundReplicas, addr)
		}
	}

	if foundPrimary == "" {
		// No master found, try to promote one of the slaves
		log.Println("No master found, attempting to promote one of the slaves...")
		return s.attemptSlavePromotion()
	}

	s.stateMu.Lock()
	s.currentPrimary = foundPrimary
	s.currentReplicas = foundReplicas
	if len(foundReplicas) == 0 {
		log.Println("Warning: No replicas found.")
	}
	s.stateMu.Unlock()

	return nil
}

// monitorPrimaryLoop periodically pings the current primary and triggers a failover if it becomes unresponsive.
func (s *Supervisor) monitorPrimaryLoop(ctx context.Context) {
	defer s.leaderWg.Done()
	ticker := time.NewTicker(s.config.PingInterval)
	defer ticker.Stop()

	failureCount := 0

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping primary monitor loop.")
			return
		case <-ticker.C:
		}

		// Check if we're still the leader before pinging
		if !s.elector.IsLeader() {
			log.Println("No longer leader, stopping primary monitor loop.")
			return
		}

		s.stateMu.RLock()
		primary := s.currentPrimary
		s.stateMu.RUnlock()

		if primary == "" {
			continue
		}

		log.Printf("Pinging primary: %s", primary)
		err := s.redisClient.Ping(primary)
		if err != nil {
			failureCount++
			log.Printf("Ping failed for primary %s. Failure count: %d/%d. Error: %v", primary, failureCount, s.config.FailureThreshold, err)

			if failureCount >= s.config.FailureThreshold {
				log.Printf("Primary %s has reached failure threshold of %d. Initiating failover.", primary, s.config.FailureThreshold)
				s.initiateFailover()
				failureCount = 0
			}
		} else {
			if failureCount > 0 {
				log.Printf("Successfully pinged primary %s after %d failures. Resetting failure count.", primary, failureCount)
				failureCount = 0
			} else {
				log.Println("Ping successful.")
			}
		}
	}
}

// initiateFailover promotes the best available replica to be the new primary.
func (s *Supervisor) initiateFailover() {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	if len(s.currentReplicas) == 0 {
		log.Println("Cannot initiate failover: no replicas are configured or available.")
		return
	}

	var chosenReplica string
	for _, replica := range s.currentReplicas {
		_, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := s.redisClient.Ping(replica)
		cancel()
		if err == nil {
			chosenReplica = replica
			break
		} else {
			log.Printf("Replica %s is not healthy: %v", replica, err)
		}
	}

	if chosenReplica == "" {
		log.Println("Cannot initiate failover: no healthy replicas found.")
		return
	}

	log.Printf("Attempting to promote %s to primary...", chosenReplica)

	err := s.redisClient.PromoteToPrimary(chosenReplica)
	if err != nil {
		log.Printf("CRITICAL: Failed to promote replica %s: %v", chosenReplica, err)
		return
	}
	log.Printf("Successfully promoted %s to be the new primary.", chosenReplica)

	oldPrimaryAddr := s.currentPrimary
	s.currentPrimary = chosenReplica

	// Reconfigure all healthy replicas to point to the new primary
	var newReplicas []string
	for _, replica := range s.currentReplicas {
		if replica != chosenReplica {
			// Check if replica is healthy and reconfigure it
			if err := s.redisClient.Ping(replica); err == nil {
				log.Printf("Reconfiguring replica %s to point to new primary %s", replica, chosenReplica)
				if reconfigErr := s.redisClient.SetAsReplicaOf(replica, chosenReplica); reconfigErr != nil {
					log.Printf("Failed to reconfigure replica %s: %v", replica, reconfigErr)
				} else {
					log.Printf("Successfully reconfigured replica %s", replica)
					newReplicas = append(newReplicas, replica)
				}
			} else {
				log.Printf("Replica %s is not healthy, skipping reconfiguration", replica)
			}
		}
	}
	
	// Add old primary to replicas list if it's different from the new primary
	if oldPrimaryAddr != "" && oldPrimaryAddr != chosenReplica {
		newReplicas = append(newReplicas, oldPrimaryAddr)
	}
	
	s.currentReplicas = newReplicas

	log.Printf("Internal state updated. New primary: %s. New replicas: %v.", s.currentPrimary, s.currentReplicas)

	go s.synchronizeDB()
	go s.elector.UpdatePrimary(s.currentPrimary)

	log.Println("Failover complete.")
}

// clusterHealthCheckLoop periodically checks if the cluster has a primary and promotes one if needed
func (s *Supervisor) clusterHealthCheckLoop(ctx context.Context) {
	defer s.leaderWg.Done()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping cluster health check loop.")
			return
		case <-ticker.C:
		}

		s.stateMu.RLock()
		primary := s.currentPrimary
		s.stateMu.RUnlock()

		if primary != "" {
			err := s.redisClient.Ping(primary)
			if err == nil {
				role, err := s.redisClient.GetRole(primary)
				if err == nil && role == "master" {
					continue
				}
			}
		}

		log.Println("Cluster health check: No healthy primary found, attempting recovery...")
		s.attemptClusterRecovery()
	}
}

// attemptClusterRecovery tries to find and promote a new primary from available replicas
func (s *Supervisor) attemptClusterRecovery() {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	var healthyReplicas []string
	for _, replica := range s.currentReplicas {
		_, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := s.redisClient.Ping(replica)
		if err != nil {
			cancel()
			continue
		}
		role, err := s.redisClient.GetRole(replica)
		cancel()
		if err == nil && role == "slave" {
			healthyReplicas = append(healthyReplicas, replica)
		}
	}

	if len(healthyReplicas) == 0 {
		log.Println("No healthy replicas found for cluster recovery")
		return
	}

	chosenReplica := healthyReplicas[0]
	log.Printf("Attempting to promote %s to primary for cluster recovery...", chosenReplica)

	err := s.redisClient.PromoteToPrimary(chosenReplica)
	if err != nil {
		log.Printf("Failed to promote replica %s during cluster recovery: %v", chosenReplica, err)
		return
	}

	log.Printf("Successfully promoted %s to primary during cluster recovery", chosenReplica)

	oldPrimary := s.currentPrimary
	s.currentPrimary = chosenReplica

	// Reconfigure all healthy replicas to point to the new primary
	var newReplicas []string
	for _, replica := range s.currentReplicas {
		if replica != chosenReplica {
			// Check if replica is healthy and reconfigure it
			if err := s.redisClient.Ping(replica); err == nil {
				log.Printf("Reconfiguring replica %s to point to new primary %s (cluster recovery)", replica, chosenReplica)
				if reconfigErr := s.redisClient.SetAsReplicaOf(replica, chosenReplica); reconfigErr != nil {
					log.Printf("Failed to reconfigure replica %s: %v", replica, reconfigErr)
				} else {
					log.Printf("Successfully reconfigured replica %s (cluster recovery)", replica)
					newReplicas = append(newReplicas, replica)
				}
			} else {
				log.Printf("Replica %s is not healthy, skipping reconfiguration (cluster recovery)", replica)
			}
		}
	}
	
	// Add old primary to replicas list if it's different from the new primary
	if oldPrimary != "" && oldPrimary != chosenReplica {
		newReplicas = append(newReplicas, oldPrimary)
	}
	
	s.currentReplicas = newReplicas

	log.Printf("Cluster recovery completed. New primary: %s, Replicas: %v", s.currentPrimary, s.currentReplicas)

	go s.synchronizeDB()
	go s.elector.UpdatePrimary(s.currentPrimary)
}

// monitorAllNodesLoop monitors all Redis nodes and reconfigures recovered nodes
func (s *Supervisor) monitorAllNodesLoop(ctx context.Context) {
	defer s.leaderWg.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping all nodes monitor loop.")
			return
		case <-ticker.C:
		}

		// Check if we're still the leader before monitoring
		if !s.elector.IsLeader() {
			log.Println("No longer leader, stopping all nodes monitor loop.")
			return
		}

		s.stateMu.RLock()
		currentPrimary := s.currentPrimary
		s.stateMu.RUnlock()

		if currentPrimary == "" {
			continue // No primary to compare against
		}

		// Check all configured Redis nodes
		for _, nodeAddr := range s.redisNodes {
			// Skip if this node is already our current primary
			if nodeAddr == currentPrimary {
				continue
			}

			// Check if node is alive
			err := s.redisClient.Ping(nodeAddr)
			if err != nil {
				continue // Node is still down
			}

			// Node is alive, check its role
			role, err := s.redisClient.GetRole(nodeAddr)
			if err != nil {
				log.Printf("Could not get role for recovered node %s: %v", nodeAddr, err)
				continue
			}

			// If node thinks it's a master but we have a different master, reconfigure it
			if role == "master" {
				log.Printf("Recovered node %s thinks it's master, reconfiguring as replica of %s", nodeAddr, currentPrimary)
				err := s.redisClient.SetAsReplicaOf(nodeAddr, currentPrimary)
				if err != nil {
					log.Printf("Failed to reconfigure %s as replica: %v", nodeAddr, err)
				} else {
					log.Printf("Successfully reconfigured %s as replica of %s", nodeAddr, currentPrimary)

					// Update our internal state if this node wasn't in our replicas list
					s.stateMu.Lock()
					found := false
					for _, replica := range s.currentReplicas {
						if replica == nodeAddr {
							found = true
							break
						}
					}
					if !found {
						s.currentReplicas = append(s.currentReplicas, nodeAddr)
						log.Printf("Added %s to replicas list. Current replicas: %v", nodeAddr, s.currentReplicas)
					}
					s.stateMu.Unlock()
				}
			} else if role == "slave" {
				// Node is a replica, verify it's pointing to the correct master
				// We could add additional logic here to verify replication health
				s.stateMu.Lock()
				found := false
				for _, replica := range s.currentReplicas {
					if replica == nodeAddr {
						found = true
						break
					}
				}
				if !found {
					s.currentReplicas = append(s.currentReplicas, nodeAddr)
					log.Printf("Added existing replica %s to replicas list. Current replicas: %v", nodeAddr, s.currentReplicas)
				}
				s.stateMu.Unlock()
			}
		}
	}
}

// immediateLeaderCheck checks if the current primary is healthy immediately after becoming leader
func (s *Supervisor) immediateLeaderCheck(ctx context.Context) {
	// Give a brief moment for leader loops to initialize
	time.Sleep(1 * time.Second)
	
	s.stateMu.RLock()
	primary := s.currentPrimary
	s.stateMu.RUnlock()
	
	if primary == "" {
		log.Println("Immediate leader check: No primary set, attempting to find one...")
		// Try to find initial primary
		if err := s.findInitialPrimary(); err != nil {
			log.Printf("Immediate leader check: Could not find initial primary: %v", err)
			// If we can't find a primary, try cluster recovery
			log.Println("Immediate leader check: Starting cluster recovery...")
			s.attemptClusterRecovery()
		}
		return
	}
	
	log.Printf("Immediate leader check: Checking health of current primary %s", primary)
	err := s.redisClient.Ping(primary)
	if err != nil {
		log.Printf("Immediate leader check: Primary %s is not responding: %v", primary, err)
		log.Println("Immediate leader check: Initiating immediate failover...")
		s.initiateFailover()
	} else {
		// Check if it's actually still a master
		role, err := s.redisClient.GetRole(primary)
		if err != nil {
			log.Printf("Immediate leader check: Could not get role for %s: %v", primary, err)
			return
		}
		
		if role != "master" {
			log.Printf("Immediate leader check: %s is not a master (role: %s), initiating failover", primary, role)
			s.initiateFailover()
		} else {
			log.Printf("Immediate leader check: Primary %s is healthy and is master", primary)
		}
	}
}

// attemptSlavePromotion tries to promote a healthy slave to master
func (s *Supervisor) attemptSlavePromotion() error {
	var healthySlaves []string
	
	for _, addr := range s.redisNodes {
		role, err := s.redisClient.GetRole(addr)
		if err != nil {
			log.Printf("Could not get role for %s: %v", addr, err)
			continue
		}
		
		if role == "slave" {
			// Check if slave is healthy
			if err := s.redisClient.Ping(addr); err != nil {
				log.Printf("Slave %s is not healthy: %v", addr, err)
				continue
			}
			healthySlaves = append(healthySlaves, addr)
		}
	}
	
	if len(healthySlaves) == 0 {
		return errors.New("no healthy slaves found to promote")
	}
	
	// Choose the first healthy slave to promote
	chosenSlave := healthySlaves[0]
	log.Printf("Attempting to promote %s to master (no master found)", chosenSlave)
	
	err := s.redisClient.PromoteToPrimary(chosenSlave)
	if err != nil {
		return fmt.Errorf("failed to promote %s: %w", chosenSlave, err)
	}
	
	log.Printf("Successfully promoted %s to master", chosenSlave)
	
	// Update state
	s.stateMu.Lock()
	s.currentPrimary = chosenSlave
	
	// Add other slaves to replicas list
	var replicas []string
	for _, addr := range s.redisNodes {
		if addr != chosenSlave {
			replicas = append(replicas, addr)
		}
	}
	s.currentReplicas = replicas
	s.stateMu.Unlock()
	
	log.Printf("New cluster state - Primary: %s, Replicas: %v", chosenSlave, replicas)
	
	return nil
}
