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
	dockerClient   *clients.DockerClient // New field for Docker client
	currentPrimary string
	currentReplica string
	redisNodes     []string
}

// New creates a new Supervisor
func New(cfg *config.Config, redisClient *clients.RedisClient, dbClient *clients.DBClient, dockerClient *clients.DockerClient) *Supervisor {
	return &Supervisor{
		config:      cfg,
		redisClient: redisClient,
		dbClient:    dbClient,
		dockerClient: dockerClient,
		redisNodes:  cfg.RedisAddrs,
	}
}

// Run starts the main monitoring loop
func (s *Supervisor) Run() {
	log.Println("Supervisor is starting...")

	// Initial attempt to find the primary
	for {
		err := s.findInitialPrimary()
		if err == nil {
			log.Printf("Initial primary found: %s. Replica: %s.", s.currentPrimary, s.currentReplica)

			// Synchronize DB service with the initial primary
			log.Println("Synchronizing DB service with initial primary...")
			err = s.dbClient.SetRedisPrimary(s.currentPrimary)
			if err != nil {
				log.Printf("CRITICAL: Failed to synchronize DB service with initial primary: %v", err)
				// Decide if we should exit here or continue. For robustness, we'll continue but log as critical.
			} else {
				log.Println("DB service synchronized with initial primary.")
			}
			break
		}
		log.Printf("Failed to find initial primary: %v. Retrying in 5 seconds...", err)
		time.Sleep(5 * time.Second)
	}

	// Start the monitoring loop
	s.monitorLoop()
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
		// For simplicity, we assume a single replica in this setup
		s.currentReplica = foundReplicas[0]
	} else {
		log.Println("Warning: No replica found.")
	}

	return nil
}

// monitorLoop periodically pings the current primary and triggers a failover if it becomes unresponsive.
func (s *Supervisor) monitorLoop() {
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
				// Reset failure count after failover attempt to avoid immediate re-triggering
				failureCount = 0
			}
		} else {
			if failureCount > 0 {
				log.Printf("Successfully pinged primary %s after %d failures. Resetting failure count.", s.currentPrimary, failureCount)
				failureCount = 0 // Reset on successful ping
			} else {
				log.Println("Ping successful.")
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
	log.Printf("Internal state updated. New primary: %s. Old primary %s is now considered the replica.", s.currentPrimary, s.currentReplica)

	// 3. Update DB service with the new primary's address
	log.Printf("Updating DB service with new primary address: %s", s.currentPrimary)
	err = s.dbClient.SetRedisPrimary(s.currentPrimary)
	if err != nil {
		// This is still a critical state, but the supervisor will now correctly monitor the new primary.
		log.Printf("CRITICAL: Failed to update DB service with new primary address: %v. The supervisor will continue monitoring the new primary, but other services may not be aware of the change.", err)
		// However, we still want to attempt resurrection even if DB update fails
	}
	log.Println("Successfully updated DB service.")

	log.Println("Failover complete.")

	// 4. Attempt to resurrect the old primary
	go s.resurrectOldPrimary(oldPrimaryAddr, s.currentPrimary) // Run in a goroutine to not block the main loop
}

// resurrectOldPrimary attempts to restart the given Redis node and configure it as a replica.
func (s *Supervisor) resurrectOldPrimary(oldPrimaryAddr, newPrimaryAddr string) {
	log.Printf("Attempting to resurrect old primary %s and configure it as replica of %s...", oldPrimaryAddr, newPrimaryAddr)

	containerName := strings.Split(oldPrimaryAddr, ":")[0] // Assuming container name is the host part of the address

	// 1. Restart the container
	log.Printf("Restarting Docker container %s...", containerName)
	err := s.dockerClient.RestartContainer(containerName)
	if err != nil {
		log.Printf("ERROR: Failed to restart container %s: %v", containerName, err)
		return
	}
	log.Printf("Container %s restarted. Waiting for Redis to become ready...", containerName)

	time.Sleep(5 * time.Second) // Give Redis some time to start up

	// 2. Configure as replica of the new primary
	log.Printf("Configuring %s as replica of %s...", oldPrimaryAddr, newPrimaryAddr)
	err = s.redisClient.SetAsReplicaOf(oldPrimaryAddr, newPrimaryAddr)
	if err != nil {
		log.Printf("ERROR: Failed to configure %s as replica of %s: %v", oldPrimaryAddr, newPrimaryAddr, err)
		return
	}
	log.Printf("Successfully resurrected %s and configured it as replica of %s.", oldPrimaryAddr, newPrimaryAddr)
}