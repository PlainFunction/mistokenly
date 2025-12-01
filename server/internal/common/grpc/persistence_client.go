package grpc

import (
	"context"
	"fmt"
	"log"

	pb "github.com/PlainFunction/mistokenly/proto/persistence"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// PersistenceServiceGRPCClient wraps a gRPC client connection to the remote Persistence service
type PersistenceServiceGRPCClient struct {
	conn   *grpc.ClientConn
	client pb.PersistenceServiceClient
}

// NewPersistenceServiceGRPCClient creates a new gRPC client for the Persistence service
func NewPersistenceServiceGRPCClient(serviceAddr string) (*PersistenceServiceGRPCClient, error) {
	log.Printf("[gRPC Client] Connecting to Persistence service at %s", serviceAddr)

	// Create gRPC connection
	conn, err := grpc.Dial(serviceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Persistence service: %w", err)
	}

	// Create gRPC client
	client := pb.NewPersistenceServiceClient(conn)

	// Test the connection with a health check
	healthReq := &pb.HealthCheckRequest{
		ServiceName: "persistence-client-connection-test",
	}
	_, err = client.HealthCheck(context.Background(), healthReq)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to verify Persistence service connection: %w", err)
	}

	log.Printf("[gRPC Client] Successfully connected to Persistence service")

	return &PersistenceServiceGRPCClient{
		conn:   conn,
		client: client,
	}, nil
}

// Close closes the gRPC connection
func (c *PersistenceServiceGRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// StorePIIToken calls the remote Persistence service to store a PII token
func (c *PersistenceServiceGRPCClient) StorePIIToken(ctx context.Context, req *pb.StorePIITokenRequest) (*pb.StorePIITokenResponse, error) {
	log.Printf("[gRPC Client] Calling remote StorePIIToken for token: %s", req.ReferenceHash)

	// Make gRPC call directly with protobuf types
	resp, err := c.client.StorePIIToken(ctx, req)
	if err != nil {
		log.Printf("[gRPC Client] StorePIIToken failed: %v", err)
		return nil, fmt.Errorf("gRPC store PII token failed: %w", err)
	}

	return resp, nil
}

// RetrievePIIToken calls the remote Persistence service to retrieve a PII token
func (c *PersistenceServiceGRPCClient) RetrievePIIToken(ctx context.Context, req *pb.RetrievePIITokenRequest) (*pb.RetrievePIITokenResponse, error) {
	log.Printf("[gRPC Client] Calling remote RetrievePIIToken for hash: %s", req.ReferenceHash)

	// Make gRPC call directly with protobuf types
	resp, err := c.client.RetrievePIIToken(ctx, req)
	if err != nil {
		log.Printf("[gRPC Client] RetrievePIIToken failed: %v", err)
		return nil, fmt.Errorf("gRPC retrieve PII token failed: %w", err)
	}

	return resp, nil
}

// HealthCheck calls the remote Persistence service health check
func (c *PersistenceServiceGRPCClient) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	log.Printf("[gRPC Client] Calling remote HealthCheck")

	// Make gRPC call directly with protobuf types
	resp, err := c.client.HealthCheck(ctx, req)
	if err != nil {
		log.Printf("[gRPC Client] HealthCheck failed: %v", err)
		return nil, fmt.Errorf("gRPC health check failed: %w", err)
	}

	return resp, nil
}

// StoreTEK calls the remote Persistence service to store a TEK
func (c *PersistenceServiceGRPCClient) StoreTEK(ctx context.Context, req *pb.StoreTEKRequest) (*pb.StoreTEKResponse, error) {
	log.Printf("[gRPC Client] Calling remote StoreTEK for organization: %s", req.OrganizationId)

	// Make gRPC call directly with protobuf types
	resp, err := c.client.StoreTEK(ctx, req)
	if err != nil {
		log.Printf("[gRPC Client] StoreTEK failed: %v", err)
		return nil, fmt.Errorf("gRPC store TEK failed: %w", err)
	}

	return resp, nil
}

// RetrieveTEK calls the remote Persistence service to retrieve a TEK
func (c *PersistenceServiceGRPCClient) RetrieveTEK(ctx context.Context, req *pb.RetrieveTEKRequest) (*pb.RetrieveTEKResponse, error) {
	log.Printf("[gRPC Client] Calling remote RetrieveTEK for organization: %s", req.OrganizationId)

	// Make gRPC call directly with protobuf types
	resp, err := c.client.RetrieveTEK(ctx, req)
	if err != nil {
		log.Printf("[gRPC Client] RetrieveTEK failed: %v", err)
		return nil, fmt.Errorf("gRPC retrieve TEK failed: %w", err)
	}

	return resp, nil
}
