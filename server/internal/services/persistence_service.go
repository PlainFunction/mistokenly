package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/PlainFunction/mistokenly/internal/common/config"
	"github.com/PlainFunction/mistokenly/internal/common/types"
	pb "github.com/PlainFunction/mistokenly/proto/persistence"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type PersistenceService struct {
	pb.UnimplementedPersistenceServiceServer
	config      *config.Config
	db          *sql.DB
	pgmqDB      *sql.DB
	redisClient *redis.Client
	stopCh      chan struct{}
}

func NewPersistenceService(cfg *config.Config) (*PersistenceService, error) {
	log.Println("[Persistence] Initializing Persistence Service")

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to storage database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping storage database: %w", err)
	}
	log.Println("[Persistence] Connected to storage database")

	pgmqDB, err := sql.Open("postgres", cfg.PGMQDatabaseURL)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to PGMQ database: %w", err)
	}

	pgmqDB.SetMaxOpenConns(10)
	pgmqDB.SetMaxIdleConns(3)
	pgmqDB.SetConnMaxLifetime(5 * time.Minute)

	if err := pgmqDB.Ping(); err != nil {
		db.Close()
		pgmqDB.Close()
		return nil, fmt.Errorf("failed to ping PGMQ database: %w", err)
	}
	log.Println("[Persistence] Connected to PGMQ database")

	// Initialize Redis client for caching
	var redisClient *redis.Client
	if cfg.CacheEnabled {
		redisAddr := fmt.Sprintf("%s:%s", cfg.CacheHost, cfg.CachePort)
		redisClient = redis.NewClient(&redis.Options{
			Addr:         redisAddr,
			Password:     "", // No password by default
			DB:           0,  // Use default DB
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
			PoolSize:     10,
			MinIdleConns: 5,
		})

		// Test Redis connection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := redisClient.Ping(ctx).Err(); err != nil {
			log.Printf("⚠️  [Persistence] Cache connection failed: %v (continuing without cache)", err)
			// Don't fail service startup if Redis is unavailable
			redisClient = nil
		} else {
			log.Printf("✅ [Persistence] Cache connected successfully at %s", redisAddr)
		}
	} else {
		log.Printf("ℹ️ [Persistence] Cache disabled via configuration")
		redisClient = nil
	}

	return &PersistenceService{
		config:      cfg,
		db:          db,
		pgmqDB:      pgmqDB,
		redisClient: redisClient,
		stopCh:      make(chan struct{}),
	}, nil
}

func (s *PersistenceService) Close() error {
	log.Println("[Persistence] Closing database connections")
	close(s.stopCh)

	var storageErr, pgmqErr, redisErr error
	if s.db != nil {
		storageErr = s.db.Close()
	}
	if s.pgmqDB != nil {
		pgmqErr = s.pgmqDB.Close()
	}
	if s.redisClient != nil {
		redisErr = s.redisClient.Close()
	}

	if storageErr != nil {
		return storageErr
	}
	if pgmqErr != nil {
		return pgmqErr
	}
	return redisErr
}

// StartWorkers starts multiple PGMQ workers to process persistence messages
func (s *PersistenceService) StartWorkers(numWorkers int) {
	log.Printf("[Persistence] Starting %d PGMQ workers", numWorkers)
	for i := 1; i <= numWorkers; i++ {
		go s.worker(i)
	}
}

// worker is a background goroutine that continuously polls PGMQ for messages
func (s *PersistenceService) worker(workerID int) {
	log.Printf("[Worker %d] Started", workerID)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			log.Printf("[Worker %d] Shutting down", workerID)
			return
		case <-ticker.C:
			s.processMessages(workerID)
		}
	}
}

// processMessages reads messages from PGMQ, stores them in the database, and deletes them from the queue
func (s *PersistenceService) processMessages(workerID int) {
	ctx := context.Background()

	query := `SELECT msg_id, message FROM pgmq.read('pii_token_persistence', 300, 10)`
	rows, err := s.pgmqDB.QueryContext(ctx, query)
	if err != nil {
		log.Printf("[Worker %d] Failed to read from PGMQ: %v", workerID, err)
		return
	}
	defer rows.Close()

	messagesProcessed := 0
	for rows.Next() {
		var msgID int64
		var messageJSON []byte

		if err := rows.Scan(&msgID, &messageJSON); err != nil {
			log.Printf("[Worker %d] Failed to scan message: %v", workerID, err)
			continue
		}

		var req pb.StorePIITokenRequest
		if err := json.Unmarshal(messageJSON, &req); err != nil {
			log.Printf("[Worker %d] Failed to unmarshal message %d: %v", workerID, msgID, err)
			s.deleteMessage(ctx, msgID)
			continue
		}

		if err := s.storePIIToken(ctx, &req); err != nil {
			log.Printf("[Worker %d] Failed to store token %s: %v", workerID, req.ReferenceHash, err)
			continue
		}

		if err := s.deleteMessage(ctx, msgID); err != nil {
			log.Printf("[Worker %d] Failed to delete message %d: %v", workerID, msgID, err)
		} else {
			messagesProcessed++
		}
	}

	if messagesProcessed > 0 {
		log.Printf("[Worker %d] Processed %d messages", workerID, messagesProcessed)
	}
}

// deleteMessage removes a message from the PGMQ queue
func (s *PersistenceService) deleteMessage(ctx context.Context, msgID int64) error {
	query := `SELECT pgmq.delete('pii_token_persistence'::text, $1::bigint)`
	_, err := s.pgmqDB.ExecContext(ctx, query, msgID)
	return err
}

// storePIIToken inserts or updates a PII token in the persistent database
func (s *PersistenceService) storePIIToken(ctx context.Context, req *pb.StorePIITokenRequest) error {
	log.Printf("[Persistence] Storing token: %s (org: %s)", req.ReferenceHash, req.OrganizationId)

	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t := req.ExpiresAt.AsTime()
		expiresAt = &t
	}

	var metadataJSON []byte
	var err error
	if len(req.Metadata) > 0 {
		metadataJSON, err = json.Marshal(req.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	} else {
		// Use empty JSON object for NULL/empty metadata
		metadataJSON = []byte("{}")
	}

	query := `
		INSERT INTO pii_tokens (reference_hash, encrypted_data, iv, data_type, client_id, organization_id, expires_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (reference_hash) 
		DO UPDATE SET
			encrypted_data = EXCLUDED.encrypted_data,
			iv = EXCLUDED.iv,
			data_type = EXCLUDED.data_type,
			client_id = EXCLUDED.client_id,
			expires_at = EXCLUDED.expires_at,
			metadata = EXCLUDED.metadata,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err = s.db.ExecContext(ctx, query,
		req.ReferenceHash,
		req.EncryptedData,
		req.Iv,
		req.DataType,
		req.ClientId,
		req.OrganizationId,
		expiresAt,
		metadataJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to insert token: %w", err)
	}

	// Cache the token after successful storage
	if s.redisClient != nil {
		if err := s.cacheToken(ctx, req, expiresAt); err != nil {
			log.Printf("⚠️  [Persistence] Failed to cache token %s: %v", req.ReferenceHash, err)
			// Don't fail the operation if caching fails
		} else {
			log.Printf("✅ [Persistence] Token cached: %s", req.ReferenceHash)
		}
	}

	log.Printf("[Persistence] Successfully stored token: %s", req.ReferenceHash)
	return nil
}

// cacheToken stores a token in Redis cache with TTL
func (s *PersistenceService) cacheToken(ctx context.Context, req *pb.StorePIITokenRequest, expiresAt *time.Time) error {
	if s.redisClient == nil {
		return fmt.Errorf("redis client not available")
	}

	// Create cache entry with proper base64 encoding for byte slices
	cacheEntry := map[string]interface{}{
		"reference_hash":  req.ReferenceHash,
		"encrypted_data":  req.EncryptedData, // []byte will be base64 encoded by json.Marshal
		"iv":              req.Iv,            // []byte will be base64 encoded by json.Marshal
		"data_type":       req.DataType,
		"client_id":       req.ClientId,
		"organization_id": req.OrganizationId,
		"metadata":        req.Metadata,
	}

	if req.CreatedAt != nil {
		cacheEntry["created_at"] = req.CreatedAt.AsTime().Unix()
	}
	if expiresAt != nil {
		cacheEntry["expires_at"] = expiresAt.Unix()
	}

	// Serialize to JSON
	data, err := json.Marshal(cacheEntry)
	if err != nil {
		return fmt.Errorf("failed to serialize cache entry: %w", err)
	}

	// Create cache key
	cacheKey := fmt.Sprintf("pii:token:%s", req.ReferenceHash)

	// Calculate TTL based on expiration time
	var ttl time.Duration
	if expiresAt != nil {
		ttl = time.Until(*expiresAt)
		if ttl <= 0 {
			return fmt.Errorf("token already expired")
		}
	} else {
		// Default TTL if no expiration
		ttl = 24 * time.Hour
	}

	// Store in Redis with TTL
	err = s.redisClient.Set(ctx, cacheKey, data, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to store in redis: %w", err)
	}

	return nil
}

// StorePIIToken is the gRPC endpoint for directly storing a PII token
func (s *PersistenceService) StorePIIToken(ctx context.Context, req *pb.StorePIITokenRequest) (*pb.StorePIITokenResponse, error) {
	log.Printf("[gRPC] StorePIIToken called for token: %s", req.ReferenceHash)

	if err := s.storePIIToken(ctx, req); err != nil {
		return &pb.StorePIITokenResponse{
			ReferenceHash: req.ReferenceHash,
			Status:        "error",
			ErrorMessage:  fmt.Sprintf("Failed to store token: %v", err),
		}, nil
	}

	return &pb.StorePIITokenResponse{
		ReferenceHash: req.ReferenceHash,
		Status:        "success",
		ErrorMessage:  "",
	}, nil
}

// RetrievePIIToken is the gRPC endpoint for retrieving a PII token from persistent storage
func (s *PersistenceService) RetrievePIIToken(ctx context.Context, req *pb.RetrievePIITokenRequest) (*pb.RetrievePIITokenResponse, error) {
	log.Printf("[gRPC] RetrievePIIToken called for hash: %s (org: %s)", req.ReferenceHash, req.OrganizationId)

	// Try cache first
	if s.redisClient != nil {
		if cachedResponse, err := s.retrieveFromCache(ctx, req.ReferenceHash, req.OrganizationId); err == nil {
			log.Printf("✅ [Persistence] Cache hit for token: %s", req.ReferenceHash)
			return cachedResponse, nil
		} else if err.Error() != "cache miss" {
			log.Printf("⚠️  [Persistence] Cache error for token %s: %v", req.ReferenceHash, err)
		} else {
			log.Printf("🔍 [Persistence] Cache miss for token: %s", req.ReferenceHash)
		}
	}

	// Fall back to database
	return s.retrieveFromDatabase(ctx, req)
}

// retrieveFromCache attempts to retrieve a token from Redis cache
func (s *PersistenceService) retrieveFromCache(ctx context.Context, hash string, organizationID string) (*pb.RetrievePIITokenResponse, error) {
	if s.redisClient == nil {
		return nil, fmt.Errorf("redis client not available")
	}

	// Create cache key
	cacheKey := fmt.Sprintf("pii:token:%s", hash)

	// Get from Redis
	data, err := s.redisClient.Get(ctx, cacheKey).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("cache miss")
	} else if err != nil {
		return nil, fmt.Errorf("redis error: %w", err)
	}

	// Deserialize the cache entry
	var cacheEntry map[string]interface{}
	if err := json.Unmarshal(data, &cacheEntry); err != nil {
		return nil, fmt.Errorf("failed to deserialize cache entry: %w", err)
	}

	// Verify organization matches
	if orgID, ok := cacheEntry["organization_id"].(string); !ok || orgID != organizationID {
		return nil, fmt.Errorf("organization mismatch in cache")
	}

	// Check if token has expired
	if expiresAtUnix, ok := cacheEntry["expires_at"].(float64); ok {
		expiresAt := time.Unix(int64(expiresAtUnix), 0)
		if time.Now().After(expiresAt) {
			// Token expired, remove from cache
			s.redisClient.Del(ctx, cacheKey)
			return nil, fmt.Errorf("token expired")
		}
	}

	// Build response
	response := &pb.RetrievePIITokenResponse{
		ReferenceHash:  hash,
		OrganizationId: organizationID,
		Status:         "success",
		ErrorMessage:   "",
	}

	// Extract fields from cache entry with proper type assertions
	if encryptedDataStr, ok := cacheEntry["encrypted_data"].(string); ok {
		// Decode base64 string back to []byte
		if data, err := base64.StdEncoding.DecodeString(encryptedDataStr); err == nil {
			response.EncryptedData = data
		} else {
			log.Printf("⚠️  [Persistence] Failed to decode encrypted_data from cache: %v", err)
			s.redisClient.Del(ctx, cacheKey)
			return nil, fmt.Errorf("corrupted cache entry: invalid encrypted_data")
		}
	}
	if ivStr, ok := cacheEntry["iv"].(string); ok {
		// Decode base64 string back to []byte
		if iv, err := base64.StdEncoding.DecodeString(ivStr); err == nil {
			// Validate IV length for AES-GCM (must be 12 bytes)
			if len(iv) != 12 {
				log.Printf("⚠️  [Persistence] Invalid IV length in cache: %d bytes, expected 12", len(iv))
				// Remove corrupted cache entry
				s.redisClient.Del(ctx, cacheKey)
				return nil, fmt.Errorf("corrupted cache entry: invalid IV length")
			}
			response.Iv = iv
		} else {
			log.Printf("⚠️  [Persistence] Failed to decode IV from cache: %v", err)
			s.redisClient.Del(ctx, cacheKey)
			return nil, fmt.Errorf("corrupted cache entry: invalid IV")
		}
	}
	if dataType, ok := cacheEntry["data_type"].(string); ok {
		response.DataType = dataType
	}
	if clientId, ok := cacheEntry["client_id"].(string); ok {
		response.ClientId = clientId
	}
	if metadata, ok := cacheEntry["metadata"].(map[string]interface{}); ok {
		// Convert map[string]interface{} to map[string]string
		stringMetadata := make(map[string]string)
		for k, v := range metadata {
			if str, ok := v.(string); ok {
				stringMetadata[k] = str
			}
		}
		response.Metadata = stringMetadata
	}

	// Handle timestamps
	if createdAtUnix, ok := cacheEntry["created_at"].(float64); ok {
		response.CreatedAt = timestamppb.New(time.Unix(int64(createdAtUnix), 0))
	}
	if expiresAtUnix, ok := cacheEntry["expires_at"].(float64); ok {
		response.ExpiresAt = timestamppb.New(time.Unix(int64(expiresAtUnix), 0))
	}

	return response, nil
}

// retrieveFromDatabase retrieves a token from the persistent database
func (s *PersistenceService) retrieveFromDatabase(ctx context.Context, req *pb.RetrievePIITokenRequest) (*pb.RetrievePIITokenResponse, error) {
	query := `
		SELECT encrypted_data, iv, data_type, client_id, created_at, metadata, expires_at
		FROM pii_tokens
		WHERE reference_hash = $1 AND organization_id = $2
	`

	var encryptedData, iv []byte
	var dataType, clientId string
	var createdAt, expiresAt *time.Time
	var metadataJSON []byte

	err := s.db.QueryRowContext(ctx, query, req.ReferenceHash, req.OrganizationId).Scan(
		&encryptedData, &iv, &dataType, &clientId, &createdAt, &metadataJSON, &expiresAt,
	)
	if err == sql.ErrNoRows {
		log.Printf("[Persistence] Token not found: %s for org: %s", req.ReferenceHash, req.OrganizationId)
		return &pb.RetrievePIITokenResponse{
			ReferenceHash: req.ReferenceHash,
			Status:        "error",
			ErrorMessage:  "Token not found in persistent storage",
		}, nil
	}
	if err != nil {
		log.Printf("[Persistence] Database error retrieving token: %v", err)
		return &pb.RetrievePIITokenResponse{
			ReferenceHash: req.ReferenceHash,
			Status:        "error",
			ErrorMessage:  fmt.Sprintf("Database error: %v", err),
		}, nil
	}

	if expiresAt != nil && expiresAt.Before(time.Now()) {
		log.Printf("[Persistence] Token expired: %s", req.ReferenceHash)
		return &pb.RetrievePIITokenResponse{
			ReferenceHash: req.ReferenceHash,
			Status:        "error",
			ErrorMessage:  "Token has expired",
		}, nil
	}

	var metadata map[string]string
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
			log.Printf("[Persistence] Failed to unmarshal metadata: %v", err)
		}
	}

	log.Printf("[Persistence] Token found: %s", req.ReferenceHash)
	response := &pb.RetrievePIITokenResponse{
		ReferenceHash:  req.ReferenceHash,
		EncryptedData:  encryptedData,
		Iv:             iv,
		DataType:       dataType,
		ClientId:       clientId,
		OrganizationId: req.OrganizationId,
		Metadata:       metadata,
		Status:         "success",
		ErrorMessage:   "",
	}

	if createdAt != nil {
		response.CreatedAt = timestamppb.New(*createdAt)
	}
	if expiresAt != nil {
		response.ExpiresAt = timestamppb.New(*expiresAt)
	}

	return response, nil
}

// HealthCheck is the gRPC endpoint for checking the health of the persistence service
func (s *PersistenceService) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	log.Printf("[gRPC] HealthCheck called from: %s", req.ServiceName)

	checks := make(map[string]string)

	if err := s.db.Ping(); err != nil {
		checks["persistent_db"] = fmt.Sprintf("unhealthy: %v", err)
	} else {
		checks["persistent_db"] = "healthy"
	}

	if err := s.pgmqDB.Ping(); err != nil {
		checks["pgmq_db"] = fmt.Sprintf("unhealthy: %v", err)
	} else {
		checks["pgmq_db"] = "healthy"
	}

	if s.redisClient != nil {
		if err := s.redisClient.Ping(ctx).Err(); err != nil {
			checks["cache"] = fmt.Sprintf("unhealthy: %v", err)
		} else {
			checks["cache"] = "healthy"
		}
	} else {
		checks["cache"] = "disabled"
	}

	var queueDepth int
	query := `SELECT COUNT(*) FROM pgmq.q_pii_token_persistence`
	if err := s.pgmqDB.QueryRowContext(ctx, query).Scan(&queueDepth); err != nil {
		checks["queue_depth"] = fmt.Sprintf("error: %v", err)
	} else {
		checks["queue_depth"] = fmt.Sprintf("%d messages", queueDepth)
	}

	status := "healthy"
	for _, checkStatus := range checks {
		if checkStatus != "healthy" && !containsMessages(checkStatus) {
			status = "unhealthy"
			break
		}
	}

	return &pb.HealthCheckResponse{
		Status:      status,
		ServiceName: "persistence",
		Version:     "1.0.0",
		Details:     checks,
	}, nil
}

// StoreTEK stores a Tenant Encryption Key for an organization
func (s *PersistenceService) StoreTEK(ctx context.Context, req *pb.StoreTEKRequest) (*pb.StoreTEKResponse, error) {
	log.Printf("[gRPC] StoreTEK called for organization: %s", req.OrganizationId)

	tekRecord := &types.OrganizationTEK{
		OrganizationID: req.OrganizationId,
		EncryptedTEK:   req.EncryptedTek,
		OrgKeyHash:     req.OrgKeyHash,
		CreatedAt:      req.CreatedAt.AsTime(),
		Version:        int(req.Version),
	}

	if req.RotatedAt != nil {
		rotatedAt := req.RotatedAt.AsTime()
		tekRecord.RotatedAt = &rotatedAt
	}

	// Persist to database
	if err := s.saveTEKToDatabase(ctx, tekRecord); err != nil {
		return &pb.StoreTEKResponse{
			Status:       "error",
			ErrorMessage: fmt.Sprintf("failed to save TEK to database: %v", err),
		}, nil
	}

	log.Printf("[Persistence] TEK stored for organization: %s", req.OrganizationId)

	return &pb.StoreTEKResponse{
		OrganizationId: req.OrganizationId,
		Status:         "success",
	}, nil
}

// RetrieveTEK retrieves a Tenant Encryption Key for an organization
func (s *PersistenceService) RetrieveTEK(ctx context.Context, req *pb.RetrieveTEKRequest) (*pb.RetrieveTEKResponse, error) {
	log.Printf("[gRPC] RetrieveTEK called for organization: %s", req.OrganizationId)

	// Load TEK from database
	tekRecord, err := s.loadTEKFromDatabase(ctx, req.OrganizationId, req.OrganizationKey)
	if err != nil {
		if err == sql.ErrNoRows {
			return &pb.RetrieveTEKResponse{
				Status:       "error",
				ErrorMessage: "TEK not found for organization",
			}, nil
		}
		return &pb.RetrieveTEKResponse{
			Status:       "error",
			ErrorMessage: fmt.Sprintf("failed to load TEK: %v", err),
		}, nil
	}

	log.Printf("[Persistence] TEK retrieved for organization: %s", req.OrganizationId)

	response := &pb.RetrieveTEKResponse{
		OrganizationId: tekRecord.OrganizationID,
		EncryptedTek:   tekRecord.EncryptedTEK,
		OrgKeyHash:     tekRecord.OrgKeyHash,
		CreatedAt:      timestamppb.New(tekRecord.CreatedAt),
		Version:        int32(tekRecord.Version),
		Status:         "success",
	}

	if tekRecord.RotatedAt != nil {
		response.RotatedAt = timestamppb.New(*tekRecord.RotatedAt)
	}

	return response, nil
}

// loadTEKFromDatabase retrieves a TEK from the database
func (s *PersistenceService) loadTEKFromDatabase(ctx context.Context, organizationID string, orgKey string) (*types.OrganizationTEK, error) {
	query := `
		SELECT encrypted_tek, org_key_hash, created_at, rotated_at, version
		FROM organization_teks
		WHERE organization_id = $1 AND is_active = true
	`

	var encryptedTEK []byte
	var orgKeyHash string
	var createdAt time.Time
	var rotatedAt *time.Time
	var version int

	err := s.db.QueryRowContext(ctx, query, organizationID).Scan(
		&encryptedTEK,
		&orgKeyHash,
		&createdAt,
		&rotatedAt,
		&version,
	)

	if err != nil {
		return nil, err
	}

	// Verify the organization key matches the stored hash
	if !s.verifyOrganizationKey(orgKey, orgKeyHash) {
		return nil, fmt.Errorf("organization key verification failed")
	}

	return &types.OrganizationTEK{
		OrganizationID: organizationID,
		EncryptedTEK:   encryptedTEK,
		OrgKeyHash:     orgKeyHash,
		CreatedAt:      createdAt,
		RotatedAt:      rotatedAt,
		Version:        version,
	}, nil
}

// saveTEKToDatabase persists a TEK to the database
func (s *PersistenceService) saveTEKToDatabase(ctx context.Context, tek *types.OrganizationTEK) error {
	query := `
		INSERT INTO organization_teks (organization_id, encrypted_tek, org_key_hash, created_at, version, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
		ON CONFLICT (organization_id) 
		DO UPDATE SET
			encrypted_tek = EXCLUDED.encrypted_tek,
			org_key_hash = EXCLUDED.org_key_hash,
			rotated_at = NOW(),
			version = organization_teks.version + 1,
			is_active = true
	`

	_, err := s.db.ExecContext(ctx, query,
		tek.OrganizationID,
		tek.EncryptedTEK,
		tek.OrgKeyHash,
		tek.CreatedAt,
		tek.Version,
	)

	return err
}

// verifyOrganizationKey validates the provided organization key against stored hash
func (s *PersistenceService) verifyOrganizationKey(orgKey string, storedHash string) bool {
	// Hash the provided key
	providedHash := sha256.Sum256([]byte(orgKey))
	providedHashStr := hex.EncodeToString(providedHash[:])

	// Compare hashes
	return providedHashStr == storedHash
}

// containsMessages checks if a string contains "messages"
func containsMessages(s string) bool {
	return len(s) >= 8 && (s[len(s)-8:] == "messages" || containsSubstring(s, "messages"))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
