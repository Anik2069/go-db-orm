package client

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	// Import drivers (user must install these)
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

// DB is the global database connection
var DB *sql.DB

// DBDriver stores the current driver name (mysql, postgres, etc.)
var DBDriver string

func SetDB(db *sql.DB) {
	DB = db
}

// Connect opens a database connection
// driver: "mysql", "postgres", etc.
// dsn: database connection string
func Connect(driver, dsn string) error {
	// Normalize driver name for Go's sql package
	openDriver := driver
	if driver == "postgresql" {
		openDriver = "postgres"
	}

	db, err := sql.Open(openDriver, dsn)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	err = db.Ping()
	if err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	DB = db
	DBDriver = driver
	fmt.Printf("Connected to %s database successfully!\n", driver)
	return nil
}

// ConnectWithConfig attempts to connect using credentials from godborm.json
func ConnectWithConfig() error {
	type Config struct {
		Driver string `json:"driver"`
		DSN    string `json:"dsn"`
	}

	data, err := os.ReadFile("godborm.json")
	if err != nil {
		return fmt.Errorf("failed to read godborm.json: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse godborm.json: %w", err)
	}

	if cfg.Driver == "" || cfg.DSN == "" {
		return fmt.Errorf("driver or dsn missing in godborm.json")
	}

	return Connect(cfg.Driver, cfg.DSN)
}


// Close closes the database connection
func Close() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}
