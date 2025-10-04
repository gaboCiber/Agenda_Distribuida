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
    
    _, err := d.DB.Exec(query, event.ID, event.Title, event.Description, 
        event.StartTime, event.EndTime, event.UserID, event.CreatedAt, event.UpdatedAt)
    return err
}

func (d *Database) CheckTimeConflict(userID string, startTime, endTime time.Time) (bool, error) {
    query := `SELECT COUNT(*) FROM events 
              WHERE user_id = ? AND 
              ((start_time BETWEEN ? AND ?) OR (end_time BETWEEN ? AND ?) OR 
               (start_time <= ? AND end_time >= ?))`
    
    var count int
    err := d.DB.QueryRow(query, userID, startTime, endTime, startTime, endTime, startTime, endTime).Scan(&count)
    if err != nil {
        return false, err
    }
    
    return count > 0, nil
}

func (d *Database) Close() error {
    return d.DB.Close()
}