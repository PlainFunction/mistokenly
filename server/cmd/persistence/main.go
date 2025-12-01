package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/PlainFunction/mistokenly/internal/common/config"
	"github.com/PlainFunction/mistokenly/internal/common/db"
	"github.com/PlainFunction/mistokenly/internal/services"
	pb "github.com/PlainFunction/mistokenly/proto/persistence"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {
	log.Println("🚀 Starting Persistence Service...")

	// Load configuration
	cfg := config.Load()
	log.Printf("📋 Configuration loaded: Environment=%s", cfg.Environment)

	// Initialize storage database (run migrations)
	if err := initializeStorageDatabase(cfg); err != nil {
		log.Fatalf("❌ Failed to initialize storage database: %v", err)
	}

	// Initialize PGMQ database (install extension and create queue)
	if err := initializePGMQDatabase(cfg); err != nil {
		log.Fatalf("❌ Failed to initialize PGMQ database: %v", err)
	}

	// Create persistence service instance
	persistenceService, err := services.NewPersistenceService(cfg)
	if err != nil {
		log.Fatalf("❌ Failed to create persistence service: %v", err)
	}
	log.Println("✅ Persistence service instance created")

	// Start PGMQ workers to process queue messages (3 concurrent workers)
	persistenceService.StartWorkers(3)

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Register persistence service
	pb.RegisterPersistenceServiceServer(grpcServer, persistenceService)
	log.Println("✅ Persistence service registered with gRPC server")

	// Register health check service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("persistence-service", grpc_health_v1.HealthCheckResponse_SERVING)
	log.Println("✅ Health check service registered")

	// Register reflection service for debugging
	reflection.Register(grpcServer)
	log.Println("✅ gRPC reflection registered")

	// Start gRPC server
	grpcPort := cfg.GRPCPersistPort
	lis, err := net.Listen("tcp", "0.0.0.0:"+grpcPort)
	if err != nil {
		log.Fatalf("❌ Failed to listen on port %s: %v", grpcPort, err)
	}

	log.Printf("🎧 Persistence Service listening on 0.0.0.0:%s", grpcPort)
	log.Println("📡 Ready to accept gRPC requests")
	log.Println("🔄 PGMQ workers processing messages from queue")

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

	// Close persistence service and workers
	if err := persistenceService.Close(); err != nil {
		log.Printf("⚠️  Error closing persistence service: %v", err)
	}

	log.Println("✅ Persistence Service stopped")
}

// initializeStorageDatabase runs database migrations for the storage database
func initializeStorageDatabase(cfg *config.Config) error {
	log.Println("🔧 [Init] Initializing storage database...")

	// Ensure the database exists, create if not
	if err := db.EnsureDatabase(cfg.DatabaseURL); err != nil {
		return fmt.Errorf("failed to ensure storage database exists: %w", err)
	}

	// Connect to storage database
	storageDB, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer storageDB.Close()

	// Test connection
	if err := storageDB.Ping(); err != nil {
		return err
	}
	log.Println("✅ [Init] Connected to storage database")

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
	migrator := db.NewMigrator(storageDB, migrationsDir)
	if err := migrator.MigrateUp(); err != nil {
		return err
	}

	log.Println("✅ [Init] Storage database initialized successfully")
	return nil
}

// initializePGMQDatabase initializes PGMQ extension and creates required queues
func initializePGMQDatabase(cfg *config.Config) error {
	log.Println("🔧 [Init] Initializing PGMQ database...")

	// Ensure the database exists, create if not
	if err := db.EnsureDatabase(cfg.PGMQDatabaseURL); err != nil {
		return fmt.Errorf("failed to ensure PGMQ database exists: %w", err)
	}

	// Connect to PGMQ database
	pgmqDB, err := sql.Open("postgres", cfg.PGMQDatabaseURL)
	if err != nil {
		return err
	}
	defer pgmqDB.Close()

	// Test connection
	if err := pgmqDB.Ping(); err != nil {
		return err
	}
	log.Println("✅ [Init] Connected to PGMQ database")

	// Initialize PGMQ
	initializer := db.NewPGMQInitializer(pgmqDB)
	if err := initializer.Initialize("pii_token_persistence"); err != nil {
		return err
	}

	// Display queue stats
	stats, err := initializer.GetQueueStats("pii_token_persistence")
	if err != nil {
		log.Printf("⚠️  [Init] Could not get queue stats: %v", err)
	} else {
		log.Printf("📊 [Init] Queue stats: %v", stats)
	}

	log.Println("✅ [Init] PGMQ database initialized successfully")
	return nil
}
