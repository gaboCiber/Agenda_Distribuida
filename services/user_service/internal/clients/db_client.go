package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

type DBServiceClient struct {
	baseURL string
	client  *http.Client
	logger  *zap.Logger
}

// AgendaEvent represents a calendar/agenda event
type AgendaEvent struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Location    string    `json:"location"`
	UserID      string    `json:"user_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func NewDBServiceClient(baseURL string, logger *zap.Logger) *DBServiceClient {
	// Asegurarse de que la URL base no termine con /
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &DBServiceClient{
		baseURL: baseURL,
		client:  &http.Client{},
		logger:  logger,
	}
}

// User representa un usuario en el sistema
type User struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// LoginRequest representa la solicitud de inicio de sesión
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse representa la respuesta de inicio de sesión
type LoginResponse struct {
	Status string `json:"status"`
	User   *User  `json:"user"`
}

// CreateUser crea un nuevo usuario en el sistema
func (c *DBServiceClient) CreateUser(ctx context.Context, email, password, username string) (*User, error) {
	url := fmt.Sprintf("%s/api/v1/users", c.baseURL)

	reqBody := map[string]interface{}{
		"email":    email,
		"password": password,
		"username": username,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		c.logger.Error("Error al serializar la solicitud", zap.Error(err))
		return nil, fmt.Errorf("error al serializar la solicitud: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPost, url, jsonBody)
	if err != nil {
		return nil, err
	}

	var response struct {
		Status string `json:"status"`
		User   *User  `json:"user"`
	}

	if err := json.Unmarshal(resp, &response); err != nil {
		c.logger.Error("Error al deserializar la respuesta", zap.Error(err))
		return nil, fmt.Errorf("error al deserializar la respuesta: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("error al crear el usuario: %s", string(resp))
	}

	return response.User, nil
}

// GetUser obtiene un usuario por su ID
func (c *DBServiceClient) GetUser(ctx context.Context, userID string) (*User, error) {
	url := fmt.Sprintf("%s/api/v1/users/%s", c.baseURL, userID)

	resp, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Status string `json:"status"`
		User   *User  `json:"user"`
	}

	if err := json.Unmarshal(resp, &response); err != nil {
		c.logger.Error("Error al deserializar la respuesta", zap.Error(err))
		return nil, fmt.Errorf("error al deserializar la respuesta: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("error al obtener el usuario: %s", string(resp))
	}

	return response.User, nil
}

// Login autentica a un usuario y devuelve sus datos
func (c *DBServiceClient) Login(ctx context.Context, email, password string) (*User, error) {
	url := fmt.Sprintf("%s/api/v1/users/login", c.baseURL)

	reqBody := LoginRequest{
		Email:    email,
		Password: password,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		c.logger.Error("Error al serializar la solicitud", zap.Error(err))
		return nil, fmt.Errorf("error al serializar la solicitud: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPost, url, jsonBody)
	if err != nil {
		return nil, err
	}

	// Primero intentamos parsear la respuesta con el formato esperado
	var response struct {
		Status string `json:"status"`
		UserID string `json:"user_id"`
	}

	if err := json.Unmarshal(resp, &response); err != nil {
		c.logger.Error("Error al deserializar la respuesta", zap.Error(err), zap.ByteString("response", resp))
		return nil, fmt.Errorf("error al deserializar la respuesta: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("error en el inicio de sesión: %s", string(resp))
	}

	// Si llegamos aquí, el login fue exitoso, obtenemos los datos del usuario
	user, err := c.GetUser(ctx, response.UserID)
	if err != nil {
		c.logger.Error("Error al obtener los datos del usuario después del login",
			zap.String("user_id", response.UserID),
			zap.Error(err))
		return nil, fmt.Errorf("error al obtener los datos del usuario: %w", err)
	}

	return user, nil
}

// CreateAgendaEvent crea un nuevo evento en la agenda
func (c *DBServiceClient) CreateAgendaEvent(ctx context.Context, event *AgendaEvent) (*AgendaEvent, error) {
	url := fmt.Sprintf("%s/api/v1/events", c.baseURL)

	eventData := map[string]interface{}{
		"title":       event.Title,
		"description": event.Description,
		"start_time":  event.StartTime.Format(time.RFC3339),
		"end_time":    event.EndTime.Format(time.RFC3339),
		"location":    event.Location,
		"user_id":     event.UserID,
	}

	jsonBody, err := json.Marshal(eventData)
	if err != nil {
		c.logger.Error("Error al serializar el evento", zap.Error(err))
		return nil, fmt.Errorf("error al serializar el evento: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPost, url, jsonBody)
	if err != nil {
		return nil, err
	}

	var response struct {
		Status string     `json:"status"`
		Event  *AgendaEvent `json:"event"`
	}

	if err := json.Unmarshal(resp, &response); err != nil {
		c.logger.Error("Error al deserializar la respuesta", zap.Error(err))
		return nil, fmt.Errorf("error al deserializar la respuesta: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("error al crear el evento: %s", string(resp))
	}

	return response.Event, nil
}

// GetAgendaEvent obtiene un evento por su ID
func (c *DBServiceClient) GetAgendaEvent(ctx context.Context, eventID string) (*AgendaEvent, error) {
	url := fmt.Sprintf("%s/api/v1/events/%s", c.baseURL, eventID)

	resp, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Status string     `json:"status"`
		Event  *AgendaEvent `json:"event"`
	}

	if err := json.Unmarshal(resp, &response); err != nil {
		c.logger.Error("Error al deserializar la respuesta", zap.Error(err))
		return nil, fmt.Errorf("error al deserializar la respuesta: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("error al obtener el evento: %s", string(resp))
	}

	return response.Event, nil
}

// UpdateAgendaEvent actualiza un evento existente
func (c *DBServiceClient) UpdateAgendaEvent(ctx context.Context, eventID string, updates map[string]interface{}) (*AgendaEvent, error) {
	url := fmt.Sprintf("%s/api/v1/events/%s", c.baseURL, eventID)

	jsonBody, err := json.Marshal(updates)
	if err != nil {
		c.logger.Error("Error al serializar la solicitud", zap.Error(err))
		return nil, fmt.Errorf("error al serializar la solicitud: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPut, url, jsonBody)
	if err != nil {
		return nil, err
	}

	var response struct {
		Status string     `json:"status"`
		Event  *AgendaEvent `json:"event"`
	}

	if err := json.Unmarshal(resp, &response); err != nil {
		c.logger.Error("Error al deserializar la respuesta", zap.Error(err))
		return nil, fmt.Errorf("error al deserializar la respuesta: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("error al actualizar el evento: %s", string(resp))
	}

	return response.Event, nil
}

// DeleteAgendaEvent elimina un evento por su ID
func (c *DBServiceClient) DeleteAgendaEvent(ctx context.Context, eventID string) error {
	url := fmt.Sprintf("%s/api/v1/events/%s", c.baseURL, eventID)

	_, err := c.doRequest(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	return nil
}

// ListAgendaEventsByUser obtiene todos los eventos de un usuario con paginación
func (c *DBServiceClient) ListAgendaEventsByUser(ctx context.Context, userID string, offset, limit int) ([]*AgendaEvent, error) {
	url := fmt.Sprintf("%s/api/v1/events/users/%s?offset=%d&limit=%d", c.baseURL, userID, offset, limit)

	resp, err := c.doRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Status string        `json:"status"`
		Events []*AgendaEvent `json:"events"`
		Count  int           `json:"count"`
	}

	if err := json.Unmarshal(resp, &response); err != nil {
		c.logger.Error("Error al deserializar la respuesta", zap.Error(err))
		return nil, fmt.Errorf("error al deserializar la respuesta: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("error al obtener los eventos: %s", string(resp))
	}

	return response.Events, nil
}

// DeleteUser elimina un usuario por su ID
func (c *DBServiceClient) DeleteUser(ctx context.Context, userID string) error {
	url := fmt.Sprintf("%s/api/v1/users/%s", c.baseURL, userID)

	_, err := c.doRequest(ctx, http.MethodDelete, url, nil)
	return err
}

// UpdateUser actualiza un usuario existente
func (c *DBServiceClient) UpdateUser(ctx context.Context, userID string, updates map[string]interface{}) (*User, error) {
	url := fmt.Sprintf("%s/api/v1/users/%s", c.baseURL, userID)

	jsonBody, err := json.Marshal(updates)
	if err != nil {
		c.logger.Error("Error al serializar la solicitud", zap.Error(err))
		return nil, fmt.Errorf("error al serializar la solicitud: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPut, url, jsonBody)
	if err != nil {
		return nil, err
	}

	var response struct {
		Status string `json:"status"`
		User   *User  `json:"user"`
	}

	if err := json.Unmarshal(resp, &response); err != nil {
		c.logger.Error("Error al deserializar la respuesta", zap.Error(err))
		return nil, fmt.Errorf("error al deserializar la respuesta: %w", err)
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("error al actualizar el usuario: %s", string(resp))
	}

	return response.User, nil
}

// doRequest es una función auxiliar para realizar peticiones HTTP
func (c *DBServiceClient) doRequest(ctx context.Context, method, url string, body []byte) ([]byte, error) {
	var req *http.Request
	var err error

	if body != nil {
		req, err = http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	}

	if err != nil {
		c.logger.Error("Error al crear la solicitud", zap.Error(err))
		return nil, fmt.Errorf("error al crear la solicitud: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Error("Error al enviar la solicitud", zap.Error(err))
		return nil, fmt.Errorf("error al enviar la solicitud: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Error("Error al leer la respuesta", zap.Error(err))
		return nil, fmt.Errorf("error al leer la respuesta: %w", err)
	}

	if resp.StatusCode >= 400 {
		c.logger.Error("Error en la respuesta del servidor",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(respBody)))
		return nil, fmt.Errorf("error en la respuesta del servidor (%d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
