package grpc

import (
	"context"
	"log"

	"github.com/PlainFunction/mistokenly/internal/services"
	pb "github.com/PlainFunction/mistokenly/proto/pii"
)

// PIIServiceServer wraps the internal PIIService to implement the gRPC server interface
// Now the service works directly with protobuf types, so no conversion needed
type PIIServiceServer struct {
	pb.UnimplementedPIIServiceServer
	service *services.PIIService
}

// NewPIIServiceServer creates a new gRPC server wrapper for PIIService
func NewPIIServiceServer(service *services.PIIService) *PIIServiceServer {
	return &PIIServiceServer{
		service: service,
	}
}

// Tokenize handles the gRPC Tokenize request - now directly passes through
func (s *PIIServiceServer) Tokenize(ctx context.Context, req *pb.TokenizeRequest) (*pb.TokenizeResponse, error) {
	log.Printf("[gRPC Server] Received Tokenize request for data type: %s", req.DataType)
	return s.service.Tokenize(ctx, req)
}

// Detokenize handles the gRPC Detokenize request - now directly passes through
func (s *PIIServiceServer) Detokenize(ctx context.Context, req *pb.DetokenizeRequest) (*pb.DetokenizeResponse, error) {
	log.Printf("[gRPC Server] Received Detokenize request for hash: %s", req.ReferenceHash)
	return s.service.Detokenize(ctx, req)
}

// HealthCheck handles the gRPC HealthCheck request - now directly passes through
func (s *PIIServiceServer) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	log.Printf("[gRPC Server] Received HealthCheck request")
	return s.service.HealthCheck(ctx, req)
}
