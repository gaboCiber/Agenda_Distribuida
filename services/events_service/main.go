package main

import (
	"encoding/json"
	"events-service/config"
	"events-service/database"
	"events-service/handlers"
	"events-service/services"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

func main() {
	// Cargar configuración
	cfg := config.Load()

	// Inicializar base de datos
	db, err := database.NewDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("❌ Error inicializando base de datos: %v", err)
	}
	defer db.Close()

	// Inicializar Redis
	redisService := services.NewRedisService(cfg.RedisHost, cfg.RedisPort, cfg.RedisDB)
	if redisService == nil {
		log.Fatalf("❌ No se pudo conectar a Redis")
	}

	// Inicializar handler de eventos
	eventsHandler := handlers.NewEventsHandler(redisService, db)

	// Iniciar listener de eventos en goroutine
	go func() {
		log.Println("👂 Iniciando listener de eventos...")
		redisService.SubscribeToChannel("events_events", eventsHandler.HandleEventCreation)
	}()

	// Configurar HTTP handlers
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/api/events", getEventsHandler(db))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Events Service is running",
			"status":  "active",
		})
	})

	// Iniciar servidor HTTP en goroutine
	go func() {
		log.Printf("🚀 Events Service iniciado en puerto %s", cfg.Port)
		if err := http.ListenAndServe(":"+cfg.Port, nil); err != nil {
			log.Fatalf("❌ Error iniciando servidor HTTP: %v", err)
		}
	}()

	// Esperar señal de terminación
	waitForShutdown()
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"service":   "events_service",
		"timestamp": map[string]interface{}{},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func getEventsHandler(db *database.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Obtener parámetros de query
		query := r.URL.Query()
		userID := query.Get("user_id")
		limitStr := query.Get("limit")
		offsetStr := query.Get("offset")

		// Convertir limit y offset con valores por defecto
		limit := 50
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
				limit = l
			}
		}

		offset := 0
		if offsetStr != "" {
			if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
				offset = o
			}
		}

		// Obtener eventos de la base de datos
		events, total, err := db.GetEvents(userID, limit, offset)
		if err != nil {
			log.Printf("❌ Error obteniendo eventos: %v", err)
			http.Error(w, `{"error": "Error obteniendo eventos"}`, http.StatusInternalServerError)
			return
		}

		// Crear respuesta
		response := map[string]interface{}{
			"events": events,
			"total":  total,
		}

		json.NewEncoder(w).Encode(response)
	}
}

func waitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("🛑 Events Service deteniéndose...")
}
