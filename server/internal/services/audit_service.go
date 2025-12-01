package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/PlainFunction/mistokenly/internal/common/config"
	pb "github.com/PlainFunction/mistokenly/proto/audit"
	_ "github.com/lib/pq"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AuditService implements gRPC audit logging with database persistence
type AuditService struct {
	pb.UnimplementedAuditServiceServer
	config *config.Config
	db     *sql.DB
}

// NewAuditService creates a new audit service instance with database persistence
func NewAuditService(cfg *config.Config) (*AuditService, error) {
	log.Println("[Audit] Initializing Audit Service with database persistence")

	// Use the same database as persistence service for now
	// In production, you might want a separate audit database
	dbURL := cfg.DatabaseURL
	if cfg.AuditDatabaseURL != "" {
		dbURL = cfg.AuditDatabaseURL
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to audit database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping audit database: %w", err)
	}
	log.Println("[Audit] Connected to audit database")

	return &AuditService{
		config: cfg,
		db:     db,
	}, nil
}

// Close closes the database connection
func (s *AuditService) Close() error {
	log.Println("[Audit] Closing database connection")
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// LogAccess logs an access event for audit trail
func (s *AuditService) LogAccess(ctx context.Context, req *pb.LogAccessRequest) (*pb.LogAccessResponse, error) {
	log.Printf("[Audit] Logging access: %s operation on %s by %s", req.Operation, req.ReferenceHash, req.RequestingService)

	// Generate audit ID
	auditID := fmt.Sprintf("audit_%s_%d", req.Operation, time.Now().UnixNano())

	// Prepare metadata JSON
	var metadataJSON []byte
	var err error
	if len(req.Metadata) > 0 {
		metadataJSON, err = json.Marshal(req.Metadata)
		if err != nil {
			log.Printf("[Audit] Failed to marshal metadata: %v", err)
			return &pb.LogAccessResponse{
				AuditId:      auditID,
				Status:       "error",
				ErrorMessage: "Failed to marshal metadata",
			}, nil
		}
	} else {
		metadataJSON = []byte("{}")
	}

	// Convert timestamp
	var timestamp time.Time
	if req.Timestamp != nil {
		timestamp = req.Timestamp.AsTime()
	} else {
		timestamp = time.Now()
	}

	// Insert audit log into database
	query := `
		INSERT INTO audit_logs (audit_id, reference_hash, operation, requesting_service, requesting_user, purpose, timestamp, client_ip, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err = s.db.ExecContext(ctx, query,
		auditID,
		req.ReferenceHash,
		req.Operation,
		req.RequestingService,
		req.RequestingUser,
		req.Purpose,
		timestamp,
		req.ClientIp,
		metadataJSON,
	)

	if err != nil {
		log.Printf("[Audit] Failed to insert audit log: %v", err)
		return &pb.LogAccessResponse{
			AuditId:      auditID,
			Status:       "error",
			ErrorMessage: fmt.Sprintf("Database error: %v", err),
		}, nil
	}

	log.Printf("[Audit] Successfully logged audit event: %s", auditID)
	return &pb.LogAccessResponse{
		AuditId: auditID,
		Status:  "success",
	}, nil
}

// GetAuditLogs retrieves audit logs based on criteria
func (s *AuditService) GetAuditLogs(ctx context.Context, req *pb.GetAuditLogsRequest) (*pb.GetAuditLogsResponse, error) {
	log.Printf("[Audit] Retrieving audit logs with filters")

	// Build query with filters
	query := `
		SELECT audit_id, reference_hash, operation, requesting_service, requesting_user, purpose, timestamp, client_ip, metadata
		FROM audit_logs
		WHERE 1=1
	`
	args := []interface{}{}
	argCount := 0

	if req.StartTime != nil {
		argCount++
		query += fmt.Sprintf(" AND timestamp >= $%d", argCount)
		args = append(args, req.StartTime.AsTime())
	}

	if req.EndTime != nil {
		argCount++
		query += fmt.Sprintf(" AND timestamp <= $%d", argCount)
		args = append(args, req.EndTime.AsTime())
	}

	if req.ReferenceHash != "" {
		argCount++
		query += fmt.Sprintf(" AND reference_hash = $%d", argCount)
		args = append(args, req.ReferenceHash)
	}

	if req.RequestingService != "" {
		argCount++
		query += fmt.Sprintf(" AND requesting_service = $%d", argCount)
		args = append(args, req.RequestingService)
	}

	// Get total count
	countQuery := "SELECT COUNT(*) FROM (" + query + ") AS subquery"
	var totalCount int32
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		log.Printf("[Audit] Failed to get total count: %v", err)
		return &pb.GetAuditLogsResponse{
			Status:       "error",
			ErrorMessage: fmt.Sprintf("Database error: %v", err),
		}, nil
	}

	// Add ordering and pagination
	query += " ORDER BY timestamp DESC"

	if req.Limit > 0 {
		argCount++
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, req.Limit)
	}

	if req.Offset > 0 {
		argCount++
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, req.Offset)
	}

	// Execute query
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		log.Printf("[Audit] Failed to query audit logs: %v", err)
		return &pb.GetAuditLogsResponse{
			Status:       "error",
			ErrorMessage: fmt.Sprintf("Database error: %v", err),
		}, nil
	}
	defer rows.Close()

	var logs []*pb.AuditLogEntry
	for rows.Next() {
		var auditID, referenceHash, operation, requestingService, requestingUser, purpose, clientIP sql.NullString
		var timestamp time.Time
		var metadataJSON []byte

		err := rows.Scan(
			&auditID, &referenceHash, &operation, &requestingService,
			&requestingUser, &purpose, &timestamp, &clientIP, &metadataJSON,
		)
		if err != nil {
			log.Printf("[Audit] Failed to scan row: %v", err)
			continue
		}

		// Parse metadata
		var metadata map[string]string
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
				log.Printf("[Audit] Failed to unmarshal metadata: %v", err)
			}
		}

		entry := &pb.AuditLogEntry{
			AuditId:           auditID.String,
			ReferenceHash:     referenceHash.String,
			Operation:         operation.String,
			RequestingService: requestingService.String,
			RequestingUser:    requestingUser.String,
			Purpose:           purpose.String,
			Timestamp:         timestamppb.New(timestamp),
			ClientIp:          clientIP.String,
			Metadata:          metadata,
		}
		logs = append(logs, entry)
	}

	if err := rows.Err(); err != nil {
		log.Printf("[Audit] Error iterating rows: %v", err)
		return &pb.GetAuditLogsResponse{
			Status:       "error",
			ErrorMessage: fmt.Sprintf("Database error: %v", err),
		}, nil
	}

	log.Printf("[Audit] Retrieved %d audit logs (total: %d)", len(logs), totalCount)
	return &pb.GetAuditLogsResponse{
		Logs:       logs,
		TotalCount: totalCount,
		Status:     "success",
	}, nil
}

// HealthCheck implements the health check
func (s *AuditService) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	log.Printf("[Audit] HealthCheck called from: %s", req.ServiceName)

	checks := make(map[string]string)

	if err := s.db.Ping(); err != nil {
		checks["audit_db"] = fmt.Sprintf("unhealthy: %v", err)
	} else {
		checks["audit_db"] = "healthy"
	}

	status := "healthy"
	for _, checkStatus := range checks {
		if checkStatus != "healthy" {
			status = "unhealthy"
			break
		}
	}

	return &pb.HealthCheckResponse{
		Status:      status,
		ServiceName: "audit-service",
		Version:     "1.0.0",
		Timestamp:   timestamppb.New(time.Now()),
		Details:     checks,
	}, nil
}
