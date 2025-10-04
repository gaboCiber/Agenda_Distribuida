package models

import (
    "time"
)

type Event struct {
    ID          string    `json:"id" db:"id"`
    Title       string    `json:"title" db:"title"`
    Description string    `json:"description" db:"description"`
    StartTime   time.Time `json:"start_time" db:"start_time"`
    EndTime     time.Time `json:"end_time" db:"end_time"`
    UserID      string    `json:"user_id" db:"user_id"`
    CreatedAt   time.Time `json:"created_at" db:"created_at"`
    UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type EventRequest struct {
    Title       string    `json:"title"`
    Description string    `json:"description"`
    StartTime   time.Time `json:"start_time"`
    EndTime     time.Time `json:"end_time"`
    UserID      string    `json:"user_id"`
}

type RedisEvent struct {
    EventID   string                 `json:"event_id"`
    Type      string                 `json:"type"`
    Timestamp string                 `json:"timestamp"`
    Version   string                 `json:"version"`
    Payload   map[string]interface{} `json:"payload"`
}