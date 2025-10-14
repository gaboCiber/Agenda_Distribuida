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

		messageHandler := func(message string) {
			var redisEvent map[string]interface{}
			if err := json.Unmarshal([]byte(message), &redisEvent); err != nil {
				log.Printf("❌ Error parsing event: %v", err)
				return
			}

			eventType, ok := redisEvent["type"].(string)
			if !ok {
				log.Printf("❌ Event type not found or invalid")
				return
			}

			// Convertir a JSON string para los handlers específicos
			eventData, _ := json.Marshal(redisEvent)

			switch eventType {
			case "event_creation_requested":
				log.Printf("📥 Procesando evento de CREACIÓN: %s", redisEvent["event_id"])
				eventsHandler.HandleEventCreation(string(eventData))
			case "event_deletion_requested":
				log.Printf("📥 Procesando evento de ELIMINACIÓN: %s", redisEvent["event_id"])
				eventsHandler.HandleEventDeletion(string(eventData)) // ✅ CORREGIDO
			default:
				log.Printf("⚠️ Tipo de evento desconocido: %s", eventType)
			}
		}

		redisService.SubscribeToChannel("events_events", messageHandler)
	}()

	// Configurar HTTP handlers
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/api/events", getEventsHandler(db))
	http.HandleFunc("/api/events/", deleteEventHandler(db)) // Nuevo endpoint para DELETE
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
		// Solo permitir GET
		if r.Method != http.MethodGet {
			http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

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

// Nuevo handler para eliminar eventos via HTTP
func deleteEventHandler(db *database.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Solo permitir DELETE
		if r.Method != http.MethodDelete {
			http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		// Extraer event ID de la URL
		eventID := r.URL.Path[len("/api/events/"):]
		if eventID == "" {
			http.Error(w, `{"error": "Event ID is required"}`, http.StatusBadRequest)
			return
		}

		// Obtener user_id de query parameters
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			http.Error(w, `{"error": "User ID is required"}`, http.StatusBadRequest)
			return
		}

		// Verificar que el evento existe y pertenece al usuario
		event, err := db.GetEventByID(eventID)
		if err != nil {
			log.Printf("❌ Error obteniendo evento: %v", err)
			http.Error(w, `{"error": "Error verificando evento"}`, http.StatusInternalServerError)
			return
		}

		if event == nil {
			http.Error(w, `{"error": "Event not found"}`, http.StatusNotFound)
			return
		}

		if event.UserID != userID {
			http.Error(w, `{"error": "Unauthorized - Event does not belong to user"}`, http.StatusForbidden)
			return
		}

		// Eliminar evento
		deleted, err := db.DeleteEvent(eventID)
		if err != nil {
			log.Printf("❌ Error eliminando evento: %v", err)
			http.Error(w, `{"error": "Error deleting event"}`, http.StatusInternalServerError)
			return
		}

		if !deleted {
			http.Error(w, `{"error": "Event could not be deleted"}`, http.StatusInternalServerError)
			return
		}

		// Respuesta exitosa
		response := map[string]interface{}{
			"status":   "success",
			"message":  "Event deleted successfully",
			"event_id": eventID,
		}

		json.NewEncoder(w).Encode(response)
		log.Printf("✅ Evento eliminado exitosamente: %s para usuario %s", eventID, userID)
	}
}

func waitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("🛑 Events Service deteniéndose...")
}
