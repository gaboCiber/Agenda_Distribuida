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
	currentReplica string
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
			log.Printf("Initial primary found: %s. Replica: %s.", s.currentPrimary, s.currentReplica)
			s.synchronizeDB()
			break
		}
		log.Printf("Failed to find initial primary: %v. Retrying in 5 seconds...", err)
		time.Sleep(5 * time.Second)
	}

	// Start the monitoring loops for both primary and replica
	go s.monitorPrimaryLoop()
	go s.monitorReplicaLoop()

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
		s.currentReplica = foundReplicas[0]
	} else {
		log.Println("Warning: No replica found.")
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

// monitorReplicaLoop periodically pings the current replica and resurrects it if it fails.
func (s *Supervisor) monitorReplicaLoop() {
	// A slightly longer interval for the replica is fine.
	ticker := time.NewTicker(s.config.PingInterval * 2)
	defer ticker.Stop()

	failureCount := 0
	const replicaFailureThreshold = 3

	for range ticker.C {
		if s.currentReplica == "" {
			continue // Do nothing if there's no replica to monitor
		}

		log.Printf("Pinging replica: %s", s.currentReplica)
		err := s.redisClient.Ping(s.currentReplica)
		if err != nil {
			failureCount++
			log.Printf("Ping failed for replica %s. Failure count: %d/%d. Error: %v", s.currentReplica, failureCount, replicaFailureThreshold, err)

			if failureCount >= replicaFailureThreshold {
				log.Printf("Replica %s has reached failure threshold. Attempting resurrection.", s.currentReplica)
				go s.resurrectNodeAsReplica(s.currentReplica, s.currentPrimary)
				failureCount = 0
			}
		} else {
			if failureCount > 0 {
				log.Printf("Successfully pinged replica %s after %d failures. Resetting.", s.currentReplica, failureCount)
				failureCount = 0
			}
		}
	}
}

// initiateFailover promotes the replica to be the new primary and attempts to resurrect the old primary.
func (s *Supervisor) initiateFailover() {
	if s.currentReplica == "" {
		log.Println("Cannot initiate failover: no replica is configured or available.")
		// We will keep trying to ping the old primary in the monitor loop
		return
	}

	log.Printf("Attempting to promote %s to primary...", s.currentReplica)

	// 1. Promote replica
	err := s.redisClient.PromoteToPrimary(s.currentReplica)
	if err != nil {
		log.Printf("CRITICAL: Failed to promote replica %s: %v", s.currentReplica, err)
		// If promotion fails, we do not proceed. The monitor loop will continue.
		return
	}
	log.Printf("Successfully promoted %s to be the new primary.", s.currentReplica)

	// 2. Update internal state *immediately*. From this point on, we ping the new primary.
	oldPrimaryAddr := s.currentPrimary
	s.currentPrimary = s.currentReplica
	s.currentReplica = oldPrimaryAddr
	log.Printf("Internal state updated. New primary: %s. Old primary %s is now replica.", s.currentPrimary, s.currentReplica)

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
