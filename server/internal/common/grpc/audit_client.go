package grpc

import (
	"context"
	"fmt"
	"log"

	pb "github.com/PlainFunction/mistokenly/proto/audit"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AuditServiceGRPCClient wraps a gRPC client connection to the remote Audit service
type AuditServiceGRPCClient struct {
	conn   *grpc.ClientConn
	client pb.AuditServiceClient
}

// NewAuditServiceGRPCClient creates a new gRPC client for the Audit service
func NewAuditServiceGRPCClient(serviceAddr string) (*AuditServiceGRPCClient, error) {
	log.Printf("[gRPC Client] Connecting to Audit service at %s", serviceAddr)

	// Create gRPC connection
	conn, err := grpc.Dial(serviceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Audit service: %w", err)
	}

	// Create gRPC client
	client := pb.NewAuditServiceClient(conn)

	// Test the connection with a health check
	healthReq := &pb.HealthCheckRequest{
		ServiceName: "audit-client-connection-test",
	}
	_, err = client.HealthCheck(context.Background(), healthReq)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to verify Audit service connection: %w", err)
	}

	log.Printf("[gRPC Client] Successfully connected to Audit service")

	return &AuditServiceGRPCClient{
		conn:   conn,
		client: client,
	}, nil
}

// Close closes the gRPC connection
func (c *AuditServiceGRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// LogAccess calls the remote Audit service to log access events
func (c *AuditServiceGRPCClient) LogAccess(ctx context.Context, req *pb.LogAccessRequest) (*pb.LogAccessResponse, error) {
	log.Printf("[gRPC Client] Calling remote LogAccess for operation: %s", req.Operation)

	// Make gRPC call directly with protobuf types
	resp, err := c.client.LogAccess(ctx, req)
	if err != nil {
		log.Printf("[gRPC Client] LogAccess failed: %v", err)
		return nil, fmt.Errorf("gRPC log access failed: %w", err)
	}

	return resp, nil
}

// GetAuditLogs calls the remote Audit service to retrieve audit logs
func (c *AuditServiceGRPCClient) GetAuditLogs(ctx context.Context, req *pb.GetAuditLogsRequest) (*pb.GetAuditLogsResponse, error) {
	log.Printf("[gRPC Client] Calling remote GetAuditLogs")

	// Make gRPC call directly with protobuf types
	resp, err := c.client.GetAuditLogs(ctx, req)
	if err != nil {
		log.Printf("[gRPC Client] GetAuditLogs failed: %v", err)
		return nil, fmt.Errorf("gRPC get audit logs failed: %w", err)
	}

	return resp, nil
}

// HealthCheck calls the remote Audit service health check
func (c *AuditServiceGRPCClient) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	log.Printf("[gRPC Client] Calling remote HealthCheck")

	// Make gRPC call directly with protobuf types
	resp, err := c.client.HealthCheck(ctx, req)
	if err != nil {
		log.Printf("[gRPC Client] HealthCheck failed: %v", err)
		return nil, fmt.Errorf("gRPC health check failed: %w", err)
	}

	return resp, nil
}
