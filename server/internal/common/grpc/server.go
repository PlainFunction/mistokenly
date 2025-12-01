package grpc

import (
	"context"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/PlainFunction/mistokenly/internal/common/config"
)

// Server wraps the gRPC server with common functionality
type Server struct {
	grpcServer *grpc.Server
	listener   net.Listener
	config     *config.Config
}

// NewServer creates a new gRPC server instance
func NewServer(cfg *config.Config, serviceName string) (*Server, error) {
	// Create listener on the specified port
	port := cfg.GetGRPCPort(serviceName)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %s: %v", port, err)
	}

	// Create gRPC server with interceptors
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(loggingInterceptor),
		grpc.StreamInterceptor(streamLoggingInterceptor),
	)

	// Enable reflection for development
	if cfg.Environment == "development" {
		reflection.Register(grpcServer)
	}

	return &Server{
		grpcServer: grpcServer,
		listener:   lis,
		config:     cfg,
	}, nil
}

// RegisterService registers a gRPC service with the server
func (s *Server) RegisterService(registerFunc func(*grpc.Server)) {
	registerFunc(s.grpcServer)
}

// Start starts the gRPC server
func (s *Server) Start() error {
	log.Printf("Starting gRPC server on %s", s.listener.Addr().String())
	return s.grpcServer.Serve(s.listener)
}

// Stop gracefully stops the gRPC server
func (s *Server) Stop() {
	log.Println("Stopping gRPC server...")
	s.grpcServer.GracefulStop()
}

// Interceptors for logging and monitoring
func loggingInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	log.Printf("gRPC call: %s", info.FullMethod)
	resp, err := handler(ctx, req)
	if err != nil {
		log.Printf("gRPC error: %s - %v", info.FullMethod, err)
	}
	return resp, err
}

func streamLoggingInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	log.Printf("gRPC stream: %s", info.FullMethod)
	err := handler(srv, ss)
	if err != nil {
		log.Printf("gRPC stream error: %s - %v", info.FullMethod, err)
	}
	return err
}
