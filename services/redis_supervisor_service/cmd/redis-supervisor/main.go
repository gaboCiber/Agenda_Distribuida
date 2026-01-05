package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"redis_supervisor_service/internal/clients"
	"redis_supervisor_service/internal/config"
	"redis_supervisor_service/internal/election"
	httpHandler "redis_supervisor_service/internal/http"
	"redis_supervisor_service/internal/supervisor"
)

func main() {
	// Create a cancellable context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling to cancel the context on interrupt
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		<-sigs
		log.Println("Shutdown signal received, cancelling context.")
		cancel()
	}()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Println("Starting Redis Supervisor Service...")
	log.Printf("Monitoring Redis nodes: %v", cfg.RedisAddrs)
	log.Printf("DB Service URL: %s", cfg.DBServiceURL)
	log.Printf("Raft Nodes URLs: %v", cfg.RaftNodesURLs)
	log.Printf("Supervisor ID: %s", cfg.SupervisorID)
	log.Printf("Supervisor Bind Address: %s", cfg.SupervisorBindAddr)
	log.Printf("Supervisor Peers: %v", cfg.SupervisorPeers)

	// Get the service URL (host:port)
	_, port, err := net.SplitHostPort(cfg.SupervisorBindAddr)
	if err != nil {
		log.Fatalf("Invalid supervisor bind address: %v", err)
	}
	host, err := GetContainerIP()
	if err != nil {
		log.Printf("Failed to get container IP")
		os.Exit(1)
	}
	log.Printf("Container IP: %s", host)

	serviceURL := fmt.Sprintf("http://%s:%s", host, port)

	// Initialize clients
	redisClient := clients.NewRedisClient()
	dbClient := clients.NewDBClient(
		cfg.DBServiceURL,
		cfg.RaftNodesURLs,
		"redis-"+cfg.SupervisorID,
		serviceURL,
	)

	// Register the service with the registry
	// metadata := fmt.Sprintf(`{"supervisor_id":"%s","peers":%v}`, cfg.SupervisorID, cfg.SupervisorPeers)
	// if err := dbClient.RegisterService(metadata); err != nil {
	// 	log.Printf("Warning: Failed to register service: %v", err)
	// } else {
	// 	log.Printf("Successfully registered service with URL: %s", serviceURL)
	// 	defer func() {
	// 		if err := dbClient.DeregisterService(); err != nil {
	// 			log.Printf("Failed to deregister service: %v", err)
	// 		} else {
	// 			log.Println("Successfully deregistered service")
	// 		}
	// 	}()
	// }

	// Log discovered services
	// if services, err := dbClient.ListServices(); err != nil {
	// 	log.Printf("Failed to list services: %v", err)
	// } else {
	// 	log.Printf("Discovered %d services in registry", len(services))
	// 	for _, svc := range services {
	// 		log.Printf("- %s at %s", svc.ServiceName, svc.Address)
	// 	}
	// }

	// Initialize and start the leader elector
	peersMap := make(map[string]string)
	for _, p := range cfg.SupervisorPeers {
		peersMap[p.ID] = p.Address
	}
	elector, err := election.NewElector(cfg.SupervisorID, cfg.SupervisorBindAddr, peersMap, log.Default())
	if err != nil {
		log.Fatalf("Failed to create elector: %v", err)
	}

	if err := elector.Start(ctx); err != nil {
		log.Fatalf("Failed to start elector: %v", err)
	}
	defer elector.Stop()

	// Create and run the supervisor
	sup := supervisor.New(cfg, redisClient, dbClient, elector)
	go sup.Run(ctx)

	// Setup HTTP server
	mux := httpHandler.SetupRoutes(elector)
	httpPort := cfg.HTTPPort
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", httpPort),
		Handler: mux,
	}

	// Start HTTP server
	go func() {
		log.Printf("HTTP server starting on port %d", httpPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Wait for context to be cancelled (e.g., by SIGINT)
	<-ctx.Done()

	// Gracefully shutdown HTTP server
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Redis Supervisor Service shutting down gracefully.")
}

// getOutboundIP gets the preferred outbound IP address of this machine
func GetContainerIP() (string, error) {
	// Primero intentamos con el nombre del host (funciona en Docker)
	hostname, err := os.Hostname()
	if err == nil && hostname != "" {
		// Intentar resolver el hostname a una IP
		addrs, err := net.LookupIP(hostname)
		if err == nil {
			for _, addr := range addrs {
				if ipv4 := addr.To4(); ipv4 != nil {
					return ipv4.String(), nil
				}
			}
		}
	}

	// Si lo anterior falla, intentamos con las interfaces de red
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("error obteniendo interfaces de red: %v", err)
	}

	for _, iface := range ifaces {
		// Ignorar interfaces apagadas o loopback
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Verificar que sea una dirección IPv4 válida
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}

			// Verificar que no sea una dirección link-local
			if !ip.IsLinkLocalUnicast() {
				return ip.String(), nil
			}
		}
	}

	// Último recurso: usar la variable de entorno HOSTNAME (común en Docker)
	if hostIP := os.Getenv("HOSTNAME"); hostIP != "" {
		return hostIP, nil
	}

	return "", fmt.Errorf("no se pudo determinar la IP del contenedor")
}
