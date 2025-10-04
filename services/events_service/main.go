package main

import (
    "events-service/config"
    "events-service/database"
    "events-service/handlers"
    "events-service/services"
    "encoding/json"
    "log"
    "net/http"
    "os"
    "os/signal"
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
    http.HandleFunc("/", rootHandler)

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
        "status":  "healthy",
        "service": "events_service",
        "timestamp": map[string]interface{}{},
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
    response := map[string]string{
        "message": "Events Service is running",
        "status":  "active",
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func waitForShutdown() {
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan
    log.Println("🛑 Events Service deteniéndose...")
}