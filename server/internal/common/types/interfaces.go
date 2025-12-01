package types

import (
	"context"

	pbAudit "github.com/PlainFunction/mistokenly/proto/audit"
	pbPersistence "github.com/PlainFunction/mistokenly/proto/persistence"
	pbPII "github.com/PlainFunction/mistokenly/proto/pii"
)

// PIIServiceInterface defines the contract for PII operations
type PIIServiceInterface interface {
	Tokenize(ctx context.Context, req *pbPII.TokenizeRequest) (*pbPII.TokenizeResponse, error)
	Detokenize(ctx context.Context, req *pbPII.DetokenizeRequest) (*pbPII.DetokenizeResponse, error)
	HealthCheck(ctx context.Context, req *pbPII.HealthCheckRequest) (*pbPII.HealthCheckResponse, error)
}

// PersistenceServiceInterface defines the contract for persistence operations
type PersistenceServiceInterface interface {
	StorePIIToken(ctx context.Context, req *pbPersistence.StorePIITokenRequest) (*pbPersistence.StorePIITokenResponse, error)
	RetrievePIIToken(ctx context.Context, req *pbPersistence.RetrievePIITokenRequest) (*pbPersistence.RetrievePIITokenResponse, error)
	StoreTEK(ctx context.Context, req *pbPersistence.StoreTEKRequest) (*pbPersistence.StoreTEKResponse, error)
	RetrieveTEK(ctx context.Context, req *pbPersistence.RetrieveTEKRequest) (*pbPersistence.RetrieveTEKResponse, error)
	HealthCheck(ctx context.Context, req *pbPersistence.HealthCheckRequest) (*pbPersistence.HealthCheckResponse, error)
}

// AuditServiceInterface defines the contract for audit operations
type AuditServiceInterface interface {
	LogAccess(ctx context.Context, req *pbAudit.LogAccessRequest) (*pbAudit.LogAccessResponse, error)
	GetAuditLogs(ctx context.Context, req *pbAudit.GetAuditLogsRequest) (*pbAudit.GetAuditLogsResponse, error)
	HealthCheck(ctx context.Context, req *pbAudit.HealthCheckRequest) (*pbAudit.HealthCheckResponse, error)
}
