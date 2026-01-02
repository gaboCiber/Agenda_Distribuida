package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/agenda-distribuida/group-service/internal/clients"
	"github.com/agenda-distribuida/group-service/internal/config"
	handlers "github.com/agenda-distribuida/group-service/internal/handlers"
	services "github.com/agenda-distribuida/group-service/internal/services"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// Configuración
	cfg := config.Load()

	// Inicializar logger
	logger := initLogger(cfg.LogLevel)
	defer logger.Sync()

	// Configurar Redis
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		logger.Fatal("Error al analizar la URL de Redis", zap.Error(err))
	}

	redisClient := redis.NewClient(redisOpts)
	defer redisClient.Close()

	// Verificar conexión a Redis (no fatal si falla, el EventHandler se reconectará automáticamente)
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.Warn("No se pudo conectar a Redis al iniciar, el servicio continuará y se reconectará automáticamente", zap.Error(err))
	}

	// Cliente para el servicio de base de datos
	dbClient := clients.NewDBServiceClient(cfg.DBServiceURL, logger)

	// Servicio de eventos
	eventService := services.NewEventService(dbClient, logger)

	// Manejador de eventos
	eventHandler := handlers.NewEventHandler(
		redisClient,
		eventService,
		cfg.RedisChannel,
		cfg.RaftNodesURLs,
		cfg.RedisURL,
		logger,
	)

	// Contexto para manejar la señal de apagado
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Canal para errores
	errChan := make(chan error, 1)

	// Iniciar el manejador de eventos en una goroutine
	go func() {
		logger.Info("Iniciando manejador de eventos",
			zap.String("canal", cfg.RedisChannel))

		if err := eventHandler.Start(ctx); err != nil {
			errChan <- err
		}
	}()

	// Obtener la dirección IP del contenedor
	containerIP, err := clients.GetContainerIP()
	if err != nil {
		logger.Warn("No se pudo obtener la IP del contenedor, usando localhost",
			zap.Error(err))
		containerIP = "localhost"
	}
	// Configurar dirección del servidor HTTP

	addr := fmt.Sprintf("%s:%d", containerIP, 8008)

	// Configurar el servicio de registro
	serviceName := cfg.ServiceName
	serviceAddress := fmt.Sprintf("http://%s:8008", containerIP)

	// Crear el servicio de registro
	registryService := services.NewRegistryService(cfg.RaftNodesURLs, logger)

	// Registrar el servicio
	if err := registryService.RegisterService(serviceName, serviceAddress); err != nil {
		logger.Error("Error registrando el servicio",
			zap.String("service", serviceName),
			zap.String("address", serviceAddress),
			zap.Error(err))
	} else {
		logger.Info("Servicio registrado exitosamente",
			zap.String("service", serviceName),
			zap.String("address", serviceAddress))

		// Iniciar el envío de heartbeats
		go registryService.StartHeartbeats(serviceName, serviceAddress, 30*time.Second)
	}

	// Iniciar servidor HTTP para health checks
	httpHandler := handlers.NewHTTPHandler(logger)
	mux := http.NewServeMux()
	httpHandler.SetupRoutes(mux)

	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		logger.Info("Iniciando servidor HTTP en :8008")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Error al iniciar el servidor HTTP", zap.Error(err))
			errChan <- err
		}
	}()

	// Esperar señales de terminación
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		logger.Info("Recibida señal de terminación",
			zap.String("señal", sig.String()))
		cancel()
	case err := <-errChan:
		logger.Error("Error en el servicio",
			zap.Error(err))
		cancel()
	}

	// Apagar el servidor HTTP de forma ordenada
	httpShutdownCtx, httpShutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer httpShutdownCancel()

	if err := httpServer.Shutdown(httpShutdownCtx); err != nil {
		logger.Error("Error al apagar el servidor HTTP", zap.Error(err))
	}

	// Dar tiempo para que las operaciones en curso finalicen
	gracefulShutdownCtx, gracefulShutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer gracefulShutdownCancel()

	// Esperar a que todas las goroutines finalicen
	<-gracefulShutdownCtx.Done()

	logger.Info("Servicio detenido correctamente")
}

func initLogger(level string) *zap.Logger {
	// Configurar nivel de log
	var logLevel zapcore.Level
	switch level {
	case "debug":
		logLevel = zap.DebugLevel
	case "info":
		logLevel = zap.InfoLevel
	case "warn":
		logLevel = zap.WarnLevel
	case "error":
		logLevel = zap.ErrorLevel
	default:
		logLevel = zap.InfoLevel
	}

	// Configurar el logger
	config := zap.Config{
		Level:            zap.NewAtomicLevelAt(logLevel),
		Development:      false,
		Encoding:         "json",
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	// Personalizar el formato de tiempo
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := config.Build()
	if err != nil {
		log.Fatalf("No se pudo inicializar el logger: %v", err)
	}

	return logger
}
