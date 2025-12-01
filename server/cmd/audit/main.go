package main

import (
	"database/sql"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/PlainFunction/mistokenly/internal/common/config"
	"github.com/PlainFunction/mistokenly/internal/common/db"
	"github.com/PlainFunction/mistokenly/internal/services"
	pb "github.com/PlainFunction/mistokenly/proto/audit"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {
	log.Println("🚀 Starting Audit Service...")

	// Load configuration
	cfg := config.Load()
	log.Printf("📋 Configuration loaded: Environment=%s", cfg.Environment)

	// Initialize audit database (run migrations)
	if err := initializeAuditDatabase(cfg); err != nil {
		log.Fatalf("❌ Failed to initialize audit database: %v", err)
	}

	// Create audit service instance
	auditService, err := services.NewAuditService(cfg)
	if err != nil {
		log.Fatalf("❌ Failed to create audit service: %v", err)
	}
	log.Println("✅ Audit service instance created")

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Register audit service
	pb.RegisterAuditServiceServer(grpcServer, auditService)
	log.Println("✅ Audit service registered with gRPC server")

	// Register health check service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("audit-service", grpc_health_v1.HealthCheckResponse_SERVING)
	log.Println("✅ Health check service registered")

	// Register reflection service for debugging
	reflection.Register(grpcServer)
	log.Println("✅ gRPC reflection registered")

	// Start gRPC server
	grpcPort := cfg.GRPCAuditPort
	if grpcPort == "" {
		grpcPort = "50053" // Default audit service port
	}
	lis, err := net.Listen("tcp", "0.0.0.0:"+grpcPort)
	if err != nil {
		log.Fatalf("❌ Failed to listen on port %s: %v", grpcPort, err)
	}

	log.Printf("🎧 Audit Service listening on 0.0.0.0:%s", grpcPort)
	log.Println("📡 Ready to accept gRPC requests")
	log.Println("📊 Audit logging enabled with database persistence")

	// Handle graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start gRPC server in a goroutine
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("❌ Failed to serve gRPC: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-stop
	log.Println("🛑 Shutdown signal received, gracefully stopping...")

	// Stop gRPC server
	grpcServer.GracefulStop()

	// Close audit service
	if err := auditService.Close(); err != nil {
		log.Printf("⚠️  Error closing audit service: %v", err)
	}

	log.Println("✅ Audit Service stopped")
}

// initializeAuditDatabase runs database migrations for the audit database
func initializeAuditDatabase(cfg *config.Config) error {
	log.Println("🔧 [Init] Initializing audit database...")

	// Use the same database as persistence service for now
	// In production, you might want a separate audit database
	dbURL := cfg.DatabaseURL
	if cfg.AuditDatabaseURL != "" {
		dbURL = cfg.AuditDatabaseURL
	}

	// Connect to audit database
	auditDB, err := sql.Open("postgres", dbURL)
	if err != nil {
		return err
	}
	defer auditDB.Close()

	// Test connection
	if err := auditDB.Ping(); err != nil {
		return err
	}
	log.Println("✅ [Init] Connected to audit database")

	// Determine migrations directory
	migrationsDir := os.Getenv("MIGRATIONS_DIR")
	if migrationsDir == "" {
		// Default to migrations directory relative to executable
		migrationsDir = "migrations"
	}

	// Check if migrations directory exists
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		log.Printf("⚠️  [Init] Migrations directory not found: %s", migrationsDir)
		log.Println("⚠️  [Init] Skipping migrations - assuming schema already exists")
		return nil
	}

	// Run migrations
	migrator := db.NewMigrator(auditDB, migrationsDir)
	if err := migrator.MigrateUp(); err != nil {
		return err
	}

	log.Println("✅ [Init] Audit database initialized successfully")
	return nil
}
