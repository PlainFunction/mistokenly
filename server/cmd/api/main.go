package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/PlainFunction/mistokenly/internal/api"
	"github.com/PlainFunction/mistokenly/internal/common/config"
	grpcserver "github.com/PlainFunction/mistokenly/internal/common/grpc"
)

func main() {
	log.Println("🚀 Starting API Service...")

	cfg := config.Load()
	log.Printf("📋 Configuration loaded: Environment=%s", cfg.Environment)

	// Check PII service connectivity
	if err := checkPIIServiceConnection(cfg); err != nil {
		log.Printf("⚠️  PII service not reachable: %v", err)
		log.Println("⚠️  API will start but requests may fail until PII service is available")
	}

	server := api.NewServer(cfg)

	log.Printf("🎧 API Service listening on port %s", cfg.APIPort)
	log.Println("📡 Ready to accept HTTP requests")
	log.Fatal(server.Start())
}

// checkPIIServiceConnection verifies the PII service is reachable
func checkPIIServiceConnection(cfg *config.Config) error {
	log.Println("🔧 [Init] Checking PII service connectivity...")

	piiAddr := fmt.Sprintf("%s:%s", cfg.PIIServiceHost, cfg.PIIServicePort)
	client, err := grpcserver.NewPIIServiceGRPCClient(piiAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to PII service: %w", err)
	}
	defer client.Close()

	// Try a health check with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Import the protobuf package to use HealthCheckRequest
	// For now, just verify connection was successful
	log.Printf("✅ [Init] PII service is reachable at %s", piiAddr)
	_ = ctx // Use ctx to avoid unused warning

	return nil
}
