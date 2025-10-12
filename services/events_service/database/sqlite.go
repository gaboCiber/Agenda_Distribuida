package database

import (
	"database/sql"
	"events-service/models"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	DB *sql.DB
}

func NewDB(dbPath string) (*Database, error) {

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %v", err)
	}

	// Crear tabla de eventos
	createTableSQL := `
    CREATE TABLE IF NOT EXISTS events (
        id TEXT PRIMARY KEY,
        title TEXT NOT NULL,
        description TEXT,
        start_time DATETIME NOT NULL,
        end_time DATETIME NOT NULL,
        user_id TEXT NOT NULL,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );
    
    CREATE INDEX IF NOT EXISTS idx_events_user_id ON events(user_id);
    CREATE INDEX IF NOT EXISTS idx_events_time_range ON events(start_time, end_time);
    `

	if _, err := db.Exec(createTableSQL); err != nil {
		return nil, fmt.Errorf("error creating table: %v", err)
	}

	log.Println("✅ Base de datos de eventos inicializada correctamente")
	return &Database{DB: db}, nil
}

func (d *Database) CreateEvent(event *models.Event) error {
	query := `INSERT INTO events (id, title, description, start_time, end_time, user_id, created_at, updated_at) 
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := d.DB.Exec(query,
		event.ID,
		event.Title,
		event.Description,
		event.StartTime,
		event.EndTime,
		event.UserID,
		event.CreatedAt,
		event.UpdatedAt,
	)
	return err
}

func (d *Database) CheckTimeConflict(userID string, startTime, endTime time.Time) (bool, error) {
	query := `SELECT COUNT(*) FROM events 
              WHERE user_id = ? AND 
              ((start_time BETWEEN ? AND ?) OR 
               (end_time BETWEEN ? AND ?) OR
               (start_time <= ? AND end_time >= ?))`

	var count int
	err := d.DB.QueryRow(query, userID,
		startTime, endTime, // BETWEEN para start_time
		startTime, endTime, // BETWEEN para end_time
		startTime, endTime).Scan(&count) // Evento que contiene el rango completo

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (d *Database) GetEvents(userID string, limit, offset int) ([]models.Event, int, error) {
	var events []models.Event
	var total int

	// Construir query base
	baseQuery := "SELECT id, title, description, start_time, end_time, user_id, created_at, updated_at FROM events"
	countQuery := "SELECT COUNT(*) FROM events"

	// Agregar filtro por user_id si se especifica
	whereClause := ""
	if userID != "" {
		whereClause = " WHERE user_id = ?"
	}

	// Obtener total count
	countStmt := countQuery + whereClause
	var err error
	if userID != "" {
		err = d.DB.QueryRow(countStmt, userID).Scan(&total)
	} else {
		err = d.DB.QueryRow(countStmt).Scan(&total)
	}

	if err != nil {
		return nil, 0, fmt.Errorf("error counting events: %v", err)
	}

	// Obtener eventos con paginación
	query := baseQuery + whereClause + " ORDER BY start_time DESC LIMIT ? OFFSET ?"

	var rows *sql.Rows
	if userID != "" {
		rows, err = d.DB.Query(query, userID, limit, offset)
	} else {
		rows, err = d.DB.Query(query, limit, offset)
	}

	if err != nil {
		return nil, 0, fmt.Errorf("error querying events: %v", err)
	}
	defer rows.Close()

	// Escanear resultados
	for rows.Next() {
		var event models.Event
		err := rows.Scan(
			&event.ID,
			&event.Title,
			&event.Description,
			&event.StartTime,
			&event.EndTime,
			&event.UserID,
			&event.CreatedAt,
			&event.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("error scanning event: %v", err)
		}
		events = append(events, event)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating events: %v", err)
	}

	return events, total, nil
}

// ✅ FUNCIÓN CORREGIDA para verificar conflictos con detalles
func (d *Database) CheckTimeConflictWithDetails(userID string, startTime, endTime time.Time) (bool, []models.Event, error) {
	var conflictingEvents []models.Event

	query := `SELECT id, title, description, start_time, end_time, user_id, created_at, updated_at 
              FROM events 
              WHERE user_id = ? AND 
              ((start_time < ? AND end_time > ?) OR 
               (start_time BETWEEN ? AND ?) OR 
               (end_time BETWEEN ? AND ?))`

	rows, err := d.DB.Query(query, userID,
		endTime, startTime, // start_time < new_end AND end_time > new_start
		startTime, endTime, // start_time BETWEEN new_start AND new_end
		startTime, endTime) // end_time BETWEEN new_start AND new_end

	if err != nil {
		return false, nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var event models.Event
		err := rows.Scan(
			&event.ID,
			&event.Title,
			&event.Description,
			&event.StartTime,
			&event.EndTime,
			&event.UserID,
			&event.CreatedAt,
			&event.UpdatedAt,
		)
		if err != nil {
			return false, nil, err
		}
		conflictingEvents = append(conflictingEvents, event)
	}

	if err = rows.Err(); err != nil {
		return false, nil, err
	}

	return len(conflictingEvents) > 0, conflictingEvents, nil
}

func (d *Database) Close() error {
	return d.DB.Close()
}
