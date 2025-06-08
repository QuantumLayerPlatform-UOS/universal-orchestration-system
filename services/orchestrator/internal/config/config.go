package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration for the orchestrator service
type Config struct {
	Server       ServerConfig       `mapstructure:"server"`
	Database     DatabaseConfig     `mapstructure:"database"`
	Redis        RedisConfig        `mapstructure:"redis"`
	Temporal     TemporalConfig     `mapstructure:"temporal"`
	IntentAPI    IntentAPIConfig    `mapstructure:"intent_api"`
	AgentManager AgentManagerConfig `mapstructure:"agent_manager"`
	Telemetry    TelemetryConfig    `mapstructure:"telemetry"`
	Auth         AuthConfig         `mapstructure:"auth"`
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port              string `mapstructure:"port"`
	Host              string `mapstructure:"host"`
	ReadTimeout       int    `mapstructure:"read_timeout"`
	WriteTimeout      int    `mapstructure:"write_timeout"`
	ShutdownTimeout   int    `mapstructure:"shutdown_timeout"`
	MaxRequestSize    int64  `mapstructure:"max_request_size"`
	EnableProfiling   bool   `mapstructure:"enable_profiling"`
	EnableMetrics     bool   `mapstructure:"enable_metrics"`
	MetricsPort       string `mapstructure:"metrics_port"`
	GracefulShutdown  bool   `mapstructure:"graceful_shutdown"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	URL                   string `mapstructure:"url"`
	MaxOpenConns          int    `mapstructure:"max_open_conns"`
	MaxIdleConns          int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetime       int    `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime       int    `mapstructure:"conn_max_idle_time"`
	EnableAutoMigration   bool   `mapstructure:"enable_auto_migration"`
	LogLevel              string `mapstructure:"log_level"`
	SlowThreshold         int    `mapstructure:"slow_threshold"`
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Addr              string `mapstructure:"addr"`
	Password          string `mapstructure:"password"`
	DB                int    `mapstructure:"db"`
	MaxRetries        int    `mapstructure:"max_retries"`
	MinIdleConns      int    `mapstructure:"min_idle_conns"`
	MaxActiveConns    int    `mapstructure:"max_active_conns"`
	ConnMaxLifetime   int    `mapstructure:"conn_max_lifetime"`
	ReadTimeout       int    `mapstructure:"read_timeout"`
	WriteTimeout      int    `mapstructure:"write_timeout"`
	PoolTimeout       int    `mapstructure:"pool_timeout"`
	EnableTLS         bool   `mapstructure:"enable_tls"`
}

// TemporalConfig holds Temporal configuration
type TemporalConfig struct {
	HostPort                string `mapstructure:"host_port"`
	Namespace               string `mapstructure:"namespace"`
	TaskQueue               string `mapstructure:"task_queue"`
	WorkerOptions           WorkerOptions `mapstructure:"worker_options"`
	ClientOptions           ClientOptions `mapstructure:"client_options"`
	EnableMetrics           bool   `mapstructure:"enable_metrics"`
	MetricsScope            string `mapstructure:"metrics_scope"`
	MaxConcurrentActivities int    `mapstructure:"max_concurrent_activities"`
	MaxConcurrentWorkflows  int    `mapstructure:"max_concurrent_workflows"`
}

// WorkerOptions holds Temporal worker options
type WorkerOptions struct {
	MaxConcurrentActivityExecutionSize     int     `mapstructure:"max_concurrent_activity_execution_size"`
	MaxConcurrentWorkflowTaskExecutionSize int     `mapstructure:"max_concurrent_workflow_task_execution_size"`
	MaxConcurrentLocalActivityExecutionSize int    `mapstructure:"max_concurrent_local_activity_execution_size"`
	WorkerActivitiesPerSecond              float64 `mapstructure:"worker_activities_per_second"`
	TaskQueueActivitiesPerSecond           float64 `mapstructure:"task_queue_activities_per_second"`
	MaxTaskQueueActivitiesPerSecond        float64 `mapstructure:"max_task_queue_activities_per_second"`
	WorkerLocalActivitiesPerSecond         float64 `mapstructure:"worker_local_activities_per_second"`
	TaskQueueLocalActivitiesPerSecond      float64 `mapstructure:"task_queue_local_activities_per_second"`
}

// ClientOptions holds Temporal client options
type ClientOptions struct {
	ConnectionTimeout          int    `mapstructure:"connection_timeout"`
	RpcTimeout                 int    `mapstructure:"rpc_timeout"`
	RpcLongPollTimeout         int    `mapstructure:"rpc_long_poll_timeout"`
	RpcMaximumAttempts         int    `mapstructure:"rpc_maximum_attempts"`
	EnableKeepAlive            bool   `mapstructure:"enable_keep_alive"`
	KeepAliveTime              int    `mapstructure:"keep_alive_time"`
	KeepAliveTimeout           int    `mapstructure:"keep_alive_timeout"`
	KeepAlivePermitWithoutStream bool `mapstructure:"keep_alive_permit_without_stream"`
}

// IntentAPIConfig holds Intent API configuration
type IntentAPIConfig struct {
	Address            string `mapstructure:"address"`
	Timeout            int    `mapstructure:"timeout"`
	MaxRetries         int    `mapstructure:"max_retries"`
	RetryInterval      int    `mapstructure:"retry_interval"`
	EnableTLS          bool   `mapstructure:"enable_tls"`
	TLSCertFile        string `mapstructure:"tls_cert_file"`
	TLSKeyFile         string `mapstructure:"tls_key_file"`
	TLSCACertFile      string `mapstructure:"tls_ca_cert_file"`
	MaxConnectionIdle  int    `mapstructure:"max_connection_idle"`
	MaxConnectionAge   int    `mapstructure:"max_connection_age"`
	KeepAliveInterval  int    `mapstructure:"keep_alive_interval"`
	KeepAliveTimeout   int    `mapstructure:"keep_alive_timeout"`
}

// AgentManagerConfig holds Agent Manager configuration
type AgentManagerConfig struct {
	BaseURL              string `mapstructure:"base_url"`
	WebSocketURL         string `mapstructure:"websocket_url"`
	HTTPTimeout          int    `mapstructure:"http_timeout"`
	WebSocketTimeout     int    `mapstructure:"websocket_timeout"`
	MaxRetries           int    `mapstructure:"max_retries"`
	RetryInterval        int    `mapstructure:"retry_interval"`
	PingInterval         int    `mapstructure:"ping_interval"`
	PongTimeout          int    `mapstructure:"pong_timeout"`
	MaxReconnectAttempts int    `mapstructure:"max_reconnect_attempts"`
	ReconnectInterval    int    `mapstructure:"reconnect_interval"`
	BufferSize           int    `mapstructure:"buffer_size"`
	EnableCompression    bool   `mapstructure:"enable_compression"`
}

// TelemetryConfig holds telemetry configuration
type TelemetryConfig struct {
	Enabled              bool              `mapstructure:"enabled"`
	ServiceName          string            `mapstructure:"service_name"`
	ServiceVersion       string            `mapstructure:"service_version"`
	Environment          string            `mapstructure:"environment"`
	Jaeger               JaegerConfig      `mapstructure:"jaeger"`
	Prometheus           PrometheusConfig  `mapstructure:"prometheus"`
	SamplingRate         float64           `mapstructure:"sampling_rate"`
	EnableDistributedTracing bool          `mapstructure:"enable_distributed_tracing"`
	EnableMetrics        bool              `mapstructure:"enable_metrics"`
	EnableLogging        bool              `mapstructure:"enable_logging"`
	LogLevel             string            `mapstructure:"log_level"`
}

// JaegerConfig holds Jaeger configuration
type JaegerConfig struct {
	Endpoint            string `mapstructure:"endpoint"`
	AgentHost           string `mapstructure:"agent_host"`
	AgentPort           int    `mapstructure:"agent_port"`
	CollectorEndpoint   string `mapstructure:"collector_endpoint"`
	Username            string `mapstructure:"username"`
	Password            string `mapstructure:"password"`
	UseAgent            bool   `mapstructure:"use_agent"`
	BufferMaxCount      int    `mapstructure:"buffer_max_count"`
	BatchMaxCount       int    `mapstructure:"batch_max_count"`
}

// PrometheusConfig holds Prometheus configuration
type PrometheusConfig struct {
	PushGateway         string            `mapstructure:"push_gateway"`
	PushInterval        int               `mapstructure:"push_interval"`
	MetricsPath         string            `mapstructure:"metrics_path"`
	DefaultLabels       map[string]string `mapstructure:"default_labels"`
	EnableGoMetrics     bool              `mapstructure:"enable_go_metrics"`
	EnableProcessMetrics bool             `mapstructure:"enable_process_metrics"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Enabled           bool     `mapstructure:"enabled"`
	JWTSecret         string   `mapstructure:"jwt_secret"`
	JWTExpiration     int      `mapstructure:"jwt_expiration"`
	JWTRefreshExpiration int  `mapstructure:"jwt_refresh_expiration"`
	APIKeyHeader      string   `mapstructure:"api_key_header"`
	APIKeys           []string `mapstructure:"api_keys"`
	EnableOAuth       bool     `mapstructure:"enable_oauth"`
	OAuthProviders    []string `mapstructure:"oauth_providers"`
}

// Load loads configuration from environment variables and config files
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/etc/orchestrator")

	// Set default values
	setDefaults()

	// Read environment variables
	viper.SetEnvPrefix("ORCHESTRATOR")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		// It's okay if config file doesn't exist, we'll use defaults and env vars
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate configuration
	if err := validate(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Server defaults
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.read_timeout", 30)
	viper.SetDefault("server.write_timeout", 30)
	viper.SetDefault("server.shutdown_timeout", 30)
	viper.SetDefault("server.max_request_size", 10485760) // 10MB
	viper.SetDefault("server.enable_profiling", false)
	viper.SetDefault("server.enable_metrics", true)
	viper.SetDefault("server.metrics_port", "9090")
	viper.SetDefault("server.graceful_shutdown", true)

	// Database defaults
	viper.SetDefault("database.url", "postgres://postgres:postgres@localhost:5432/orchestrator?sslmode=disable")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 5)
	viper.SetDefault("database.conn_max_lifetime", 300)
	viper.SetDefault("database.conn_max_idle_time", 30)
	viper.SetDefault("database.enable_auto_migration", true)
	viper.SetDefault("database.log_level", "warn")
	viper.SetDefault("database.slow_threshold", 200)

	// Redis defaults
	viper.SetDefault("redis.addr", "localhost:6379")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.max_retries", 3)
	viper.SetDefault("redis.min_idle_conns", 5)
	viper.SetDefault("redis.max_active_conns", 100)
	viper.SetDefault("redis.conn_max_lifetime", 300)
	viper.SetDefault("redis.read_timeout", 3)
	viper.SetDefault("redis.write_timeout", 3)
	viper.SetDefault("redis.pool_timeout", 4)
	viper.SetDefault("redis.enable_tls", false)

	// Temporal defaults
	viper.SetDefault("temporal.host_port", "localhost:7233")
	viper.SetDefault("temporal.namespace", "default")
	viper.SetDefault("temporal.task_queue", "orchestrator-task-queue")
	viper.SetDefault("temporal.enable_metrics", true)
	viper.SetDefault("temporal.metrics_scope", "orchestrator")
	viper.SetDefault("temporal.max_concurrent_activities", 100)
	viper.SetDefault("temporal.max_concurrent_workflows", 100)
	
	// Temporal worker options defaults
	viper.SetDefault("temporal.worker_options.max_concurrent_activity_execution_size", 100)
	viper.SetDefault("temporal.worker_options.max_concurrent_workflow_task_execution_size", 100)
	viper.SetDefault("temporal.worker_options.max_concurrent_local_activity_execution_size", 100)
	viper.SetDefault("temporal.worker_options.worker_activities_per_second", 100000.0)
	viper.SetDefault("temporal.worker_options.task_queue_activities_per_second", 100000.0)
	viper.SetDefault("temporal.worker_options.max_task_queue_activities_per_second", 100000.0)
	viper.SetDefault("temporal.worker_options.worker_local_activities_per_second", 100000.0)
	viper.SetDefault("temporal.worker_options.task_queue_local_activities_per_second", 100000.0)

	// Temporal client options defaults
	viper.SetDefault("temporal.client_options.connection_timeout", 10)
	viper.SetDefault("temporal.client_options.rpc_timeout", 10)
	viper.SetDefault("temporal.client_options.rpc_long_poll_timeout", 60)
	viper.SetDefault("temporal.client_options.rpc_maximum_attempts", 3)
	viper.SetDefault("temporal.client_options.enable_keep_alive", true)
	viper.SetDefault("temporal.client_options.keep_alive_time", 30)
	viper.SetDefault("temporal.client_options.keep_alive_timeout", 10)
	viper.SetDefault("temporal.client_options.keep_alive_permit_without_stream", true)

	// Intent API defaults
	viper.SetDefault("intent_api.address", "localhost:50051")
	viper.SetDefault("intent_api.timeout", 30)
	viper.SetDefault("intent_api.max_retries", 3)
	viper.SetDefault("intent_api.retry_interval", 1)
	viper.SetDefault("intent_api.enable_tls", false)
	viper.SetDefault("intent_api.max_connection_idle", 300)
	viper.SetDefault("intent_api.max_connection_age", 600)
	viper.SetDefault("intent_api.keep_alive_interval", 30)
	viper.SetDefault("intent_api.keep_alive_timeout", 10)

	// Agent Manager defaults
	viper.SetDefault("agent_manager.base_url", "http://localhost:8081")
	viper.SetDefault("agent_manager.websocket_url", "ws://localhost:8081")
	viper.SetDefault("agent_manager.http_timeout", 30)
	viper.SetDefault("agent_manager.websocket_timeout", 60)
	viper.SetDefault("agent_manager.max_retries", 3)
	viper.SetDefault("agent_manager.retry_interval", 1)
	viper.SetDefault("agent_manager.ping_interval", 30)
	viper.SetDefault("agent_manager.pong_timeout", 10)
	viper.SetDefault("agent_manager.max_reconnect_attempts", 5)
	viper.SetDefault("agent_manager.reconnect_interval", 5)
	viper.SetDefault("agent_manager.buffer_size", 1024)
	viper.SetDefault("agent_manager.enable_compression", true)

	// Telemetry defaults
	viper.SetDefault("telemetry.enabled", true)
	viper.SetDefault("telemetry.service_name", "orchestrator")
	viper.SetDefault("telemetry.service_version", "1.0.0")
	viper.SetDefault("telemetry.environment", "development")
	viper.SetDefault("telemetry.sampling_rate", 1.0)
	viper.SetDefault("telemetry.enable_distributed_tracing", true)
	viper.SetDefault("telemetry.enable_metrics", true)
	viper.SetDefault("telemetry.enable_logging", true)
	viper.SetDefault("telemetry.log_level", "info")

	// Jaeger defaults
	viper.SetDefault("telemetry.jaeger.agent_host", "localhost")
	viper.SetDefault("telemetry.jaeger.agent_port", 6831)
	viper.SetDefault("telemetry.jaeger.use_agent", true)
	viper.SetDefault("telemetry.jaeger.buffer_max_count", 100)
	viper.SetDefault("telemetry.jaeger.batch_max_count", 100)

	// Prometheus defaults
	viper.SetDefault("telemetry.prometheus.metrics_path", "/metrics")
	viper.SetDefault("telemetry.prometheus.push_interval", 10)
	viper.SetDefault("telemetry.prometheus.enable_go_metrics", true)
	viper.SetDefault("telemetry.prometheus.enable_process_metrics", true)

	// Auth defaults
	viper.SetDefault("auth.enabled", false)
	viper.SetDefault("auth.jwt_expiration", 3600)
	viper.SetDefault("auth.jwt_refresh_expiration", 86400)
	viper.SetDefault("auth.api_key_header", "X-API-Key")
	viper.SetDefault("auth.enable_oauth", false)
}

// validate validates the configuration
func validate(cfg *Config) error {
	if cfg.Server.Port == "" {
		return fmt.Errorf("server port is required")
	}

	if cfg.Database.URL == "" {
		return fmt.Errorf("database URL is required")
	}

	if cfg.Temporal.HostPort == "" {
		return fmt.Errorf("temporal host:port is required")
	}

	if cfg.Temporal.TaskQueue == "" {
		return fmt.Errorf("temporal task queue is required")
	}

	if cfg.Auth.Enabled && cfg.Auth.JWTSecret == "" {
		return fmt.Errorf("JWT secret is required when auth is enabled")
	}

	return nil
}