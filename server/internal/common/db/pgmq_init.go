package db

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

// PGMQInitializer handles PGMQ extension and queue initialization
type PGMQInitializer struct {
	db *sql.DB
}

// NewPGMQInitializer creates a new PGMQ initializer
func NewPGMQInitializer(db *sql.DB) *PGMQInitializer {
	return &PGMQInitializer{db: db}
}

// CheckExtension checks if the PGMQ extension is installed
func (p *PGMQInitializer) CheckExtension() (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'pgmq')`
	err := p.db.QueryRow(query).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check PGMQ extension: %w", err)
	}
	return exists, nil
}

// InstallExtension installs the PGMQ extension
func (p *PGMQInitializer) InstallExtension() error {
	log.Println("📦 [PGMQ] Installing PGMQ extension...")

	_, err := p.db.Exec("CREATE EXTENSION IF NOT EXISTS pgmq CASCADE")
	if err != nil {
		log.Println("❌ [PGMQ] Failed to install PGMQ extension")
		return fmt.Errorf("pgmq extension not available on PostgreSQL server - see logs for installation instructions: %w", err)
	}

	log.Println("✅ [PGMQ] PGMQ extension installed successfully")
	return nil
}

// CheckQueue checks if a specific queue exists
func (p *PGMQInitializer) CheckQueue(queueName string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM pgmq.meta WHERE queue_name = $1)`
	err := p.db.QueryRow(query, queueName).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check queue existence: %w", err)
	}
	return exists, nil
}

// CreateQueue creates a new PGMQ queue
func (p *PGMQInitializer) CreateQueue(queueName string) error {
	log.Printf("📬 [PGMQ] Creating queue: %s", queueName)

	query := `SELECT pgmq.create($1)`
	_, err := p.db.Exec(query, queueName)
	if err != nil {
		return fmt.Errorf("failed to create queue: %w", err)
	}

	log.Printf("✅ [PGMQ] Queue '%s' created successfully", queueName)
	return nil
}

// EnsureQueue ensures a queue exists, creating it if necessary
func (p *PGMQInitializer) EnsureQueue(queueName string) error {
	exists, err := p.CheckQueue(queueName)
	if err != nil {
		return err
	}

	if exists {
		log.Printf("✅ [PGMQ] Queue '%s' already exists", queueName)
		return nil
	}

	return p.CreateQueue(queueName)
}

// Initialize performs full PGMQ initialization
func (p *PGMQInitializer) Initialize(queueNames ...string) error {
	log.Println("🚀 [PGMQ] Initializing PGMQ...")

	// Check and install extension if needed
	exists, err := p.CheckExtension()
	if err != nil {
		return err
	}

	if !exists {
		if err := p.InstallExtension(); err != nil {
			return err
		}
	} else {
		log.Println("✅ [PGMQ] PGMQ extension already installed")
	}

	// Ensure all required queues exist
	for _, queueName := range queueNames {
		if err := p.EnsureQueue(queueName); err != nil {
			return err
		}
	}

	log.Println("✅ [PGMQ] PGMQ initialization complete")
	return nil
}

// GetQueueStats returns statistics for a queue
func (p *PGMQInitializer) GetQueueStats(queueName string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	query := `
		SELECT 
			COUNT(*) as total_messages,
			COUNT(*) FILTER (WHERE vt > NOW()) as invisible_messages,
			COUNT(*) FILTER (WHERE vt <= NOW()) as visible_messages
		FROM pgmq.q_` + queueName

	var total, invisible, visible int64
	err := p.db.QueryRow(query).Scan(&total, &invisible, &visible)
	if err != nil {
		return nil, fmt.Errorf("failed to get queue stats: %w", err)
	}

	stats["total_messages"] = total
	stats["invisible_messages"] = invisible
	stats["visible_messages"] = visible
	stats["queue_name"] = queueName

	return stats, nil
}
