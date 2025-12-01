package grpc

import (
	"fmt"
	"os"
	"sync"

	grpclib "google.golang.org/grpc"

	"github.com/PlainFunction/mistokenly/internal/common/config"
	"github.com/PlainFunction/mistokenly/internal/common/types"
	"github.com/PlainFunction/mistokenly/internal/services"
)

// ServiceRegistry manages gRPC client connections to other services
type ServiceRegistry struct {
	config  *config.Config
	clients map[string]*grpclib.ClientConn
	mutex   sync.RWMutex
}

// NewServiceRegistry creates a new service registry
func NewServiceRegistry(cfg *config.Config) *ServiceRegistry {
	return &ServiceRegistry{
		config:  cfg,
		clients: make(map[string]*grpclib.ClientConn),
	}
}

// GetPIIServiceClient returns a client connection to the PII service
func (sr *ServiceRegistry) GetPIIServiceClient() (PIIServiceInterface, error) {
	// Mocks removed - only use real services

	// Use environment variable to decide between local or remote service
	useRemoteService := getEnv("USE_REMOTE_SERVICES", "true") == "true"

	if useRemoteService {
		// Create gRPC client to remote PII service
		serviceAddr := fmt.Sprintf("%s:%s", sr.config.PIIServiceHost, sr.config.PIIServicePort)
		fmt.Printf("🌐 [Registry] Connecting to REMOTE PII Service at %s\n", serviceAddr)
		return NewPIIServiceGRPCClient(serviceAddr)
	}

	// Create local PII service (monolithic mode for development)
	fmt.Printf("🏠 [Registry] Using LOCAL PII Service\n")
	return services.NewPIIService(sr.config)
}

// GetPersistenceServiceClient returns a client connection to the Persistence service
func (sr *ServiceRegistry) GetPersistenceServiceClient() (types.PersistenceServiceInterface, error) {
	// Use environment variable to decide between local or remote service
	useRemoteService := getEnv("USE_REMOTE_SERVICES", "true") == "true"

	if useRemoteService {
		// Create gRPC client to remote Persistence service
		serviceAddr := fmt.Sprintf("%s:%s", sr.config.PersistServiceHost, sr.config.PersistServicePort)
		fmt.Printf("🌐 [Registry] Connecting to REMOTE Persistence Service at %s\n", serviceAddr)
		return NewPersistenceServiceGRPCClient(serviceAddr)
	}

	// Create local Persistence service (monolithic mode for development)
	fmt.Printf("🏠 [Registry] Using LOCAL Persistence Service\n")
	return services.NewPersistenceService(sr.config)
}

// GetAuditServiceClient returns a client connection to the Audit service
func (sr *ServiceRegistry) GetAuditServiceClient() (types.AuditServiceInterface, error) {
	// Use environment variable to decide between local or remote service
	useRemoteService := getEnv("USE_REMOTE_SERVICES", "true") == "true"

	if useRemoteService {
		// Create gRPC client to remote Audit service
		serviceAddr := fmt.Sprintf("%s:%s", sr.config.AuditServiceHost, sr.config.AuditServicePort)
		fmt.Printf("🌐 [Registry] Connecting to REMOTE Audit Service at %s\n", serviceAddr)
		return NewAuditServiceGRPCClient(serviceAddr)
	}

	// Create local Audit service (monolithic mode for development)
	fmt.Printf("🏠 [Registry] Using LOCAL Audit Service\n")
	return services.NewAuditService(sr.config)
}

// GetServiceEndpoint returns the endpoint for a service
func (sr *ServiceRegistry) GetServiceEndpoint(serviceName string) string {
	switch serviceName {
	case "api":
		return fmt.Sprintf("pii-api-service:%s", sr.config.GetGRPCPort("api"))
	case "audit":
		return fmt.Sprintf("pii-audit-service:%s", sr.config.GetGRPCPort("audit"))
	case "persistence":
		return fmt.Sprintf("pii-persistence-service:%s", sr.config.GetGRPCPort("persistence"))
	default:
		return fmt.Sprintf("localhost:%s", sr.config.GetGRPCPort(serviceName))
	}
}

// Close closes all client connections
func (sr *ServiceRegistry) Close() {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()

	for name, conn := range sr.clients {
		if err := conn.Close(); err != nil {
			fmt.Printf("Error closing connection to %s: %v\n", name, err)
		}
	}
	sr.clients = make(map[string]*grpclib.ClientConn)
}

// Helper function for environment variables
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
