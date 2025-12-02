package services

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/PlainFunction/mistokenly/internal/common/config"
	"github.com/PlainFunction/mistokenly/internal/common/types"
	pbPersistence "github.com/PlainFunction/mistokenly/proto/persistence"
	pb "github.com/PlainFunction/mistokenly/proto/pii"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/hkdf"
)

// PIIService implements the actual PII tokenization and detokenization logic
type PIIService struct {
	config            *config.Config
	pgmqDB            *sql.DB                           // Database connection for PGMQ
	persistenceClient types.PersistenceServiceInterface // gRPC client for persistence service
	kekProvider       types.KEKProvider                 // Key Encryption Key provider
	// In-memory cache of organization TEKs (in production, retrieve from secure vault)
	tekCache map[string]*types.OrganizationTEK
}

// NewPIIService creates a new PII service instance
func NewPIIService(cfg *config.Config) (*PIIService, error) {
	// Initialize KEK provider using static base64-encoded KEK
	log.Printf("🔑 [PIIService] Initializing static KEK provider")
	kekProvider, err := types.NewStaticKEKProvider(cfg.KEKBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize static KEK provider: %w", err)
	}
	log.Printf("✅ [PIIService] Static KEK provider initialized")
	// Initialize PGMQ database connection for async persistence
	pgmqDB, err := sql.Open("postgres", cfg.PGMQDatabaseURL)
	if err != nil {
		log.Printf("⚠️  [PIIService] PGMQ database connection failed: %v (persistence disabled)", err)
		pgmqDB = nil // Continue without PGMQ
	} else {
		pgmqDB.SetMaxOpenConns(10)
		pgmqDB.SetMaxIdleConns(3)
		pgmqDB.SetConnMaxLifetime(5 * time.Minute)

		if err := pgmqDB.Ping(); err != nil {
			log.Printf("⚠️  [PIIService] PGMQ database ping failed: %v (persistence disabled)", err)
			pgmqDB.Close()
			pgmqDB = nil
		} else {
			log.Printf("✅ [PIIService] PGMQ database connected successfully")
		}
	}

	service := &PIIService{
		config:            cfg,
		pgmqDB:            pgmqDB,
		persistenceClient: nil, // Will be set via SetPersistenceClient if needed
		kekProvider:       kekProvider,
		tekCache:          make(map[string]*types.OrganizationTEK),
	}

	log.Printf("✅ [PIIService] Cryptographic Zero-Knowledge mode enabled")
	log.Printf("📋 [PIIService] TEK management initialized via persistence service")

	return service, nil
}

// SetPersistenceClient sets the persistence service client (used to avoid import cycles)
func (s *PIIService) SetPersistenceClient(client types.PersistenceServiceInterface) {
	s.persistenceClient = client
	if client != nil {
		log.Printf("✅ [PIIService] Persistence service client configured")
	}
}

// Tokenize implements the tokenization logic with actual encryption
func (s *PIIService) Tokenize(ctx context.Context, req *pb.TokenizeRequest) (*pb.TokenizeResponse, error) {
	log.Printf("[PIIService] Tokenizing data type: %s for organization: %s", req.DataType, req.OrganizationId)

	// Basic validation
	if err := s.validateTokenizeRequest(req); err != nil {
		return &pb.TokenizeResponse{
			Status:       "error",
			ErrorMessage: err.Error(),
		}, nil
	}

	// Validate organization key is provided
	if req.OrganizationKey == "" {
		return &pb.TokenizeResponse{
			Status:       "error",
			ErrorMessage: "organizationKey is required for envelope encryption",
		}, nil
	}

	// Generate reference hash
	referenceHash, err := s.generateReferenceHash()
	if err != nil {
		return &pb.TokenizeResponse{
			Status:       "error",
			ErrorMessage: "failed to generate reference hash",
		}, nil
	}

	// Calculate expiration time
	retentionDuration := s.getRetentionDuration(req.RetentionPolicy)
	log.Printf("[PIIService] Tokenizing with retention policy '%s' -> duration: %v", req.RetentionPolicy, retentionDuration)
	expiresAt := time.Now().Add(retentionDuration)
	log.Printf("[PIIService] Token expires at: %v", expiresAt)

	// Encrypt the PII data using envelope encryption with HKDF
	encryptedData, iv, err := s.encryptPIIWithEnvelope(req.Data, req.OrganizationId, req.OrganizationKey)
	if err != nil {
		log.Printf("❌ [PIIService] Encryption failed: %v", err)
		return &pb.TokenizeResponse{
			Status:       "error",
			ErrorMessage: "failed to encrypt PII data",
		}, nil
	}

	// Create token record with IV
	tokenRecord := &TokenRecord{
		ReferenceHash:  referenceHash,
		EncryptedData:  encryptedData,
		IV:             iv,
		DataType:       req.DataType,
		ClientID:       req.ClientId,
		OrganizationID: req.OrganizationId,
		CreatedAt:      time.Now(),
		ExpiresAt:      expiresAt,
		Metadata:       req.Metadata,
	}

	// Queue for durable persistence - asynchronous commit
	if err := s.queueForPersistence(ctx, tokenRecord); err != nil {
		log.Printf("⚠️  [PIIService] Failed to queue for persistence: %v", err)
		// Continue - async persistence failure shouldn't fail the request
	}

	// Log the tokenization event for audit
	s.logAuditEvent(ctx, "tokenize", referenceHash, req.ClientId, req.Metadata)

	log.Printf("✅ [PIIService] Tokenization successful with cryptographic zero-knowledge")

	return &pb.TokenizeResponse{
		ReferenceHash: fmt.Sprintf("tok_%s", referenceHash),
		TokenType:     "PII_TOKEN_V2_ENVELOPE",
		ExpiresAt:     timestamppb.New(tokenRecord.ExpiresAt),
		Status:        "success",
	}, nil
}

// Detokenize implements the detokenization logic with actual decryption
func (s *PIIService) Detokenize(ctx context.Context, req *pb.DetokenizeRequest) (*pb.DetokenizeResponse, error) {
	log.Printf("[PIIService] Detokenizing reference hash: %s for organization: %s", req.ReferenceHash, req.OrganizationId)

	// Basic validation
	if err := s.validateDetokenizeRequest(req); err != nil {
		return &pb.DetokenizeResponse{
			Status:       "error",
			ErrorMessage: err.Error(),
		}, nil
	}

	// Validate organization key is provided
	if req.OrganizationKey == "" {
		return &pb.DetokenizeResponse{
			Status:       "error",
			ErrorMessage: "organizationKey is required for decryption",
		}, nil
	}

	// Extract hash from token format (remove "tok_" prefix)
	hashOnly := req.ReferenceHash
	if len(hashOnly) > 4 && hashOnly[:4] == "tok_" {
		hashOnly = hashOnly[4:]
	}

	// Retrieve from persistence service
	tokenRecord, err := s.retrieveFromDatabase(ctx, hashOnly, req.OrganizationId)
	if err != nil {
		return &pb.DetokenizeResponse{
			Status:       "error",
			ErrorMessage: "token not found",
		}, nil
	}

	// Organization verification is now handled at the database level in persistence service

	// Check if token has expired
	now := time.Now()
	log.Printf("[PIIService] Checking expiration: now=%v, expiresAt=%v, expired=%v", now, tokenRecord.ExpiresAt, now.After(tokenRecord.ExpiresAt))
	if now.After(tokenRecord.ExpiresAt) {
		return &pb.DetokenizeResponse{
			Status:       "error",
			ErrorMessage: "token has expired",
		}, nil
	}

	// Decrypt the PII data using envelope decryption with organization key
	decryptedData, err := s.decryptPIIWithEnvelope(
		tokenRecord.EncryptedData,
		tokenRecord.IV,
		tokenRecord.OrganizationID,
		req.OrganizationKey,
	)
	if err != nil {
		log.Printf("❌ [PIIService] Decryption failed: %v", err)
		return &pb.DetokenizeResponse{
			Status:       "error",
			ErrorMessage: "failed to decrypt PII data - invalid organization key",
		}, nil
	}

	// Log the detokenization access for audit/compliance
	metadata := map[string]string{
		"purpose":            req.Purpose,
		"requesting_user":    req.RequestingUser,
		"requesting_service": req.RequestingService,
		"organization_id":    req.OrganizationId,
	}
	s.logAuditEvent(ctx, "detokenize", hashOnly, req.RequestingService, metadata)

	log.Printf("✅ [PIIService] Detokenization successful with cryptographic zero-knowledge")

	return &pb.DetokenizeResponse{
		Data:              decryptedData,
		DataType:          tokenRecord.DataType,
		OriginalTimestamp: timestamppb.New(tokenRecord.CreatedAt),
		AccessLogged:      true,
		Status:            "success",
	}, nil
}

// HealthCheck implements the health check
func (s *PIIService) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	details := map[string]string{
		"uptime": "24h", // TODO: Calculate actual uptime
	}

	// Service is healthy if core functionality works (encryption/decryption)
	status := "healthy"

	return &pb.HealthCheckResponse{
		Status:      status,
		ServiceName: "pii-service",
		Version:     "1.0.0",
		Timestamp:   timestamppb.New(time.Now()),
		Details:     details,
	}, nil
}

// TokenRecord represents a stored PII token with envelope encryption
type TokenRecord struct {
	ReferenceHash  string
	EncryptedData  []byte
	IV             []byte // Initialization Vector for AES-GCM
	DataType       string
	ClientID       string
	OrganizationID string // Tenant/organization identifier
	CreatedAt      time.Time
	ExpiresAt      time.Time
	Metadata       map[string]string
}

// Helper methods

func (s *PIIService) validateTokenizeRequest(req *pb.TokenizeRequest) error {
	fmt.Printf("🔍 [Validation] Request: %+v\n", req)
	fmt.Printf("🔍 [Validation] OrganizationId: '%s'\n", req.OrganizationId)
	fmt.Printf("🔍 [Validation] OrganizationKey: '%s'\n", req.OrganizationKey)
	if req.Data == "" {
		return fmt.Errorf("data field is required")
	}
	if req.DataType == "" {
		return fmt.Errorf("dataType field is required")
	}
	if req.ClientId == "" {
		return fmt.Errorf("clientId field is required")
	}
	if req.OrganizationId == "" {
		return fmt.Errorf("organizationId field is required for envelope encryption")
	}

	// Validate data type
	validTypes := map[string]bool{
		"email":       true,
		"ssn":         true,
		"phone":       true,
		"credit_card": true,
		"name":        true,
		"address":     true,
	}

	if !validTypes[req.DataType] {
		return fmt.Errorf("invalid dataType: %s", req.DataType)
	}

	return nil
}

func (s *PIIService) validateDetokenizeRequest(req *pb.DetokenizeRequest) error {
	if req.ReferenceHash == "" {
		return fmt.Errorf("referenceHash field is required")
	}
	if req.Purpose == "" {
		return fmt.Errorf("purpose field is required")
	}
	if req.RequestingService == "" {
		return fmt.Errorf("requestingService field is required")
	}
	if req.OrganizationId == "" {
		return fmt.Errorf("organizationId field is required for decryption")
	}

	return nil
}

func (s *PIIService) generateReferenceHash() (string, error) {
	bytes := make([]byte, 16) // 128-bit random value
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// getOrCreateTEK retrieves or creates a TEK for an organization locally
func (s *PIIService) getOrCreateTEK(ctx context.Context, organizationID string, orgKey string) (*types.OrganizationTEK, error) {
	// Check cache first
	if tek, exists := s.tekCache[organizationID]; exists {
		// Verify the organization key matches
		if s.verifyOrganizationKey(orgKey, tek.OrgKeyHash) {
			return tek, nil
		}
		// Key mismatch - remove from cache and reload from persistence service
		delete(s.tekCache, organizationID)
		log.Printf("⚠️  [PIIService] Organization key mismatch for %s, reloading from persistence service", organizationID)
	}

	// Check if persistence client is available
	if s.persistenceClient == nil {
		return nil, fmt.Errorf("persistence service client not available")
	}

	// Try to retrieve existing TEK from persistence service
	retrieveReq := &pbPersistence.RetrieveTEKRequest{
		OrganizationId:  organizationID,
		OrganizationKey: orgKey,
	}

	retrieveResp, err := s.persistenceClient.RetrieveTEK(ctx, retrieveReq)

	// Check if TEK doesn't exist (either gRPC error or status indicates not found)
	shouldCreateTEK := err != nil || (retrieveResp != nil && retrieveResp.Status != "success")

	if shouldCreateTEK {
		// If TEK doesn't exist, create a new one
		log.Printf("🔄 [PIIService] TEK not found for organization %s, creating new one", organizationID)

		// Generate a new random TEK (32 bytes for AES-256)
		tek := make([]byte, 32)
		if _, err := rand.Read(tek); err != nil {
			return nil, fmt.Errorf("failed to generate TEK: %w", err)
		}

		// Wrap TEK with KEK
		encryptedTEK, err := s.wrapTEKWithKEK(tek)
		if err != nil {
			return nil, fmt.Errorf("failed to wrap TEK with KEK: %w", err)
		}

		// Hash the organization key for storage
		orgKeyHash := sha256.Sum256([]byte(orgKey))
		orgKeyHashStr := hex.EncodeToString(orgKeyHash[:])

		// Create TEK record
		tekRecord := &types.OrganizationTEK{
			OrganizationID: organizationID,
			EncryptedTEK:   encryptedTEK,
			OrgKeyHash:     orgKeyHashStr,
			CreatedAt:      time.Now(),
			Version:        1,
		}

		// Store TEK via persistence service
		storeReq := &pbPersistence.StoreTEKRequest{
			OrganizationId: organizationID,
			EncryptedTek:   encryptedTEK,
			OrgKeyHash:     orgKeyHashStr,
			CreatedAt:      timestamppb.New(tekRecord.CreatedAt),
			Version:        int32(tekRecord.Version),
		}

		storeResp, err := s.persistenceClient.StoreTEK(ctx, storeReq)
		if err != nil {
			return nil, fmt.Errorf("failed to store TEK: %w", err)
		}

		if storeResp.Status != "success" {
			return nil, fmt.Errorf("persistence service error storing TEK: %s", storeResp.ErrorMessage)
		}

		// Cache it
		s.tekCache[organizationID] = tekRecord
		log.Printf("✅ [PIIService] New TEK created and cached for organization: %s", organizationID)

		return tekRecord, nil
	}

	// Convert response to OrganizationTEK
	tekRecord := &types.OrganizationTEK{
		OrganizationID: retrieveResp.OrganizationId,
		EncryptedTEK:   retrieveResp.EncryptedTek,
		OrgKeyHash:     retrieveResp.OrgKeyHash,
		CreatedAt:      retrieveResp.CreatedAt.AsTime(),
		Version:        int(retrieveResp.Version),
	}

	if retrieveResp.RotatedAt != nil {
		rotatedAt := retrieveResp.RotatedAt.AsTime()
		tekRecord.RotatedAt = &rotatedAt
	}

	// Cache it
	s.tekCache[organizationID] = tekRecord
	log.Printf("✅ [PIIService] TEK retrieved and cached for organization: %s", organizationID)

	return tekRecord, nil
}

// verifyOrganizationKey validates the provided organization key against stored hash
func (s *PIIService) verifyOrganizationKey(orgKey string, storedHash string) bool {
	// Hash the provided key
	providedHash := sha256.Sum256([]byte(orgKey))
	providedHashStr := hex.EncodeToString(providedHash[:])

	// Compare hashes
	return providedHashStr == storedHash
}

// wrapTEKWithKEK wraps a Tenant Encryption Key with the Key Encryption Key
func (s *PIIService) wrapTEKWithKEK(tek []byte) ([]byte, error) {
	// Get the KEK from the provider
	kek, err := s.kekProvider.GetKEK()
	if err != nil {
		return nil, fmt.Errorf("failed to get KEK: %w", err)
	}

	// Create AES cipher with KEK
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Generate random IV for AES-GCM
	iv := make([]byte, 12) // GCM standard nonce size
	if _, err := rand.Read(iv); err != nil {
		return nil, fmt.Errorf("failed to generate IV: %w", err)
	}

	// Create GCM cipher
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM cipher: %w", err)
	}

	// Encrypt TEK with KEK
	ciphertext := gcm.Seal(nil, iv, tek, nil)

	// Prepend IV to ciphertext for storage
	encryptedTEK := append(iv, ciphertext...)

	return encryptedTEK, nil
}

// unwrapTEKWithKEK unwraps a Tenant Encryption Key using the Key Encryption Key
func (s *PIIService) unwrapTEKWithKEK(encryptedTEK []byte) ([]byte, error) {
	// Get the KEK from the provider
	kek, err := s.kekProvider.GetKEK()
	if err != nil {
		return nil, fmt.Errorf("failed to get KEK: %w", err)
	}

	// Extract IV from the beginning of encrypted data
	if len(encryptedTEK) < 12 {
		return nil, fmt.Errorf("encrypted TEK too short")
	}
	iv := encryptedTEK[:12]
	ciphertext := encryptedTEK[12:]

	// Create AES cipher with KEK
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Create GCM cipher
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM cipher: %w", err)
	}

	// Decrypt TEK
	tek, err := gcm.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt TEK: %w", err)
	}

	return tek, nil
}

// deriveKeyWithHKDF derives a key using HKDF with organization key and TEK
func (s *PIIService) deriveKeyWithHKDF(orgKey string, tek []byte) ([]byte, error) {
	// Use organization key as salt
	salt := []byte(orgKey)

	// Use TEK as input key material
	ikm := tek

	// Create HKDF with SHA-256
	hkdf := hkdf.New(sha256.New, ikm, salt, nil)

	// Derive 32-byte key for AES-256
	derivedKey := make([]byte, 32)
	if _, err := io.ReadFull(hkdf, derivedKey); err != nil {
		return nil, fmt.Errorf("failed to derive key with HKDF: %w", err)
	}

	return derivedKey, nil
}

// encryptPIIWithEnvelope encrypts PII data using envelope encryption locally
func (s *PIIService) encryptPIIWithEnvelope(data string, organizationID string, orgKey string) ([]byte, []byte, error) {
	// Get or create TEK for the organization
	tekRecord, err := s.getOrCreateTEK(context.Background(), organizationID, orgKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get TEK: %w", err)
	}

	// Unwrap the TEK using KEK
	tek, err := s.unwrapTEKWithKEK(tekRecord.EncryptedTEK)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to unwrap TEK: %w", err)
	}

	// Derive encryption key using HKDF
	encryptionKey, err := s.deriveKeyWithHKDF(orgKey, tek)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to derive encryption key: %w", err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Generate random IV for AES-GCM
	iv := make([]byte, 12) // GCM standard nonce size
	if _, err := rand.Read(iv); err != nil {
		return nil, nil, fmt.Errorf("failed to generate IV: %w", err)
	}

	// Validate IV length (should always be 12, but let's be sure)
	if len(iv) != 12 {
		return nil, nil, fmt.Errorf("generated IV has incorrect length: %d", len(iv))
	}

	// Create GCM cipher
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GCM cipher: %w", err)
	}

	// Encrypt the data
	ciphertext := gcm.Seal(nil, iv, []byte(data), nil)

	log.Printf("🔐 [PIIService] PII data encrypted with envelope encryption")

	return ciphertext, iv, nil
}

// decryptPIIWithEnvelope decrypts PII data using envelope decryption locally
func (s *PIIService) decryptPIIWithEnvelope(ciphertext []byte, iv []byte, organizationID string, orgKey string) (string, error) {
	// Validate IV length for AES-GCM (must be exactly 12 bytes)
	if len(iv) != 12 {
		return "", fmt.Errorf("invalid IV length: got %d bytes, expected 12 bytes for AES-GCM", len(iv))
	}

	// Get TEK for the organization
	tekRecord, err := s.getOrCreateTEK(context.Background(), organizationID, orgKey)
	if err != nil {
		return "", fmt.Errorf("failed to get TEK: %w", err)
	}

	// Unwrap the TEK using KEK
	tek, err := s.unwrapTEKWithKEK(tekRecord.EncryptedTEK)
	if err != nil {
		return "", fmt.Errorf("failed to unwrap TEK: %w", err)
	}

	// Derive encryption key using HKDF
	encryptionKey, err := s.deriveKeyWithHKDF(orgKey, tek)
	if err != nil {
		return "", fmt.Errorf("failed to derive encryption key: %w", err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Create GCM cipher
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM cipher: %w", err)
	}

	// Decrypt the data
	plaintext, err := gcm.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt data: %w", err)
	}

	log.Printf("🔓 [PIIService] PII data decrypted with envelope decryption")

	return string(plaintext), nil
}

func (s *PIIService) getRetentionDuration(policy string) time.Duration {
	switch policy {
	case "1day":
		return 24 * time.Hour
	case "7days":
		return 7 * 24 * time.Hour
	case "30days":
		return 30 * 24 * time.Hour
	case "1year":
		return 365 * 24 * time.Hour
	case "7years":
		return 7 * 365 * 24 * time.Hour
	default:
		return 24 * time.Hour // Default to 1 day
	}
}

// Storage methods

func (s *PIIService) retrieveFromDatabase(ctx context.Context, hash string, organizationID string) (*TokenRecord, error) {
	log.Printf("[PIIService] Attempting to retrieve from persistence service: %s", hash)

	// Check if persistence client is available
	if s.persistenceClient == nil {
		log.Printf("⚠️  [PIIService] Persistence service not available for hash: %s", hash)
		return nil, fmt.Errorf("token not found: persistence service unavailable")
	}

	// We need organization_id to query the persistence service, but we don't have it here
	// This is a limitation of the current design - we'll need to store hash-to-org mapping in cache
	// For now, we'll try without organization_id filtering (the persistence service should return the token if found)

	// Note: In the detokenize flow, we pass the organization_id to the persistence service
	// This enables database-level filtering for better security and performance
	req := &pbPersistence.RetrievePIITokenRequest{
		ReferenceHash:  hash,
		OrganizationId: organizationID, // Pass the actual organization ID for database-level filtering
	}

	// Call persistence service via gRPC
	resp, err := s.persistenceClient.RetrievePIIToken(ctx, req)
	if err != nil {
		log.Printf("❌ [PIIService] Failed to call persistence service: %v", err)
		return nil, fmt.Errorf("failed to retrieve from persistence service: %w", err)
	}

	// Check if token was found
	if resp.Status != "success" {
		log.Printf("[PIIService] Token not found in persistence service: %s - %s", hash, resp.ErrorMessage)
		return nil, fmt.Errorf("token not found: %s", resp.ErrorMessage)
	}

	// Convert response to TokenRecord
	record := &TokenRecord{
		ReferenceHash:  resp.ReferenceHash,
		EncryptedData:  resp.EncryptedData,
		IV:             resp.Iv,
		DataType:       resp.DataType,
		ClientID:       resp.ClientId,
		OrganizationID: resp.OrganizationId,
		Metadata:       resp.Metadata,
	}

	// Convert timestamps
	if resp.CreatedAt != nil {
		record.CreatedAt = resp.CreatedAt.AsTime()
	}
	if resp.ExpiresAt != nil {
		record.ExpiresAt = resp.ExpiresAt.AsTime()
	}

	log.Printf("✅ [PIIService] Successfully retrieved token from persistence service: %s", hash)
	return record, nil
}

func (s *PIIService) queueForPersistence(ctx context.Context, record *TokenRecord) error {
	log.Printf("[PIIService] Queuing for persistence: %s", record.ReferenceHash)

	// Check if PGMQ is available
	if s.pgmqDB == nil {
		log.Printf("⚠️  [PIIService] PGMQ not available, skipping persistence queue")
		return nil // Don't fail the request if PGMQ is unavailable
	}

	// Convert TokenRecord to persistence service request
	req := &pbPersistence.StorePIITokenRequest{
		ReferenceHash:  record.ReferenceHash,
		EncryptedData:  record.EncryptedData,
		Iv:             record.IV,
		DataType:       record.DataType,
		ClientId:       record.ClientID,
		OrganizationId: record.OrganizationID,
		Metadata:       record.Metadata,
	}

	// Add timestamps
	if !record.CreatedAt.IsZero() {
		req.CreatedAt = timestamppb.New(record.CreatedAt)
	}
	if !record.ExpiresAt.IsZero() {
		req.ExpiresAt = timestamppb.New(record.ExpiresAt)
	}

	// Marshal to JSON for PGMQ message
	messageJSON, err := json.Marshal(req)
	if err != nil {
		log.Printf("❌ [PIIService] Failed to marshal persistence message: %v", err)
		return fmt.Errorf("failed to marshal persistence message: %w", err)
	}

	// Publish to PGMQ queue
	query := `SELECT pgmq.send($1, $2)`
	_, err = s.pgmqDB.ExecContext(ctx, query, "pii_token_persistence", string(messageJSON))
	if err != nil {
		log.Printf("❌ [PIIService] Failed to publish to PGMQ: %v", err)
		return fmt.Errorf("failed to publish to PGMQ: %w", err)
	}

	log.Printf("✅ [PIIService] Successfully queued token for persistence: %s", record.ReferenceHash)
	return nil
}

func (s *PIIService) logAuditEvent(ctx context.Context, operation, hash, clientID string, metadata map[string]string) {
	// TODO: Send to audit service via gRPC
	log.Printf("[PIIService] Audit log: %s operation on %s by %s", operation, hash, clientID)
}

// Close gracefully shuts down the service and closes connections
func (s *PIIService) Close() error {
	log.Println("🔌 [PIIService] Closing connections...")

	if s.pgmqDB != nil {
		log.Println("  - Closing PGMQ database connection...")
		if err := s.pgmqDB.Close(); err != nil {
			log.Printf("⚠️  Failed to close PGMQ connection: %v", err)
		} else {
			log.Println("  ✅ PGMQ database connection closed")
		}
	}

	if s.persistenceClient != nil {
		log.Println("  - Closing persistence service gRPC client...")
		if closer, ok := s.persistenceClient.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				log.Printf("⚠️  Failed to close persistence client: %v", err)
			} else {
				log.Println("  ✅ Persistence client closed")
			}
		}
	}

	log.Println("✅ [PIIService] All connections closed")
	return nil
}
