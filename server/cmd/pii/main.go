package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/PlainFunction/mistokenly/internal/common/config"
	"github.com/PlainFunction/mistokenly/internal/common/db"
	grpcserver "github.com/PlainFunction/mistokenly/internal/common/grpc"
	"github.com/PlainFunction/mistokenly/internal/services"
	pb "github.com/PlainFunction/mistokenly/proto/pii"
	_ "github.com/lib/pq"
)

func main() {
	log.Println("🚀 Starting PII Service...")

	// Load configuration
	cfg := config.Load()
	log.Printf("📋 Configuration loaded: Environment=%s", cfg.Environment)

	// Initialize PGMQ (check extension and queue)
	if err := initializePGMQ(cfg); err != nil {
		log.Printf("⚠️  PGMQ initialization failed: %v (async persistence disabled)", err)
	}

	// Create the PII service implementation
	piiService, err := services.NewPIIService(cfg)
	if err != nil {
		log.Fatalf("❌ Failed to create PII service: %v", err)
	}
	log.Println("✅ PII service instance created")

	// Initialize persistence service gRPC client (optional - for cache miss queries)
	persistenceAddr := fmt.Sprintf("%s:%s", cfg.PersistServiceHost, cfg.PersistServicePort)
	persistenceClient, err := grpcserver.NewPersistenceServiceGRPCClient(persistenceAddr)
	if err != nil {
		log.Printf("⚠️  Persistence service connection failed: %v (cache miss queries disabled)", err)
	} else {
		piiService.SetPersistenceClient(persistenceClient)
		log.Printf("✅ Persistence service connected at %s", persistenceAddr)
	}

	// Create gRPC server wrapper
	piiServerWrapper := grpcserver.NewPIIServiceServer(piiService)
	log.Println("✅ gRPC server wrapper created")

	// Create gRPC server with options
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(10*1024*1024), // 10MB max message size
		grpc.MaxSendMsgSize(10*1024*1024),
	)

	// Register PII service
	pb.RegisterPIIServiceServer(grpcServer, piiServerWrapper)
	log.Println("✅ PII service registered with gRPC server")

	// Register health check service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("pii.PIIService", grpc_health_v1.HealthCheckResponse_SERVING)
	log.Println("✅ Health check service registered")

	// Register reflection service (useful for debugging with grpcurl)
	reflection.Register(grpcServer)
	log.Println("✅ gRPC reflection registered")

	// Get port from config or environment
	port := cfg.PIIServicePort
	if port == "" {
		port = os.Getenv("PII_SERVICE_PORT")
		if port == "" {
			port = "9080"
		}
	}

	// Create listener
	addr := fmt.Sprintf("0.0.0.0:%s", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("❌ Failed to listen on %s: %v", addr, err)
	}

	log.Printf("🎧 PII Service listening on %s", addr)
	log.Println("📡 Ready to accept gRPC requests")

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		log.Println("🛑 Shutdown signal received, gracefully stopping...")
		grpcServer.GracefulStop()
		log.Println("✅ PII Service stopped")
	}()

	// Start serving
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("❌ Failed to serve: %v", err)
	}
}

// initializePGMQ checks PGMQ connectivity and ensures the queue exists
func initializePGMQ(cfg *config.Config) error {
	log.Println("🔧 [Init] Checking PGMQ database...")

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

	// Check PGMQ extension and queue
	initializer := db.NewPGMQInitializer(pgmqDB)

	// Check if extension exists
	exists, err := initializer.CheckExtension()
	if err != nil {
		return err
	}

	if !exists {
		log.Println("⚠️  [Init] PGMQ extension not installed - async persistence will be disabled")
		return fmt.Errorf("PGMQ extension not installed")
	}
	log.Println("✅ [Init] PGMQ extension is installed")

	// Check if queue exists
	queueExists, err := initializer.CheckQueue("pii_token_persistence")
	if err != nil {
		return err
	}

	if !queueExists {
		log.Println("⚠️  [Init] pii_token_persistence queue does not exist - async persistence will be disabled")
		return fmt.Errorf("pii_token_persistence queue not found")
	}
	log.Println("✅ [Init] pii_token_persistence queue exists")

	log.Println("✅ [Init] PGMQ initialization complete")
	return nil
}
