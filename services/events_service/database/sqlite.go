package database

import (
	"database/sql"
	"events-service/models"
	"fmt"
	"log"

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

	// Crear tabla de eventos (si no existe)
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

// ✅ MÉTODO EXISTENTE: Crear evento
func (d *Database) CreateEvent(event *models.Event) error {
	query := `INSERT INTO events (id, title, description, start_time, end_time, user_id, created_at, updated_at) 
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := d.DB.Exec(query, event.ID, event.Title, event.Description,
		event.StartTime, event.EndTime, event.UserID, event.CreatedAt, event.UpdatedAt)
	return err
}

// ✅ NUEVO MÉTODO: Obtener evento por ID
func (d *Database) GetEventByID(eventID string) (*models.Event, error) {
	query := `SELECT id, title, description, start_time, end_time, user_id, created_at, updated_at 
			  FROM events WHERE id = ?`

	row := d.DB.QueryRow(query, eventID)

	var event models.Event
	err := row.Scan(
		&event.ID,
		&event.Title,
		&event.Description,
		&event.StartTime,
		&event.EndTime,
		&event.UserID,
		&event.CreatedAt,
		&event.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Evento no encontrado
	}
	if err != nil {
		return nil, err
	}

	return &event, nil
}

// ✅ NUEVO MÉTODO: Eliminar evento
func (d *Database) DeleteEvent(eventID string) (bool, error) {
	query := `DELETE FROM events WHERE id = ?`

	result, err := d.DB.Exec(query, eventID)
	if err != nil {
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rowsAffected > 0, nil
}

// ✅ MÉTODO EXISTENTE: Obtener eventos con paginación
func (d *Database) GetEvents(userID string, limit, offset int) ([]models.Event, int, error) {
	var events []models.Event
	var total int

	// Construir query base
	baseQuery := "SELECT * FROM events"
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
		return nil, 0, err
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
		return nil, 0, err
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
			return nil, 0, err
		}
		events = append(events, event)
	}

	return events, total, nil
}

// ✅ MÉTODO EXISTENTE: Verificar conflictos con detalles
func (d *Database) CheckTimeConflictWithDetails(userID string, startTime, endTime interface{}) (bool, []models.Event, error) {
	var conflictingEvents []models.Event

	query := `SELECT id, title, description, start_time, end_time, user_id, created_at, updated_at 
              FROM events 
              WHERE user_id = ? AND 
              ((start_time < ? AND end_time > ?) OR 
               (start_time BETWEEN ? AND ?) OR 
               (end_time BETWEEN ? AND ?))`

	rows, err := d.DB.Query(query, userID,
		endTime, startTime,
		startTime, endTime,
		startTime, endTime)

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
