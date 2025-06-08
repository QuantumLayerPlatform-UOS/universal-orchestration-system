package database

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/quantumlayer/uos/services/orchestrator/internal/models"
)

// Connect establishes a database connection
func Connect(databaseURL string) (*gorm.DB, error) {
	// Configure GORM
	config := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
		PrepareStmt:                              true,
		DisableForeignKeyConstraintWhenMigrating: true,
	}

	// Open database connection
	db, err := gorm.Open(postgres.Open(databaseURL), config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying SQL database
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetConnMaxIdleTime(30 * time.Second)

	// Test connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Run migrations
	if err := AutoMigrate(db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// AutoMigrate runs database migrations
func AutoMigrate(db *gorm.DB) error {
	// Enable PostgreSQL extensions
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"").Error; err != nil {
		return fmt.Errorf("failed to create uuid extension: %w", err)
	}

	// Migrate models
	models := []interface{}{
		// Project models
		&models.Project{},
		&models.ProjectMember{},
		&models.Environment{},
		&models.Resource{},
		&models.Integration{},

		// Workflow models
		&models.Workflow{},
		&models.WorkflowStep{},
		&models.WorkflowTemplate{},
		&models.WorkflowExecution{},

		// Execution models
		&models.Execution{},
		&models.Artifact{},
		&models.Metric{},
		&models.ExecutionLog{},
		&models.ExecutionEvent{},
	}

	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to migrate %T: %w", model, err)
		}
	}

	// Create indexes
	if err := createIndexes(db); err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}

// createIndexes creates database indexes for better performance
func createIndexes(db *gorm.DB) error {
	indexes := []struct {
		table string
		name  string
		columns string
	}{
		// Project indexes
		{"projects", "idx_projects_status", "status"},
		{"projects", "idx_projects_owner", "owner_id"},
		{"projects", "idx_projects_org", "organization_id"},
		{"projects", "idx_projects_created", "created_at"},
		
		// Workflow indexes
		{"workflows", "idx_workflows_project", "project_id"},
		{"workflows", "idx_workflows_status", "status"},
		{"workflows", "idx_workflows_type", "type"},
		{"workflows", "idx_workflows_temporal", "temporal_id"},
		{"workflows", "idx_workflows_created", "created_at"},
		
		// Execution indexes
		{"executions", "idx_executions_project", "project_id"},
		{"executions", "idx_executions_workflow", "workflow_id"},
		{"executions", "idx_executions_agent", "agent_id"},
		{"executions", "idx_executions_status", "status"},
		{"executions", "idx_executions_type", "type"},
		{"executions", "idx_executions_created", "created_at"},
		
		// Member indexes
		{"project_members", "idx_members_user", "user_id"},
		{"project_members", "idx_members_project_user", "project_id,user_id"},
		
		// Composite indexes
		{"workflows", "idx_workflows_project_status", "project_id,status"},
		{"executions", "idx_executions_workflow_status", "workflow_id,status"},
	}

	for _, idx := range indexes {
		sql := fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s (%s)", idx.name, idx.table, idx.columns)
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("failed to create index %s: %w", idx.name, err)
		}
	}

	return nil
}

// Health checks database health
func Health(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Check if we can query
	var count int64
	if err := db.WithContext(ctx).Model(&models.Project{}).Count(&count).Error; err != nil {
		return fmt.Errorf("database query failed: %w", err)
	}

	return nil
}