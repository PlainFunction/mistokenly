package types

import (
	"encoding/base64"
	"fmt"
	"log"
	"time"
)

// KEKProvider defines the interface for KEK management
type KEKProvider interface {
	GetKEK() ([]byte, error)
	Close() error
}

// StaticKEKProvider implements KEKProvider using a static base64-encoded KEK
type StaticKEKProvider struct {
	kek []byte
}

// NewStaticKEKProvider creates a new static KEK provider from base64-encoded KEK
func NewStaticKEKProvider(kekBase64 string) (*StaticKEKProvider, error) {
	if kekBase64 == "" {
		return nil, fmt.Errorf("KEK_BASE64 environment variable is required")
	}

	kek, err := base64.StdEncoding.DecodeString(kekBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode KEK_BASE64: %w", err)
	}

	if len(kek) != 32 {
		return nil, fmt.Errorf("KEK must be 32 bytes (256 bits), got %d bytes", len(kek))
	}

	log.Printf("✅ [KEKProvider] Static KEK loaded from environment (length: %d bytes)", len(kek))

	return &StaticKEKProvider{
		kek: kek,
	}, nil
}

// GetKEK returns the static KEK
func (p *StaticKEKProvider) GetKEK() ([]byte, error) {
	// Return a copy to prevent external modification
	kekCopy := make([]byte, len(p.kek))
	copy(kekCopy, p.kek)
	return kekCopy, nil
}

// Close is a no-op for static provider
func (p *StaticKEKProvider) Close() error {
	return nil
}

// OrganizationTEK represents a Tenant Encryption Key for an organization
type OrganizationTEK struct {
	OrganizationID string
	EncryptedTEK   []byte // TEK encrypted by KEK
	OrgKeyHash     string // SHA-256 hash of organization key for verification
	CreatedAt      time.Time
	RotatedAt      *time.Time // Nullable - only set when key is rotated
	Version        int
}

// Message structs (simplified versions of protobuf messages)

type TokenizeRequest struct {
	Data            string
	DataType        string
	RetentionPolicy string
	ClientID        string
	OrganizationID  string // Organization/tenant identifier
	OrganizationKey string // Client-provided secret for envelope encryption (ephemeral)
	Metadata        map[string]string
}

type TokenizeResponse struct {
	ReferenceHash string
	TokenType     string
	ExpiresAt     time.Time
	Status        string
	ErrorMessage  string
}

type DetokenizeRequest struct {
	ReferenceHash     string
	Purpose           string
	RequestingService string
	RequestingUser    string
	OrganizationID    string // Organization/tenant identifier
	OrganizationKey   string // Client-provided secret for decryption (ephemeral)
}

type DetokenizeResponse struct {
	Data              string
	DataType          string
	OriginalTimestamp time.Time
	AccessLogged      bool
	Status            string
	ErrorMessage      string
}

type HealthCheckRequest struct {
	ServiceName string
}

type HealthCheckResponse struct {
	Status      string
	ServiceName string
	Version     string
	Timestamp   time.Time
	Details     map[string]string
}

// Audit service message types
type LogAccessRequest struct {
	ReferenceHash     string
	Operation         string // "tokenize" or "detokenize"
	RequestingService string
	RequestingUser    string
	Purpose           string
	Timestamp         time.Time
	ClientIP          string
	Metadata          map[string]string
}

type LogAccessResponse struct {
	AuditID      string
	Status       string
	ErrorMessage string
}

type GetAuditLogsRequest struct {
	StartTime         time.Time
	EndTime           time.Time
	ReferenceHash     string
	RequestingService string
	Limit             int32
	Offset            int32
}

type AuditLogEntry struct {
	AuditID           string
	ReferenceHash     string
	Operation         string
	RequestingService string
	RequestingUser    string
	Purpose           string
	Timestamp         time.Time
	ClientIP          string
	Metadata          map[string]string
}

type GetAuditLogsResponse struct {
	Logs         []*AuditLogEntry
	TotalCount   int32
	Status       string
	ErrorMessage string
}

type ComplianceReportRequest struct {
	StartTime  time.Time
	EndTime    time.Time
	ReportType string // "gdpr", "hipaa", "pci"
}

type ComplianceReportResponse struct {
	ReportID     string
	ReportType   string
	GeneratedAt  time.Time
	ReportData   []byte // JSON or PDF data
	Status       string
	ErrorMessage string
}

// Persistence service message types
type StorePIIRequest struct {
	ReferenceHash   string
	EncryptedData   []byte
	DataType        string
	EncryptionKeyID string
	ExpiresAt       time.Time
	Metadata        map[string]string
}

type StorePIIResponse struct {
	ReferenceHash string
	Status        string
	ErrorMessage  string
}

type RetrievePIIRequest struct {
	ReferenceHash string
}

type RetrievePIIResponse struct {
	ReferenceHash   string
	EncryptedData   []byte
	DataType        string
	EncryptionKeyID string
	CreatedAt       time.Time
	ExpiresAt       time.Time
	Metadata        map[string]string
	Status          string
	ErrorMessage    string
}
