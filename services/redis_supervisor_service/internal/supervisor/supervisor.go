package supervisor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
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

			// Even if we can't find a primary, sync any existing primary state
			s.stateMu.RLock()
			existingPrimary := s.currentPrimary
			s.stateMu.RUnlock()
			if existingPrimary != "" {
				log.Printf("Using existing primary %s for DB synchronization", existingPrimary)
				s.synchronizeDB()
				s.elector.UpdatePrimary(existingPrimary)
			}

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
		log.Printf("Will retry DB synchronization in 10 seconds...")

		// Retry DB synchronization after 10 seconds
		go func() {
			time.Sleep(10 * time.Second)
			log.Printf("Retrying DB synchronization with primary %s...", primary)
			retryErr := s.dbClient.SetRedisPrimary(primary)
			if retryErr != nil {
				log.Printf("Retry failed: %v. Will continue trying...", retryErr)
			} else {
				log.Printf("DB service successfully synchronized with primary %s on retry.", primary)
			}
		}()
	} else {
		log.Printf("DB service synchronized with primary %s.", primary)
	}
}

// findInitialPrimary queries all configured redis nodes to determine the primary and replica.
func (s *Supervisor) findInitialPrimary() error {
	log.Println("Searching for initial Redis primary among:", s.redisNodes)
	var foundMasters []string
	var foundReplicas []string

	for _, addr := range s.redisNodes {
		role, err := s.redisClient.GetRole(addr)
		if err != nil {
			log.Printf("Could not get role for %s: %v", addr, err)
			continue
		}

		if role == "master" {
			foundMasters = append(foundMasters, addr)
		} else {
			foundReplicas = append(foundReplicas, addr)
		}
	}

	if len(foundMasters) == 0 {
		// No master found, try to promote one of the slaves
		log.Println("No master found, attempting to promote one of the slaves...")
		return s.attemptSlavePromotion()
	} else if len(foundMasters) == 1 {
		// Single master found - healthy state
		s.stateMu.Lock()
		s.currentPrimary = foundMasters[0]
		s.currentReplicas = foundReplicas
		if len(foundReplicas) == 0 {
			log.Println("Warning: No replicas found.")
		}
		s.stateMu.Unlock()
		return nil
	} else {
		// Multiple masters found - split-brain
		log.Printf("split-brain detected: %d primaries found (%v), attempting resolution", len(foundMasters), foundMasters)
		return s.resolveMultipleMasters(foundMasters, foundReplicas)
	}
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

	var healthyReplicas []string
	for _, replica := range s.currentReplicas {
		_, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := s.redisClient.Ping(replica)
		cancel()
		if err == nil {
			healthyReplicas = append(healthyReplicas, replica)
		} else {
			log.Printf("Replica %s is not healthy: %v", replica, err)
		}
	}

	if len(healthyReplicas) == 0 {
		log.Println("Cannot initiate failover: no healthy replicas found.")
		return
	}

	// Choose the replica with highest priority (lexicographically highest) for consistency
	chosenReplica := s.selectPreferredMasterFromList(healthyReplicas)

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

	// Reconfigure all reachable nodes to point to the new primary
	go s.reconfigureAllNodes(s.currentPrimary)
	go s.synchronizeDB()
	go s.elector.UpdatePrimary(s.currentPrimary)

	log.Println("Failover complete.")
}

// clusterHealthCheckLoop periodically checks if the cluster has a primary and promotes one if needed
func (s *Supervisor) clusterHealthCheckLoop(ctx context.Context) {
	defer s.leaderWg.Done()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	log.Println("Starting cluster health check loop (15s interval)...")

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping cluster health check loop.")
			return
		case <-ticker.C:
			log.Println("Cluster health check: scanning cluster state...")
		}

		// Check for split-brain scenario
		masters, replicas, err := s.scanClusterState()
		if err != nil {
			log.Printf("Cluster health check: Failed to scan cluster state: %v", err)
			continue
		}

		if len(masters) == 0 {
			log.Println("Cluster health check: No primary found, attempting recovery...")
			s.attemptClusterRecovery()
		} else if len(masters) > 1 {
			log.Printf("Cluster health check: Split-brain detected with %d masters: %v", len(masters), masters)
			s.resolveMultipleMasters(masters, replicas)
		} else {
			// Exactly 1 master found - leader ensures DB is synchronized
			foundMaster := masters[0]

			// Leader's responsibility: keep DB synchronized with actual master
			go s.synchronizeDB()

			log.Printf("Cluster health check: Master: %s, Replicas: %d", foundMaster, len(replicas))
		}
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

	// Choose the replica with highest priority (lexicographically highest) for consistency
	chosenReplica := s.selectPreferredMasterFromList(healthyReplicas)
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

	// Reconfigure all reachable nodes to point to the new primary
	go s.reconfigureAllNodes(s.currentPrimary)
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

	// Choose the slave with highest priority (lexicographically highest) for consistency
	chosenSlave := s.selectPreferredMasterFromList(healthySlaves)
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

	// Reconfigure all reachable nodes to point to the new primary
	go s.reconfigureAllNodes(chosenSlave)

	// Synchronize with DB service and elector
	go s.synchronizeDB()
	go s.elector.UpdatePrimary(chosenSlave)

	return nil
}

// reconfigureAllNodes configures all reachable Redis nodes (except the primary) as replicas
func (s *Supervisor) reconfigureAllNodes(primaryAddr string) {
	log.Printf("Reconfiguring all reachable nodes to point to primary %s", primaryAddr)

	for _, nodeAddr := range s.redisNodes {
		// Skip the primary itself
		if nodeAddr == primaryAddr {
			continue
		}

		// Check if node is reachable
		if err := s.redisClient.Ping(nodeAddr); err != nil {
			log.Printf("Node %s is not reachable, skipping reconfiguration: %v", nodeAddr, err)
			continue
		}

		// Check current role
		role, err := s.redisClient.GetRole(nodeAddr)
		if err != nil {
			log.Printf("Could not get role for node %s: %v", nodeAddr, err)
			continue
		}

		// If node is not already a replica of the correct primary, reconfigure it
		if role == "master" {
			log.Printf("Reconfiguring master %s as replica of %s", nodeAddr, primaryAddr)
			if err := s.redisClient.SetAsReplicaOf(nodeAddr, primaryAddr); err != nil {
				log.Printf("Failed to reconfigure %s as replica: %v", nodeAddr, err)
			} else {
				log.Printf("Successfully reconfigured %s as replica of %s", nodeAddr, primaryAddr)
			}
		} else if role == "slave" {
			// Check if it's already pointing to the correct master
			masterHost, err := s.redisClient.GetMasterHost(nodeAddr)
			if err != nil {
				log.Printf("Could not get master host for %s: %v", nodeAddr, err)
				// Try reconfiguring anyway
				if err := s.redisClient.SetAsReplicaOf(nodeAddr, primaryAddr); err != nil {
					log.Printf("Failed to reconfigure replica %s: %v", nodeAddr, err)
				} else {
					log.Printf("Successfully reconfigured replica %s to point to %s", nodeAddr, primaryAddr)
				}
				continue
			}

			// Extract just the hostname part (without port) for comparison
			expectedMaster := strings.Split(primaryAddr, ":")[0]
			if masterHost != expectedMaster {
				log.Printf("Reconfiguring replica %s from master %s to %s", nodeAddr, masterHost, primaryAddr)
				if err := s.redisClient.SetAsReplicaOf(nodeAddr, primaryAddr); err != nil {
					log.Printf("Failed to reconfigure replica %s: %v", nodeAddr, err)
				} else {
					log.Printf("Successfully reconfigured replica %s to point to %s", nodeAddr, primaryAddr)
				}
			} else {
				log.Printf("Replica %s is already pointing to correct master %s", nodeAddr, masterHost)
			}
		}
	}

	log.Printf("Node reconfiguration completed for primary %s", primaryAddr)
}

// resolveSplitBrain handles the case where multiple Redis masters are detected
func (s *Supervisor) resolveSplitBrain(master1, master2 string, currentReplicas []string) error {
	log.Printf("Resolving split-brain between %s and %s", master1, master2)

	// Choose the master with higher priority (simple selection: choose the one with higher port/IP)
	chosenMaster := s.selectPreferredMaster(master1, master2)
	demotedMaster := master2
	if chosenMaster == master2 {
		demotedMaster = master1
	}

	log.Printf("Choosing %s as primary, demoting %s to replica", chosenMaster, demotedMaster)

	// Step 1: Force the demoted master to become a replica of the chosen master
	err := s.redisClient.SetAsReplicaOf(demotedMaster, chosenMaster)
	if err != nil {
		log.Printf("Failed to demote %s to replica of %s: %v", demotedMaster, chosenMaster, err)
		// Try the other way around
		chosenMaster = demotedMaster
		demotedMaster = master1
		log.Printf("Trying alternative: choosing %s as primary, demoting %s", chosenMaster, demotedMaster)

		err = s.redisClient.SetAsReplicaOf(demotedMaster, chosenMaster)
		if err != nil {
			return fmt.Errorf("failed to resolve split-brain: could not demote either master: %v", err)
		}
	}

	log.Printf("Successfully demoted %s to replica of %s", demotedMaster, chosenMaster)

	// Step 2: Update internal state
	s.stateMu.Lock()
	s.currentPrimary = chosenMaster

	// Build new replicas list
	var newReplicas []string
	newReplicas = append(newReplicas, demotedMaster)      // Add the demoted master
	newReplicas = append(newReplicas, currentReplicas...) // Add existing replicas

	s.currentReplicas = newReplicas
	s.stateMu.Unlock()

	// Step 3: Reconfigure all reachable nodes to point to the correct master
	go s.reconfigureAllNodes(chosenMaster)

	// Step 4: Synchronize with DB service
	go s.synchronizeDB()
	go s.elector.UpdatePrimary(chosenMaster)

	log.Printf("Split-brain resolved. New primary: %s, Replicas: %v", chosenMaster, newReplicas)
	return nil
}

// selectPreferredMaster chooses which master to keep based on consistent criteria
func (s *Supervisor) selectPreferredMaster(master1, master2 string) string {
	// Simple selection: choose the one with lexicographically higher address
	// This ensures consistent selection across all supervisors
	if master1 > master2 {
		return master1
	}
	return master2
}

// scanClusterState scans all Redis nodes and returns masters and replicas
func (s *Supervisor) scanClusterState() ([]string, []string, error) {
	var masters []string
	var replicas []string

	for _, addr := range s.redisNodes {
		role, err := s.redisClient.GetRole(addr)
		if err != nil {
			log.Printf("Could not get role for %s during cluster scan: %v", addr, err)
			continue
		}

		if role == "master" {
			masters = append(masters, addr)
		} else {
			replicas = append(replicas, addr)
		}
	}

	return masters, replicas, nil
}

// resolveMultipleMasters handles the case where multiple Redis masters are detected (N > 2)
func (s *Supervisor) resolveMultipleMasters(masters []string, currentReplicas []string) error {
	log.Printf("Resolving split-brain with %d masters: %v", len(masters), masters)

	// Choose the master with highest priority (lexicographically highest)
	chosenMaster := s.selectPreferredMasterFromList(masters)

	var demotedMasters []string
	for _, master := range masters {
		if master != chosenMaster {
			demotedMasters = append(demotedMasters, master)
		}
	}

	log.Printf("Choosing %s as primary, demoting %d masters: %v", chosenMaster, len(demotedMasters), demotedMasters)

	// Step 1: Force all other masters to become replicas of the chosen master
	for _, demotedMaster := range demotedMasters {
		err := s.redisClient.SetAsReplicaOf(demotedMaster, chosenMaster)
		if err != nil {
			log.Printf("Failed to demote %s to replica of %s: %v", demotedMaster, chosenMaster, err)
			// If we can't demote this one, try choosing another master
			// Remove this master from candidates and retry
			var remainingMasters []string
			for _, m := range masters {
				if m != demotedMaster {
					remainingMasters = append(remainingMasters, m)
				}
			}
			if len(remainingMasters) <= 1 {
				return fmt.Errorf("failed to resolve split-brain: could not demote any masters")
			}
			log.Printf("Retrying with remaining masters: %v", remainingMasters)
			return s.resolveMultipleMasters(remainingMasters, currentReplicas)
		}
		log.Printf("Successfully demoted %s to replica of %s", demotedMaster, chosenMaster)
	}

	// Step 2: Update internal state
	s.stateMu.Lock()
	s.currentPrimary = chosenMaster

	// Build new replicas list
	var newReplicas []string
	newReplicas = append(newReplicas, demotedMasters...)  // Add all demoted masters
	newReplicas = append(newReplicas, currentReplicas...) // Add existing replicas

	s.currentReplicas = newReplicas
	s.stateMu.Unlock()

	// Step 3: Reconfigure all reachable nodes to point to the correct master
	go s.reconfigureAllNodes(chosenMaster)

	// Step 4: Synchronize with DB service
	go s.synchronizeDB()
	go s.elector.UpdatePrimary(chosenMaster)

	log.Printf("Split-brain resolved. New primary: %s, Replicas: %v", chosenMaster, newReplicas)
	return nil
}

// selectPreferredMasterFromList chooses which master to keep from a list based on consistent criteria
func (s *Supervisor) selectPreferredMasterFromList(masters []string) string {
	// Simple selection: choose the lexicographically highest address
	// This ensures consistent selection across all supervisors
	chosen := masters[0]
	for _, master := range masters[1:] {
		if master > chosen {
			chosen = master
		}
	}
	return chosen
}
