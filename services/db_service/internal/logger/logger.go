package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

var (
	// InfoLogger es usado para mensajes informativos
	InfoLogger *log.Logger
	// ErrorLogger es usado para mensajes de error
	ErrorLogger *log.Logger
)

// InitLogger inicializa los loggers para escribir tanto en consola como en archivo
func InitLogger(logDir, nodeID string) error {
	// Obtener la ruta absoluta del directorio de logs
	absLogDir, err := filepath.Abs(logDir)
	if err != nil {
		return fmt.Errorf("no se pudo obtener la ruta absoluta para el directorio de logs: %v", err)
	}

	// Crear el directorio de logs si no existe
	if err := os.MkdirAll(absLogDir, 0755); err != nil {
		return fmt.Errorf("no se pudo crear el directorio de logs: %v", err)
	}

	// Crear archivo de log
	logFile := filepath.Join(absLogDir, nodeID+".log")
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("no se pudo crear/abrir el archivo de log: %v", err)
	}

	// Configurar el logger para que escriba tanto en archivo como en consola
	multiWriter := io.MultiWriter(os.Stdout, file)

	// Inicializar loggers con prefijo que incluya el ID del nodo
	prefix := fmt.Sprintf("[%s] ", nodeID)
	InfoLogger = log.New(multiWriter, prefix+"INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	ErrorLogger = log.New(multiWriter, prefix+"ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	// Escribir un mensaje de inicio para confirmar que el logger está funcionando
	InfoLogger.Printf("Iniciando logger. Archivo de log: %s", logFile)

	return nil
}
