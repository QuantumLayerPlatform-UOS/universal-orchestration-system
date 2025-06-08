module github.com/quantumlayer/uos/services/orchestrator

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/golang-migrate/migrate/v4 v4.16.2
	github.com/gorilla/websocket v1.5.1
	github.com/lib/pq v1.10.9
	github.com/prometheus/client_golang v1.17.0
	github.com/redis/go-redis/v9 v9.3.0
	github.com/spf13/cobra v1.8.0
	github.com/spf13/viper v1.17.0
	github.com/stretchr/testify v1.8.4
	go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin v0.46.1
	go.opentelemetry.io/otel v1.21.0
	go.opentelemetry.io/otel/exporters/jaeger v1.17.0
	go.opentelemetry.io/otel/sdk v1.21.0
	go.opentelemetry.io/otel/trace v1.21.0
	go.temporal.io/sdk v1.25.1
	go.uber.org/zap v1.26.0
	google.golang.org/grpc v1.59.0
	google.golang.org/protobuf v1.31.0
	gorm.io/driver/postgres v1.5.4
	gorm.io/gorm v1.25.5
)

require (
	// Indirect dependencies will be added by go mod tidy
)
