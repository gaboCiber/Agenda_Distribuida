package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/agenda-distribuida/group-service/internal/clients"
	"github.com/agenda-distribuida/group-service/internal/config"
	"github.com/agenda-distribuida/group-service/internal/handlers"
	"github.com/agenda-distribuida/group-service/internal/services"
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

	// Verificar conexión a Redis
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.Fatal("No se pudo conectar a Redis", zap.Error(err))
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

	// Esperar señales de terminación
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		logger.Info("Recibida señal de terminación",
			zap.String("señal", sig.String()))
		cancel()
	case err := <-errChan:
		logger.Error("Error en el manejador de eventos",
			zap.Error(err))
		cancel()
	}

	// Dar tiempo para que las operaciones en curso finalicen
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	// Esperar a que todas las goroutines finalicen
	<-shutdownCtx.Done()

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
