package db

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	_ "github.com/lib/pq"
)

// DatabaseInfo represents database connection details
type DatabaseInfo struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// ParseDatabaseURL parses a PostgreSQL connection URL into components
func ParseDatabaseURL(url string) (*DatabaseInfo, error) {
	// Example: postgres://user:pass@host:port/dbname?sslmode=disable
	info := &DatabaseInfo{
		SSLMode: "disable",
	}

	// Remove postgres:// prefix
	url = strings.TrimPrefix(url, "postgres://")
	url = strings.TrimPrefix(url, "postgresql://")

	// Split by @ to separate auth from host
	parts := strings.SplitN(url, "@", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid connection URL format")
	}

	// Parse auth (user:password)
	authParts := strings.SplitN(parts[0], ":", 2)
	info.User = authParts[0]
	if len(authParts) == 2 {
		info.Password = authParts[1]
	}

	// Parse host, port, dbname, and params
	hostAndDB := parts[1]

	// Split by ? to separate params
	if idx := strings.Index(hostAndDB, "?"); idx != -1 {
		params := hostAndDB[idx+1:]
		hostAndDB = hostAndDB[:idx]

		// Parse SSL mode
		if strings.Contains(params, "sslmode=") {
			for _, param := range strings.Split(params, "&") {
				if strings.HasPrefix(param, "sslmode=") {
					info.SSLMode = strings.TrimPrefix(param, "sslmode=")
				}
			}
		}
	}

	// Split by / to separate host:port from dbname
	dbParts := strings.SplitN(hostAndDB, "/", 2)
	if len(dbParts) == 2 {
		info.DBName = dbParts[1]
	}

	// Parse host and port
	hostPort := strings.SplitN(dbParts[0], ":", 2)
	info.Host = hostPort[0]
	if len(hostPort) == 2 {
		info.Port = hostPort[1]
	} else {
		info.Port = "5432"
	}

	return info, nil
}

// BuildConnectionURL builds a connection URL from DatabaseInfo
func (info *DatabaseInfo) BuildConnectionURL(dbName string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		info.User, info.Password, info.Host, info.Port, dbName, info.SSLMode)
}

// CheckDatabaseExists checks if a database exists
func CheckDatabaseExists(info *DatabaseInfo) (bool, error) {
	// Connect to 'postgres' system database to check if target DB exists
	adminURL := info.BuildConnectionURL("postgres")
	db, err := sql.Open("postgres", adminURL)
	if err != nil {
		return false, fmt.Errorf("failed to connect to postgres database: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return false, fmt.Errorf("failed to ping postgres database: %w", err)
	}

	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)"
	err = db.QueryRow(query, info.DBName).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check database existence: %w", err)
	}

	return exists, nil
}

// CreateDatabase creates a new database if it doesn't exist
func CreateDatabase(info *DatabaseInfo) error {
	log.Printf("🔧 [DB Init] Checking if database '%s' exists...", info.DBName)

	exists, err := CheckDatabaseExists(info)
	if err != nil {
		return err
	}

	if exists {
		log.Printf("✅ [DB Init] Database '%s' already exists", info.DBName)
		return nil
	}

	log.Printf("📦 [DB Init] Creating database '%s'...", info.DBName)

	// Connect to 'postgres' system database to create the target database
	adminURL := info.BuildConnectionURL("postgres")
	db, err := sql.Open("postgres", adminURL)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres database: %w", err)
	}
	defer db.Close()

	// Create database (cannot use prepared statements for CREATE DATABASE)
	query := fmt.Sprintf("CREATE DATABASE %s", info.DBName)
	_, err = db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	log.Printf("✅ [DB Init] Database '%s' created successfully", info.DBName)
	return nil
}

// EnsureDatabase ensures a database exists, creating it if necessary
func EnsureDatabase(databaseURL string) error {
	info, err := ParseDatabaseURL(databaseURL)
	if err != nil {
		return fmt.Errorf("failed to parse database URL: %w", err)
	}

	return CreateDatabase(info)
}

// EnsureDatabaseWithConnection ensures database exists and returns an open connection
func EnsureDatabaseWithConnection(databaseURL string) (*sql.DB, error) {
	// First ensure the database exists
	if err := EnsureDatabase(databaseURL); err != nil {
		return nil, err
	}

	// Now connect to the actual database
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}
