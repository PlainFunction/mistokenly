package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/lib/pq"
)

// Migrator handles database migrations
type Migrator struct {
	db            *sql.DB
	migrationsDir string
}

// NewMigrator creates a new migrator instance
func NewMigrator(db *sql.DB, migrationsDir string) *Migrator {
	return &Migrator{
		db:            db,
		migrationsDir: migrationsDir,
	}
}

// InitializeMigrationsTable creates the migrations tracking table if it doesn't exist
func (m *Migrator) InitializeMigrationsTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err := m.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}
	log.Println("✅ [Migrator] Schema migrations table initialized")
	return nil
}

// GetAppliedMigrations returns a list of already applied migration versions
func (m *Migrator) GetAppliedMigrations() (map[string]bool, error) {
	applied := make(map[string]bool)

	rows, err := m.db.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("failed to query migrations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, nil
}

// GetPendingMigrations returns migrations that haven't been applied yet
func (m *Migrator) GetPendingMigrations() ([]string, error) {
	// Get list of migration files
	files, err := os.ReadDir(m.migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Get applied migrations
	applied, err := m.GetAppliedMigrations()
	if err != nil {
		return nil, err
	}

	// Find pending migrations
	var pending []string
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".sql") {
			continue
		}

		version := strings.TrimSuffix(file.Name(), ".sql")
		if !applied[version] {
			pending = append(pending, file.Name())
		}
	}

	// Sort migrations by name (which should be numeric prefixed)
	sort.Strings(pending)

	return pending, nil
}

// ApplyMigration applies a single migration file
func (m *Migrator) ApplyMigration(filename string) error {
	log.Printf("📝 [Migrator] Applying migration: %s", filename)

	// Read migration file
	filepath := filepath.Join(m.migrationsDir, filename)
	content, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	// Start transaction
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute migration
	if _, err := tx.Exec(string(content)); err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	// Record migration
	version := strings.TrimSuffix(filename, ".sql")
	if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration: %w", err)
	}

	log.Printf("✅ [Migrator] Successfully applied: %s", filename)
	return nil
}

// MigrateUp runs all pending migrations
func (m *Migrator) MigrateUp() error {
	log.Println("🚀 [Migrator] Starting database migration...")

	// Initialize migrations table
	if err := m.InitializeMigrationsTable(); err != nil {
		return err
	}

	// Get pending migrations
	pending, err := m.GetPendingMigrations()
	if err != nil {
		return err
	}

	if len(pending) == 0 {
		log.Println("✅ [Migrator] No pending migrations, database is up to date")
		return nil
	}

	log.Printf("📋 [Migrator] Found %d pending migration(s)", len(pending))

	// Apply each migration
	for _, filename := range pending {
		if err := m.ApplyMigration(filename); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	log.Printf("✅ [Migrator] Successfully applied %d migration(s)", len(pending))
	return nil
}

// CheckConnection verifies database connectivity
func CheckConnection(databaseURL string) error {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	return nil
}
