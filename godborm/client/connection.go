package client

import (
	"database/sql"
	"fmt"

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
	db, err := sql.Open(driver, dsn)
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

// Close closes the database connection
func Close() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}
