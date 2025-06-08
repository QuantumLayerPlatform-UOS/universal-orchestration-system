package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"orchestrator/internal/config"
)

// AgentClient handles communication with the Agent Manager service
type AgentClient struct {
	httpClient       *http.Client
	wsDialer         *websocket.Dialer
	config           *config.AgentManagerConfig
	logger           *zap.Logger
	tracer           trace.Tracer
	wsConnections    map[string]*AgentConnection
	wsConnectionsMux sync.RWMutex
}

// AgentConnection represents a WebSocket connection to an agent
type AgentConnection struct {
	conn         *websocket.Conn
	agentID      string
	projectID    string
	sendChan     chan []byte
	receiveChan  chan []byte
	closeChan    chan struct{}
	closeOnce    sync.Once
	pingTicker   *time.Ticker
	lastPongTime time.Time
	mu           sync.Mutex
}

// NewAgentClient creates a new Agent Manager client
func NewAgentClient(cfg *config.AgentManagerConfig, logger *zap.Logger) (*AgentClient, error) {
	httpClient := &http.Client{
		Timeout: time.Duration(cfg.HTTPTimeout) * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	wsDialer := &websocket.Dialer{
		HandshakeTimeout: time.Duration(cfg.WebSocketTimeout) * time.Second,
		ReadBufferSize:   cfg.BufferSize,
		WriteBufferSize:  cfg.BufferSize,
		EnableCompression: cfg.EnableCompression,
	}

	return &AgentClient{
		httpClient:    httpClient,
		wsDialer:      wsDialer,
		config:        cfg,
		logger:        logger,
		tracer:        otel.Tracer("agent-client"),
		wsConnections: make(map[string]*AgentConnection),
	}, nil
}

// Close closes all connections
func (c *AgentClient) Close() error {
	c.wsConnectionsMux.Lock()
	defer c.wsConnectionsMux.Unlock()

	for _, conn := range c.wsConnections {
		conn.Close()
	}

	return nil
}

// HTTP API Methods

// CreateAgent creates a new agent
func (c *AgentClient) CreateAgent(ctx context.Context, req *CreateAgentRequest) (*Agent, error) {
	ctx, span := c.tracer.Start(ctx, "CreateAgent",
		trace.WithAttributes(
			attribute.String("agent.type", req.Type),
			attribute.String("project.id", req.ProjectID),
		),
	)
	defer span.End()

	url := fmt.Sprintf("%s/api/v1/agents", c.config.BaseURL)
	var agent Agent
	_, err := c.doRequest(ctx, http.MethodPost, url, req, &agent)
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

// GetAgent retrieves agent details
func (c *AgentClient) GetAgent(ctx context.Context, agentID string) (*Agent, error) {
	ctx, span := c.tracer.Start(ctx, "GetAgent",
		trace.WithAttributes(
			attribute.String("agent.id", agentID),
		),
	)
	defer span.End()

	url := fmt.Sprintf("%s/api/v1/agents/%s", c.config.BaseURL, agentID)
	var agent Agent
	_, err := c.doRequest(ctx, http.MethodGet, url, nil, &agent)
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

// ListAgents lists agents with filters
func (c *AgentClient) ListAgents(ctx context.Context, filters *AgentFilters) (*AgentList, error) {
	ctx, span := c.tracer.Start(ctx, "ListAgents")
	defer span.End()

	url := fmt.Sprintf("%s/api/v1/agents", c.config.BaseURL)
	// Add query parameters based on filters
	if filters != nil {
		// Build query string
		params := buildQueryParams(filters)
		if params != "" {
			url += "?" + params
		}
	}

	var agentList AgentList
	_, err := c.doRequest(ctx, http.MethodGet, url, nil, &agentList)
	if err != nil {
		return nil, err
	}
	return &agentList, nil
}

// UpdateAgent updates agent configuration
func (c *AgentClient) UpdateAgent(ctx context.Context, agentID string, req *UpdateAgentRequest) (*Agent, error) {
	ctx, span := c.tracer.Start(ctx, "UpdateAgent",
		trace.WithAttributes(
			attribute.String("agent.id", agentID),
		),
	)
	defer span.End()

	url := fmt.Sprintf("%s/api/v1/agents/%s", c.config.BaseURL, agentID)
	var agent Agent
	_, err := c.doRequest(ctx, http.MethodPut, url, req, &agent)
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

// DeleteAgent deletes an agent
func (c *AgentClient) DeleteAgent(ctx context.Context, agentID string) error {
	ctx, span := c.tracer.Start(ctx, "DeleteAgent",
		trace.WithAttributes(
			attribute.String("agent.id", agentID),
		),
	)
	defer span.End()

	url := fmt.Sprintf("%s/api/v1/agents/%s", c.config.BaseURL, agentID)
	_, err := c.doRequest(ctx, http.MethodDelete, url, nil, nil)
	return err
}

// ExecuteTask executes a task on an agent
func (c *AgentClient) ExecuteTask(ctx context.Context, agentID string, req *ExecuteTaskRequest) (*TaskExecution, error) {
	ctx, span := c.tracer.Start(ctx, "ExecuteTask",
		trace.WithAttributes(
			attribute.String("agent.id", agentID),
			attribute.String("task.type", req.Type),
		),
	)
	defer span.End()

	url := fmt.Sprintf("%s/api/v1/agents/%s/execute", c.config.BaseURL, agentID)
	var taskExecution TaskExecution
	_, err := c.doRequest(ctx, http.MethodPost, url, req, &taskExecution)
	if err != nil {
		return nil, err
	}
	return &taskExecution, nil
}

// GetTaskStatus retrieves task execution status
func (c *AgentClient) GetTaskStatus(ctx context.Context, agentID, taskID string) (*TaskExecution, error) {
	ctx, span := c.tracer.Start(ctx, "GetTaskStatus",
		trace.WithAttributes(
			attribute.String("agent.id", agentID),
			attribute.String("task.id", taskID),
		),
	)
	defer span.End()

	url := fmt.Sprintf("%s/api/v1/agents/%s/tasks/%s", c.config.BaseURL, agentID, taskID)
	var taskExecution TaskExecution
	_, err := c.doRequest(ctx, http.MethodGet, url, nil, &taskExecution)
	if err != nil {
		return nil, err
	}
	return &taskExecution, nil
}

// CancelTask cancels a running task
func (c *AgentClient) CancelTask(ctx context.Context, agentID, taskID string) error {
	ctx, span := c.tracer.Start(ctx, "CancelTask",
		trace.WithAttributes(
			attribute.String("agent.id", agentID),
			attribute.String("task.id", taskID),
		),
	)
	defer span.End()

	url := fmt.Sprintf("%s/api/v1/agents/%s/tasks/%s/cancel", c.config.BaseURL, agentID, taskID)
	_, err := c.doRequest(ctx, http.MethodPost, url, nil, nil)
	return err
}

// WebSocket Methods

// ConnectToAgent establishes a WebSocket connection to an agent
func (c *AgentClient) ConnectToAgent(ctx context.Context, agentID, projectID string) (*AgentConnection, error) {
	ctx, span := c.tracer.Start(ctx, "ConnectToAgent",
		trace.WithAttributes(
			attribute.String("agent.id", agentID),
			attribute.String("project.id", projectID),
		),
	)
	defer span.End()

	// Check if connection already exists
	c.wsConnectionsMux.RLock()
	if conn, exists := c.wsConnections[agentID]; exists {
		c.wsConnectionsMux.RUnlock()
		return conn, nil
	}
	c.wsConnectionsMux.RUnlock()

	// Create new connection
	url := fmt.Sprintf("%s/api/v1/agents/%s/connect", c.config.WebSocketURL, agentID)
	
	header := http.Header{}
	header.Add("X-Project-ID", projectID)

	wsConn, _, err := c.wsDialer.DialContext(ctx, url, header)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to agent: %w", err)
	}

	conn := &AgentConnection{
		conn:         wsConn,
		agentID:      agentID,
		projectID:    projectID,
		sendChan:     make(chan []byte, 100),
		receiveChan:  make(chan []byte, 100),
		closeChan:    make(chan struct{}),
		pingTicker:   time.NewTicker(time.Duration(c.config.PingInterval) * time.Second),
		lastPongTime: time.Now(),
	}

	// Set pong handler
	wsConn.SetPongHandler(func(string) error {
		conn.mu.Lock()
		conn.lastPongTime = time.Now()
		conn.mu.Unlock()
		return nil
	})

	// Start goroutines for reading and writing
	go conn.readPump(c.logger)
	go conn.writePump(c.logger, c.config)

	// Store connection
	c.wsConnectionsMux.Lock()
	c.wsConnections[agentID] = conn
	c.wsConnectionsMux.Unlock()

	return conn, nil
}

// DisconnectFromAgent closes WebSocket connection to an agent
func (c *AgentClient) DisconnectFromAgent(agentID string) error {
	c.wsConnectionsMux.Lock()
	defer c.wsConnectionsMux.Unlock()

	if conn, exists := c.wsConnections[agentID]; exists {
		conn.Close()
		delete(c.wsConnections, agentID)
	}

	return nil
}

// SendMessage sends a message to an agent via WebSocket
func (c *AgentClient) SendMessage(agentID string, message interface{}) error {
	c.wsConnectionsMux.RLock()
	conn, exists := c.wsConnections[agentID]
	c.wsConnectionsMux.RUnlock()

	if !exists {
		return fmt.Errorf("no connection to agent %s", agentID)
	}

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	select {
	case conn.sendChan <- data:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("send timeout")
	}
}

// ReceiveMessage receives a message from an agent via WebSocket
func (c *AgentClient) ReceiveMessage(agentID string, timeout time.Duration) ([]byte, error) {
	c.wsConnectionsMux.RLock()
	conn, exists := c.wsConnections[agentID]
	c.wsConnectionsMux.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no connection to agent %s", agentID)
	}

	select {
	case data := <-conn.receiveChan:
		return data, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("receive timeout")
	}
}

// Helper methods

func (c *AgentClient) doRequest(ctx context.Context, method, url string, body interface{}, result interface{}) (interface{}, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Add tracing headers
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		req.Header.Set("X-Trace-ID", span.SpanContext().TraceID().String())
		req.Header.Set("X-Span-ID", span.SpanContext().SpanID().String())
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errorResp ErrorResponse
		if err := json.Unmarshal(respBody, &errorResp); err == nil {
			return nil, fmt.Errorf("API error: %s (code: %s)", errorResp.Message, errorResp.Code)
		}
		return nil, fmt.Errorf("API error: %s", string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}
		return result, nil
	}

	return nil, nil
}

// AgentConnection methods

func (conn *AgentConnection) Close() {
	conn.closeOnce.Do(func() {
		close(conn.closeChan)
		conn.pingTicker.Stop()
		conn.conn.Close()
	})
}

func (conn *AgentConnection) readPump(logger *zap.Logger) {
	defer conn.Close()

	for {
		_, message, err := conn.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Error("websocket read error", zap.Error(err))
			}
			return
		}

		select {
		case conn.receiveChan <- message:
		case <-conn.closeChan:
			return
		default:
			// Drop message if receiver is not ready
			logger.Warn("dropping message, receive channel full")
		}
	}
}

func (conn *AgentConnection) writePump(logger *zap.Logger, cfg *config.AgentManagerConfig) {
	defer conn.Close()

	for {
		select {
		case message := <-conn.sendChan:
			conn.conn.SetWriteDeadline(time.Now().Add(time.Duration(cfg.WebSocketTimeout) * time.Second))
			if err := conn.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				logger.Error("websocket write error", zap.Error(err))
				return
			}

		case <-conn.pingTicker.C:
			conn.conn.SetWriteDeadline(time.Now().Add(time.Duration(cfg.WebSocketTimeout) * time.Second))
			if err := conn.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				logger.Error("websocket ping error", zap.Error(err))
				return
			}

			// Check for pong timeout
			conn.mu.Lock()
			if time.Since(conn.lastPongTime) > time.Duration(cfg.PongTimeout)*time.Second {
				conn.mu.Unlock()
				logger.Error("websocket pong timeout")
				return
			}
			conn.mu.Unlock()

		case <-conn.closeChan:
			// Send close message
			conn.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}
	}
}

// Request and response types

type CreateAgentRequest struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	ProjectID   string                 `json:"project_id"`
	Config      map[string]interface{} `json:"config"`
	Capabilities []string              `json:"capabilities"`
	Tags        []string               `json:"tags"`
}

type UpdateAgentRequest struct {
	Name         string                 `json:"name,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty"`
	Capabilities []string               `json:"capabilities,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
	Status       string                 `json:"status,omitempty"`
}

type ExecuteTaskRequest struct {
	Type       string                 `json:"type"`
	Input      map[string]interface{} `json:"input"`
	Config     map[string]interface{} `json:"config"`
	Priority   string                 `json:"priority"`
	Timeout    int                    `json:"timeout"`
	MaxRetries int                    `json:"max_retries"`
}

type Agent struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Type         string                 `json:"type"`
	Status       string                 `json:"status"`
	ProjectID    string                 `json:"project_id"`
	Config       map[string]interface{} `json:"config"`
	Capabilities []string               `json:"capabilities"`
	Tags         []string               `json:"tags"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

type AgentList struct {
	Agents     []Agent `json:"agents"`
	TotalCount int64   `json:"total_count"`
	Page       int     `json:"page"`
	PageSize   int     `json:"page_size"`
}

type TaskExecution struct {
	ID          string                 `json:"id"`
	AgentID     string                 `json:"agent_id"`
	Type        string                 `json:"type"`
	Status      string                 `json:"status"`
	Input       map[string]interface{} `json:"input"`
	Output      map[string]interface{} `json:"output"`
	Error       string                 `json:"error"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt *time.Time             `json:"completed_at"`
	Duration    int64                  `json:"duration"`
}

type AgentFilters struct {
	ProjectID string
	Type      string
	Status    string
	Tags      []string
	Page      int
	PageSize  int
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

func buildQueryParams(filters *AgentFilters) string {
	// Implementation for building query parameters from filters
	// This is a simplified version
	params := ""
	if filters.ProjectID != "" {
		params += fmt.Sprintf("project_id=%s&", filters.ProjectID)
	}
	if filters.Type != "" {
		params += fmt.Sprintf("type=%s&", filters.Type)
	}
	if filters.Status != "" {
		params += fmt.Sprintf("status=%s&", filters.Status)
	}
	if filters.Page > 0 {
		params += fmt.Sprintf("page=%d&", filters.Page)
	}
	if filters.PageSize > 0 {
		params += fmt.Sprintf("page_size=%d&", filters.PageSize)
	}
	// Remove trailing &
	if len(params) > 0 {
		params = params[:len(params)-1]
	}
	return params
}