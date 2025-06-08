package services

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"

	"orchestrator/internal/config"
	pb "orchestrator/internal/proto/intent"
)

// IntentClient handles communication with the Intent Processor service
type IntentClient struct {
	conn     *grpc.ClientConn
	client   pb.IntentServiceClient
	logger   *zap.Logger
	config   *config.IntentAPIConfig
	tracer   trace.Tracer
}

// NewIntentClient creates a new Intent API client
func NewIntentClient(cfg *config.IntentAPIConfig, logger *zap.Logger) (*IntentClient, error) {
	// Set up connection options
	opts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(10 * 1024 * 1024), // 10MB
			grpc.MaxCallSendMsgSize(10 * 1024 * 1024), // 10MB
		),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                time.Duration(cfg.KeepAliveInterval) * time.Second,
			Timeout:             time.Duration(cfg.KeepAliveTimeout) * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff: backoff.Config{
				BaseDelay:  1.0 * time.Second,
				Multiplier: 1.5,
				Jitter:     0.2,
				MaxDelay:   30 * time.Second,
			},
			MinConnectTimeout: time.Duration(cfg.Timeout) * time.Second,
		}),
	}

	// Configure TLS if enabled
	if cfg.EnableTLS {
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		
		if cfg.TLSCACertFile != "" {
			creds, err := credentials.NewClientTLSFromFile(cfg.TLSCACertFile, "")
			if err != nil {
				return nil, fmt.Errorf("failed to load TLS credentials: %w", err)
			}
			opts = append(opts, grpc.WithTransportCredentials(creds))
		} else {
			opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
		}
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Establish connection
	conn, err := grpc.Dial(cfg.Address, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to intent service: %w", err)
	}

	return &IntentClient{
		conn:   conn,
		client: pb.NewIntentServiceClient(conn),
		logger: logger,
		config: cfg,
		tracer: otel.Tracer("intent-client"),
	}, nil
}

// Close closes the gRPC connection
func (c *IntentClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ProcessIntent sends an intent to the Intent Processor service
func (c *IntentClient) ProcessIntent(ctx context.Context, req *ProcessIntentRequest) (*ProcessIntentResponse, error) {
	ctx, span := c.tracer.Start(ctx, "ProcessIntent",
		trace.WithAttributes(
			attribute.String("intent.type", req.Type),
			attribute.String("project.id", req.ProjectID),
		),
	)
	defer span.End()

	// Set timeout
	ctx, cancel := context.WithTimeout(ctx, time.Duration(c.config.Timeout)*time.Second)
	defer cancel()

	// Add metadata
	ctx = metadata.AppendToOutgoingContext(ctx,
		"x-project-id", req.ProjectID,
		"x-user-id", req.UserID,
		"x-request-id", req.RequestID,
	)

	// Create gRPC request
	grpcReq := &pb.ProcessIntentRequest{
		Intent: &pb.Intent{
			Type:        req.Type,
			Content:     req.Content,
			Context:     req.Context,
			Parameters:  req.Parameters,
			Constraints: req.Constraints,
		},
		ProjectId: req.ProjectID,
		UserId:    req.UserID,
		RequestId: req.RequestID,
		Options: &pb.ProcessingOptions{
			Async:          req.Async,
			Priority:       req.Priority,
			TimeoutSeconds: int32(req.TimeoutSeconds),
			MaxRetries:     int32(req.MaxRetries),
		},
	}

	// Make the call with retry
	var resp *pb.ProcessIntentResponse
	var err error
	
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		resp, err = c.client.ProcessIntent(ctx, grpcReq)
		if err == nil {
			break
		}

		if attempt < c.config.MaxRetries {
			c.logger.Warn("intent processing failed, retrying",
				zap.Error(err),
				zap.Int("attempt", attempt+1),
				zap.Int("max_retries", c.config.MaxRetries),
			)
			time.Sleep(time.Duration(c.config.RetryInterval) * time.Second)
			continue
		}
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to process intent after %d attempts: %w", c.config.MaxRetries, err)
	}

	span.SetStatus(codes.Ok, "Intent processed successfully")

	// Convert response
	result := &ProcessIntentResponse{
		IntentID:    resp.IntentId,
		Status:      resp.Status,
		Message:     resp.Message,
		Result:      resp.Result,
		Confidence:  resp.Confidence,
		Actions:     convertActions(resp.Actions),
		Suggestions: resp.Suggestions,
		Metadata:    resp.Metadata,
	}

	return result, nil
}

// GetIntentStatus retrieves the status of an intent
func (c *IntentClient) GetIntentStatus(ctx context.Context, intentID string) (*IntentStatus, error) {
	ctx, span := c.tracer.Start(ctx, "GetIntentStatus",
		trace.WithAttributes(
			attribute.String("intent.id", intentID),
		),
	)
	defer span.End()

	ctx, cancel := context.WithTimeout(ctx, time.Duration(c.config.Timeout)*time.Second)
	defer cancel()

	req := &pb.GetIntentStatusRequest{
		IntentId: intentID,
	}

	resp, err := c.client.GetIntentStatus(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to get intent status: %w", err)
	}

	return &IntentStatus{
		IntentID:    resp.IntentId,
		Status:      resp.Status,
		Progress:    int(resp.Progress),
		Message:     resp.Message,
		StartedAt:   resp.StartedAt.AsTime(),
		CompletedAt: resp.CompletedAt.AsTime(),
		Error:       resp.Error,
	}, nil
}

// CancelIntent cancels a running intent
func (c *IntentClient) CancelIntent(ctx context.Context, intentID string, reason string) error {
	ctx, span := c.tracer.Start(ctx, "CancelIntent",
		trace.WithAttributes(
			attribute.String("intent.id", intentID),
			attribute.String("reason", reason),
		),
	)
	defer span.End()

	ctx, cancel := context.WithTimeout(ctx, time.Duration(c.config.Timeout)*time.Second)
	defer cancel()

	req := &pb.CancelIntentRequest{
		IntentId: intentID,
		Reason:   reason,
	}

	_, err := c.client.CancelIntent(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to cancel intent: %w", err)
	}

	return nil
}

// AnalyzeIntent analyzes an intent without processing it
func (c *IntentClient) AnalyzeIntent(ctx context.Context, req *AnalyzeIntentRequest) (*AnalyzeIntentResponse, error) {
	ctx, span := c.tracer.Start(ctx, "AnalyzeIntent")
	defer span.End()

	ctx, cancel := context.WithTimeout(ctx, time.Duration(c.config.Timeout)*time.Second)
	defer cancel()

	grpcReq := &pb.AnalyzeIntentRequest{
		Content:    req.Content,
		Context:    req.Context,
		ProjectId:  req.ProjectID,
		UserId:     req.UserID,
	}

	resp, err := c.client.AnalyzeIntent(ctx, grpcReq)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to analyze intent: %w", err)
	}

	return &AnalyzeIntentResponse{
		IntentType:     resp.IntentType,
		Confidence:     resp.Confidence,
		Entities:       convertEntities(resp.Entities),
		RequiredParams: resp.RequiredParams,
		OptionalParams: resp.OptionalParams,
		Suggestions:    resp.Suggestions,
		Risks:          resp.Risks,
		EstimatedTime:  int(resp.EstimatedTimeSeconds),
		EstimatedCost:  resp.EstimatedCost,
	}, nil
}

// convertActions converts gRPC actions to internal format
func convertActions(pbActions []*pb.Action) []Action {
	actions := make([]Action, len(pbActions))
	for i, a := range pbActions {
		actions[i] = Action{
			ID:          a.Id,
			Type:        a.Type,
			Description: a.Description,
			Parameters:  a.Parameters,
			Status:      a.Status,
			Result:      a.Result,
		}
	}
	return actions
}

// convertEntities converts gRPC entities to internal format
func convertEntities(pbEntities []*pb.Entity) []Entity {
	entities := make([]Entity, len(pbEntities))
	for i, e := range pbEntities {
		entities[i] = Entity{
			Type:       e.Type,
			Value:      e.Value,
			Confidence: e.Confidence,
			Start:      int(e.Start),
			End:        int(e.End),
		}
	}
	return entities
}

// Request and response types

// ProcessIntentRequest represents a request to process an intent
type ProcessIntentRequest struct {
	Type           string                 `json:"type"`
	Content        string                 `json:"content"`
	Context        map[string]string      `json:"context"`
	Parameters     map[string]interface{} `json:"parameters"`
	Constraints    map[string]interface{} `json:"constraints"`
	ProjectID      string                 `json:"project_id"`
	UserID         string                 `json:"user_id"`
	RequestID      string                 `json:"request_id"`
	Async          bool                   `json:"async"`
	Priority       string                 `json:"priority"`
	TimeoutSeconds int                    `json:"timeout_seconds"`
	MaxRetries     int                    `json:"max_retries"`
}

// ProcessIntentResponse represents a response from processing an intent
type ProcessIntentResponse struct {
	IntentID    string                 `json:"intent_id"`
	Status      string                 `json:"status"`
	Message     string                 `json:"message"`
	Result      map[string]interface{} `json:"result"`
	Confidence  float32                `json:"confidence"`
	Actions     []Action               `json:"actions"`
	Suggestions []string               `json:"suggestions"`
	Metadata    map[string]string      `json:"metadata"`
}

// IntentStatus represents the status of an intent
type IntentStatus struct {
	IntentID    string    `json:"intent_id"`
	Status      string    `json:"status"`
	Progress    int       `json:"progress"`
	Message     string    `json:"message"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	Error       string    `json:"error"`
}

// AnalyzeIntentRequest represents a request to analyze an intent
type AnalyzeIntentRequest struct {
	Content   string            `json:"content"`
	Context   map[string]string `json:"context"`
	ProjectID string            `json:"project_id"`
	UserID    string            `json:"user_id"`
}

// AnalyzeIntentResponse represents a response from analyzing an intent
type AnalyzeIntentResponse struct {
	IntentType     string   `json:"intent_type"`
	Confidence     float32  `json:"confidence"`
	Entities       []Entity `json:"entities"`
	RequiredParams []string `json:"required_params"`
	OptionalParams []string `json:"optional_params"`
	Suggestions    []string `json:"suggestions"`
	Risks          []string `json:"risks"`
	EstimatedTime  int      `json:"estimated_time"`
	EstimatedCost  float32  `json:"estimated_cost"`
}

// Action represents an action to be performed
type Action struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Status      string                 `json:"status"`
	Result      map[string]interface{} `json:"result"`
}

// Entity represents an entity extracted from intent
type Entity struct {
	Type       string  `json:"type"`
	Value      string  `json:"value"`
	Confidence float32 `json:"confidence"`
	Start      int     `json:"start"`
	End        int     `json:"end"`
}