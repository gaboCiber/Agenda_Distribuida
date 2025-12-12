package supervisor

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"redis_supervisor_service/internal/clients"
	"redis_supervisor_service/internal/config"
)

// Supervisor contains the core logic for monitoring and failover.
type Supervisor struct {
	config         *config.Config
	redisClient    *clients.RedisClient
	dbClient       *clients.DBClient
	dockerClient   *clients.DockerClient
	currentPrimary string
	currentReplicas []string
	redisNodes     []string
}

// New creates a new Supervisor
func New(cfg *config.Config, redisClient *clients.RedisClient, dbClient *clients.DBClient, dockerClient *clients.DockerClient) *Supervisor {
	return &Supervisor{
		config:       cfg,
		redisClient:  redisClient,
		dbClient:     dbClient,
		dockerClient: dockerClient,
		redisNodes:   cfg.RedisAddrs,
	}
}

// Run starts the main monitoring loops.
func (s *Supervisor) Run() {
	log.Println("Supervisor is starting...")

	// Initial attempt to find the primary
	for {
		err := s.findInitialPrimary()
		if err == nil {
			log.Printf("Initial primary found: %s. Replicas: %v.", s.currentPrimary, s.currentReplicas)
			s.synchronizeDB()
			break
		}
		log.Printf("Failed to find initial primary: %v. Retrying in 5 seconds...", err)
		time.Sleep(5 * time.Second)
	}

	// Start the monitoring loops for primary and replicas
	go s.monitorPrimaryLoop()
	go s.monitorReplicasLoop()
	go s.clusterHealthCheckLoop()

	// Block forever. The main function will handle graceful shutdown.
	select {}
}

func (s *Supervisor) synchronizeDB() {
	log.Println("Synchronizing DB service with initial primary...")
	err := s.dbClient.SetRedisPrimary(s.currentPrimary)
	if err != nil {
		log.Printf("CRITICAL: Failed to synchronize DB service with initial primary: %v", err)
	} else {
		log.Println("DB service synchronized with initial primary.")
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
		return errors.New("no primary found")
	}

	s.currentPrimary = foundPrimary
	if len(foundReplicas) > 0 {
		s.currentReplicas = foundReplicas
	} else {
		log.Println("Warning: No replicas found.")
		s.currentReplicas = []string{}
	}

	return nil
}

// monitorPrimaryLoop periodically pings the current primary and triggers a failover if it becomes unresponsive.
func (s *Supervisor) monitorPrimaryLoop() {
	ticker := time.NewTicker(s.config.PingInterval)
	defer ticker.Stop()

	failureCount := 0

	for range ticker.C {
		log.Printf("Pinging primary: %s", s.currentPrimary)
		err := s.redisClient.Ping(s.currentPrimary)
		if err != nil {
			failureCount++
			log.Printf("Ping failed for primary %s. Failure count: %d/%d. Error: %v", s.currentPrimary, failureCount, s.config.FailureThreshold, err)

			if failureCount >= s.config.FailureThreshold {
				log.Printf("Primary %s has reached failure threshold of %d. Initiating failover.", s.currentPrimary, s.config.FailureThreshold)
				s.initiateFailover()
				failureCount = 0
			}
		} else {
			if failureCount > 0 {
				log.Printf("Successfully pinged primary %s after %d failures. Resetting failure count.", s.currentPrimary, failureCount)
				failureCount = 0
			} else {
				log.Println("Ping successful.")
			}
		}
	}
}

// monitorReplicasLoop periodically pings all current replicas and resurrects them if they fail.
func (s *Supervisor) monitorReplicasLoop() {
	// A slightly longer interval for the replicas is fine.
	ticker := time.NewTicker(s.config.PingInterval * 2)
	defer ticker.Stop()

	failureCounts := make(map[string]int)
	const replicaFailureThreshold = 3

	for range ticker.C {
		for _, replica := range s.currentReplicas {
			log.Printf("Pinging replica: %s", replica)
			err := s.redisClient.Ping(replica)
			if err != nil {
				failureCounts[replica]++
				log.Printf("Ping failed for replica %s. Failure count: %d/%d. Error: %v", replica, failureCounts[replica], replicaFailureThreshold, err)

				if failureCounts[replica] >= replicaFailureThreshold {
					log.Printf("Replica %s has reached failure threshold. Attempting resurrection.", replica)
					go s.resurrectNodeAsReplica(replica, s.currentPrimary)
					failureCounts[replica] = 0
				}
			} else {
				if failureCounts[replica] > 0 {
					log.Printf("Successfully pinged replica %s after %d failures. Resetting.", replica, failureCounts[replica])
					failureCounts[replica] = 0
				}
			}
		}
	}
}

// initiateFailover promotes the best available replica to be the new primary and attempts to resurrect the old primary.
func (s *Supervisor) initiateFailover() {
	if len(s.currentReplicas) == 0 {
		log.Println("Cannot initiate failover: no replicas are configured or available.")
		// We will keep trying to ping the old primary in the monitor loop
		return
	}

	// Find the best replica to promote (first healthy one)
	var chosenReplica string
	for _, replica := range s.currentReplicas {
		err := s.redisClient.Ping(replica)
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

	// 1. Promote chosen replica
	err := s.redisClient.PromoteToPrimary(chosenReplica)
	if err != nil {
		log.Printf("CRITICAL: Failed to promote replica %s: %v", chosenReplica, err)
		// If promotion fails, we do not proceed. The monitor loop will continue.
		return
	}
	log.Printf("Successfully promoted %s to be the new primary.", chosenReplica)

	// 2. Update internal state *immediately*. From this point on, we ping the new primary.
	oldPrimaryAddr := s.currentPrimary
	s.currentPrimary = chosenReplica
	
	// Update replicas list: remove the promoted replica and add the old primary
	newReplicas := []string{}
	for _, replica := range s.currentReplicas {
		if replica != chosenReplica {
			newReplicas = append(newReplicas, replica)
		}
	}
	newReplicas = append(newReplicas, oldPrimaryAddr)
	s.currentReplicas = newReplicas
	
	log.Printf("Internal state updated. New primary: %s. New replicas: %v.", s.currentPrimary, s.currentReplicas)

	// 3. Update DB service with the new primary's address
	log.Printf("Updating DB service with new primary address: %s", s.currentPrimary)
	err = s.dbClient.SetRedisPrimary(s.currentPrimary)
	if err != nil {
		log.Printf("CRITICAL: Failed to update DB about new primary: %v.", err)
	} else {
		log.Println("Successfully updated DB service.")
	}

	log.Println("Failover complete.")

	// 4. Attempt to resurrect the old primary as a replica of the new primary
	go s.resurrectNodeAsReplica(oldPrimaryAddr, s.currentPrimary)
}

// resurrectNodeAsReplica attempts to restart a given Redis node and configure it as a replica of the current primary.
func (s *Supervisor) resurrectNodeAsReplica(nodeToResurrectAddr, currentPrimaryAddr string) {
	log.Printf("Attempting to resurrect %s as replica of %s...", nodeToResurrectAddr, currentPrimaryAddr)

	containerName := strings.Split(nodeToResurrectAddr, ":")[0]

	log.Printf("Restarting Docker container %s...", containerName)
	err := s.dockerClient.RestartContainer(containerName)
	if err != nil {
		log.Printf("ERROR: Failed to restart container %s: %v", containerName, err)
		return
	}
	log.Printf("Container %s restarted. Waiting for Redis to become ready...", containerName)

	time.Sleep(5 * time.Second)

	log.Printf("Configuring %s as replica of %s...", nodeToResurrectAddr, currentPrimaryAddr)
	err = s.redisClient.SetAsReplicaOf(nodeToResurrectAddr, currentPrimaryAddr)
	if err != nil {
		log.Printf("ERROR: Failed to configure %s as replica: %v", nodeToResurrectAddr, err)
		return
	}
	log.Printf("Successfully resurrected and reconfigured %s as a replica.", nodeToResurrectAddr)
}

// clusterHealthCheckLoop periodically checks if the cluster has a primary and promotes one if needed
func (s *Supervisor) clusterHealthCheckLoop() {
	ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
	defer ticker.Stop()

	for range ticker.C {
		// Check if current primary is still healthy
		if s.currentPrimary != "" {
			err := s.redisClient.Ping(s.currentPrimary)
			if err == nil {
				// Primary is healthy, check if it's actually still the primary
				role, err := s.redisClient.GetRole(s.currentPrimary)
				if err == nil && role == "master" {
					continue // Everything is fine
				}
			}
		}

		// Current primary is not healthy or not primary, find a new one
		log.Println("Cluster health check: No healthy primary found, attempting recovery...")
		s.attemptClusterRecovery()
	}
}

// attemptClusterRecovery tries to find and promote a new primary from available replicas
func (s *Supervisor) attemptClusterRecovery() {
	// Find all healthy replicas
	var healthyReplicas []string
	for _, replica := range s.currentReplicas {
		err := s.redisClient.Ping(replica)
		if err == nil {
			role, err := s.redisClient.GetRole(replica)
			if err == nil && role == "slave" {
				healthyReplicas = append(healthyReplicas, replica)
			}
		}
	}

	if len(healthyReplicas) == 0 {
		log.Println("No healthy replicas found for cluster recovery")
		return
	}

	// Try to promote the first healthy replica
	chosenReplica := healthyReplicas[0]
	log.Printf("Attempting to promote %s to primary for cluster recovery...", chosenReplica)

	err := s.redisClient.PromoteToPrimary(chosenReplica)
	if err != nil {
		log.Printf("Failed to promote replica %s during cluster recovery: %v", chosenReplica, err)
		return
	}

	log.Printf("Successfully promoted %s to primary during cluster recovery", chosenReplica)

	// Update internal state
	oldPrimary := s.currentPrimary
	s.currentPrimary = chosenReplica
	
	// Update replicas list
	newReplicas := []string{}
	for _, replica := range s.currentReplicas {
		if replica != chosenReplica {
			newReplicas = append(newReplicas, replica)
		}
	}
	if oldPrimary != "" {
		newReplicas = append(newReplicas, oldPrimary)
	}
	s.currentReplicas = newReplicas

	log.Printf("Cluster recovery completed. New primary: %s, Replicas: %v", s.currentPrimary, s.currentReplicas)

	// Update DB service
	err = s.dbClient.SetRedisPrimary(s.currentPrimary)
	if err != nil {
		log.Printf("CRITICAL: Failed to update DB about new primary during recovery: %v", err)
	}

	// Try to resurrect other nodes as replicas
	for _, replica := range s.currentReplicas {
		if replica != oldPrimary {
			go s.resurrectNodeAsReplica(replica, s.currentPrimary)
		}
	}
}
