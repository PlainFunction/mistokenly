package grpc

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// ClientConfig holds gRPC client configuration
type ClientConfig struct {
	Target     string
	Timeout    time.Duration
	MaxRetries int
	KeepAlive  time.Duration
}

// NewClientConfig creates a default client configuration
func NewClientConfig(host string, port string) *ClientConfig {
	return &ClientConfig{
		Target:     fmt.Sprintf("%s:%s", host, port),
		Timeout:    5 * time.Second,
		MaxRetries: 3,
		KeepAlive:  30 * time.Second,
	}
}

// NewClient creates a new gRPC client connection
func NewClient(config *ClientConfig) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	// gRPC dial options
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                config.KeepAlive,
			Timeout:             config.Timeout,
			PermitWithoutStream: true,
		}),
		grpc.WithUnaryInterceptor(clientLoggingInterceptor),
	}

	conn, err := grpc.DialContext(ctx, config.Target, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %v", config.Target, err)
	}

	return conn, nil
}

// clientLoggingInterceptor logs outgoing gRPC calls
func clientLoggingInterceptor(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	start := time.Now()
	err := invoker(ctx, method, req, reply, cc, opts...)
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("[gRPC Client] %s failed in %v: %v\n", method, duration, err)
	} else {
		fmt.Printf("[gRPC Client] %s completed in %v\n", method, duration)
	}

	return err
}
