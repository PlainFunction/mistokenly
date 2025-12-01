package grpc

import (
	"context"
	"fmt"
	"log"

	pb "github.com/PlainFunction/mistokenly/proto/pii"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// PIIServiceGRPCClient wraps a gRPC client connection to the remote PII service
type PIIServiceGRPCClient struct {
	conn   *grpc.ClientConn
	client pb.PIIServiceClient
}

// NewPIIServiceGRPCClient creates a new gRPC client for the PII service
func NewPIIServiceGRPCClient(serviceAddr string) (*PIIServiceGRPCClient, error) {
	log.Printf("[gRPC Client] Connecting to PII service at %s", serviceAddr)

	// Create gRPC connection
	conn, err := grpc.Dial(serviceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PII service: %w", err)
	}

	// Create gRPC client
	client := pb.NewPIIServiceClient(conn)

	// Test the connection with a health check
	healthReq := &pb.HealthCheckRequest{
		ServiceName: "pii-client-connection-test",
	}
	_, err = client.HealthCheck(context.Background(), healthReq)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to verify PII service connection: %w", err)
	}

	log.Printf("[gRPC Client] Successfully connected to PII service")

	return &PIIServiceGRPCClient{
		conn:   conn,
		client: client,
	}, nil
}

// Close closes the gRPC connection
func (c *PIIServiceGRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Tokenize calls the remote PII service to tokenize data
func (c *PIIServiceGRPCClient) Tokenize(ctx context.Context, req *pb.TokenizeRequest) (*pb.TokenizeResponse, error) {
	log.Printf("[gRPC Client] Calling remote Tokenize for data type: %s", req.DataType)

	// Make gRPC call directly with protobuf types
	resp, err := c.client.Tokenize(ctx, req)
	if err != nil {
		log.Printf("[gRPC Client] Tokenize failed: %v", err)
		return nil, fmt.Errorf("gRPC tokenize failed: %w", err)
	}

	return resp, nil
}

// Detokenize calls the remote PII service to detokenize a token
func (c *PIIServiceGRPCClient) Detokenize(ctx context.Context, req *pb.DetokenizeRequest) (*pb.DetokenizeResponse, error) {
	log.Printf("[gRPC Client] Calling remote Detokenize for hash: %s", req.ReferenceHash)

	// Make gRPC call directly with protobuf types
	resp, err := c.client.Detokenize(ctx, req)
	if err != nil {
		log.Printf("[gRPC Client] Detokenize failed: %v", err)
		return nil, fmt.Errorf("gRPC detokenize failed: %w", err)
	}

	return resp, nil
}

// HealthCheck calls the remote PII service health check
func (c *PIIServiceGRPCClient) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	log.Printf("[gRPC Client] Calling remote HealthCheck")

	// Make gRPC call directly with protobuf types
	resp, err := c.client.HealthCheck(ctx, req)
	if err != nil {
		log.Printf("[gRPC Client] HealthCheck failed: %v", err)
		return nil, fmt.Errorf("gRPC health check failed: %w", err)
	}

	return resp, nil
}
