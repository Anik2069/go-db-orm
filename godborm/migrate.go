package godborm

import (
	"fmt"
	"go-db-orm/godborm/client"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ColumnInfo holds DB column metadata we need for migrations.
type ColumnInfo struct {
	Name string
	Type string
}

// Migrate reads schema files, generates migration SQL if needed, and applies any pending migration files to the DB.
func Migrate(schemaPath, migrationsPath string) error {
	// 1. Ensure migrations table exists
	if err := ensureMigrationTable(); err != nil {
		return err
	}

	// 2. Read applied migrations from DB
	applied, err := getAppliedMigrations()
	if err != nil {
		return err
	}

	// 3. Apply existing unapplied migration files from migrationsPath
	if migrationsPath != "" {
		files, _ := os.ReadDir(migrationsPath)
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".sql") {
				if !applied[file.Name()] {
					fmt.Printf("Applying pending migration: %s\n", file.Name())
					if err := applyMigrationFile(filepath.Join(migrationsPath, file.Name())); err != nil {
						return err
					}
					if err := recordMigration(file.Name()); err != nil {
						return err
					}
					applied[file.Name()] = true
				}
			}
		}
	}

	// 4. Check for NEW schema changes
	sqls, err := PlanMigration(schemaPath)
	if err != nil {
		return err
	}

	if len(sqls) == 0 {
		fmt.Println("No new schema changes detected.")
		return nil
	}

	// 5. Generate and apply a new migration file for the current changes
	timestamp := time.Now().Format("20060102150405")
	fileName := fmt.Sprintf("%s_migration.sql", timestamp)
	fullPath := filepath.Join(migrationsPath, fileName)

	content := strings.Join(sqls, ";\n") + ";\n"
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write migration file: %w", err)
	}
	fmt.Printf("New migration generated and applied: %s\n", fullPath)

	if err := applyMigrationFile(fullPath); err != nil {
		return err
	}
	return recordMigration(fileName)
}

func ensureMigrationTable() error {
	var sql string
	if client.DBDriver == "postgres" || client.DBDriver == "postgresql" {
		sql = `CREATE TABLE IF NOT EXISTS "migrations" (
			"id" SERIAL PRIMARY KEY,
			"migration" VARCHAR(255) UNIQUE NOT NULL,
			"applied_at" TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`
	} else {
		sql = `CREATE TABLE IF NOT EXISTS migrations (
			id INT AUTO_INCREMENT PRIMARY KEY,
			migration VARCHAR(255) UNIQUE NOT NULL,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`
	}
	_, err := client.DB.Exec(sql)
	return err
}

func getAppliedMigrations() (map[string]bool, error) {
	rows, err := client.DB.Query(`SELECT migration FROM migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		applied[name] = true
	}
	return applied, nil
}

func applyMigrationFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Split by semicolon for execution (basic splitter)
	sqls := strings.Split(string(data), ";")
	for _, sql := range sqls {
		sql = strings.TrimSpace(sql)
		if sql == "" {
			continue
		}
		if _, err := client.DB.Exec(sql); err != nil {
			return fmt.Errorf("failed to execute SQL from %s: %w\nSQL: %s", filePath, err, sql)
		}
	}
	return nil
}

func recordMigration(name string) error {
	_, err := client.DB.Exec(`INSERT INTO migrations (migration) VALUES ($1)`, name)
	if err != nil {
		// Fallback for MySQL
		if client.DBDriver == "mysql" {
			_, err = client.DB.Exec(`INSERT INTO migrations (migration) VALUES (?)`, name)
		}
	}
	return err
}

// PlanMigration compares schema files with DB and returns a list of SQL statements to sync them.
func PlanMigration(schemaPath string) ([]string, error) {
	files, err := os.ReadDir(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read schema folder: %w", err)
	}

	var allSqls []string
	for _, file := range files {
		if !isSchemaFile(file.Name()) {
			continue
		}

		filePath := filepath.Join(schemaPath, file.Name())
		fileModels, err := parseSchemaFile(filePath)
		if err != nil {
			return nil, err
		}

		for _, model := range fileModels {
			sqls, err := createTableSQL(model)
			if err != nil {
				return nil, err
			}
			allSqls = append(allSqls, sqls...)
		}
	}

	return allSqls, nil
}

func createTableSQL(model Model) ([]string, error) {
	tableName := client.ToTableName(model.Name)
	quotedTable := quoteIdentifier(tableName)
	var sqls []string

	existingCols, err := existingColumns(tableName)
	if err != nil {
		return nil, err
	}

	// If table doesn't exist, create it from scratch.
	if len(existingCols) == 0 {
		var columns []string
		var fks []string
		for _, f := range model.Fields {
			colName := client.ToSnakeCase(f.Name)
			colType := mapTypeToSQL(f.Type, client.DBDriver)
			if f.IsID {
				if f.DefaultValue == "uuid()" {
					if client.DBDriver == "postgres" || client.DBDriver == "postgresql" {
						colType = "UUID PRIMARY KEY DEFAULT gen_random_uuid()"
					} else {
						colType = "VARCHAR(36) PRIMARY KEY"
					}
				} else if f.DefaultValue == "cuid()" {
					colType = "VARCHAR(30) PRIMARY KEY"
				} else {
					// Default to autoincrement for int id
					if client.DBDriver == "postgres" || client.DBDriver == "postgresql" {
						colType = "SERIAL PRIMARY KEY"
					} else {
						colType += " PRIMARY KEY AUTO_INCREMENT"
					}
				}
			}
			columns = append(columns, fmt.Sprintf("%s %s", quoteIdentifier(colName), colType))

			if f.ForeignTable != "" {
				fks = append(fks, fmt.Sprintf("FOREIGN KEY (%s) REFERENCES %s(%s)",
					quoteIdentifier(colName),
					quoteIdentifier(f.ForeignTable),
					quoteIdentifier(f.ForeignColumn)))
			}
		}

		allParts := append(columns, fks...)
		sqls = append(sqls, fmt.Sprintf("CREATE TABLE %s (%s)", quotedTable, strings.Join(allParts, ", ")))
		return sqls, nil
	}

	// Otherwise, add missing columns, handle renames, and detect TYPE changes.
	desiredCols := make(map[string]Field)
	for _, f := range model.Fields {
		desiredCols[strings.ToLower(client.ToSnakeCase(f.Name))] = f
	}

	missing := []Field{}
	changed := []Field{}
	for name, f := range desiredCols {
		if existing, ok := existingCols[name]; ok {
			// Check if type changed
			newType := mapTypeToSQL(f.Type, client.DBDriver)
			if f.IsID {
				if f.DefaultValue == "uuid()" {
					if client.DBDriver == "postgres" || client.DBDriver == "postgresql" {
						newType = "uuid"
					} else {
						newType = "varchar(36)"
					}
				} else if f.DefaultValue == "cuid()" {
					newType = "varchar(30)"
				}
			}
			
			if !typesMatch(existing.Type, newType) {
				changed = append(changed, f)
			}
		} else {
			missing = append(missing, f)
		}
	}

	extra := []ColumnInfo{}
	for name, info := range existingCols {
		if _, ok := desiredCols[name]; !ok {
			extra = append(extra, info)
		}
	}

	// Heuristic rename
	if len(missing) == 1 && len(extra) == 1 {
		newCol := missing[0]
		oldCol := extra[0]
		sql, err := getRenameSQL(tableName, oldCol.Name, client.ToSnakeCase(newCol.Name), mapTypeToSQL(newCol.Type, client.DBDriver))
		if err == nil {
			sqls = append(sqls, sql)
			missing = nil
			extra = nil
		}
	}

	// Handle type changes
	for _, f := range changed {
		colName := client.ToSnakeCase(f.Name)
		newType := mapTypeToSQL(f.Type, client.DBDriver)
		if f.IsID {
			if f.DefaultValue == "uuid()" {
				if client.DBDriver == "postgres" || client.DBDriver == "postgresql" {
					newType = "UUID USING " + quoteIdentifier(colName) + "::uuid"
				} else {
					newType = "VARCHAR(36)"
				}
			} else if f.DefaultValue == "cuid()" {
				newType = "VARCHAR(30)"
			}
		}
		
		if client.DBDriver == "postgres" || client.DBDriver == "postgresql" {
			sqls = append(sqls, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s", quotedTable, quoteIdentifier(colName), newType))
		} else {
			sqls = append(sqls, fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s %s", quotedTable, quoteIdentifier(colName), newType))
		}
	}

	for _, f := range missing {
		colType := mapTypeToSQL(f.Type, client.DBDriver)
		if f.IsID {
			if client.DBDriver == "postgres" || client.DBDriver == "postgresql" {
				colType = "SERIAL PRIMARY KEY"
			} else {
				colType += " PRIMARY KEY AUTO_INCREMENT"
			}
		}
		col := fmt.Sprintf("%s %s", quoteIdentifier(client.ToSnakeCase(f.Name)), colType)
		sqls = append(sqls, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", quotedTable, col))
	}

	for _, col := range extra {
		sql, err := getDropSQL(tableName, col.Name)
		if err != nil {
			return nil, err
		}
		sqls = append(sqls, sql)
	}

	return sqls, nil
}

func quoteIdentifier(s string) string {
	if client.DBDriver == "postgres" || client.DBDriver == "postgresql" {
		return fmt.Sprintf("\"%s\"", s)
	}
	return fmt.Sprintf("`%s`", s)
}

func typesMatch(existing, desired string) bool {
	existing = strings.ToLower(existing)
	desired = strings.ToLower(desired)

	// Normalize PostgreSQL types
	if existing == "integer" {
		existing = "int"
	}
	if strings.HasPrefix(existing, "character varying") {
		existing = "varchar"
	}
	if strings.HasPrefix(desired, "varchar") {
		desired = "varchar"
	}

	return strings.Contains(existing, desired) || strings.Contains(desired, existing)
}

// existingColumns returns a lowercase map of column name -> ColumnInfo for tableName.
// If table doesn't exist, the returned map is empty.
func existingColumns(tableName string) (map[string]ColumnInfo, error) {
	result := make(map[string]ColumnInfo)

	switch client.DBDriver {
	case "mysql":
		rows, err := client.DB.Query(`SELECT COLUMN_NAME, COLUMN_TYPE FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ?`, tableName)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var name, typ string
		for rows.Next() {
			if err := rows.Scan(&name, &typ); err != nil {
				return nil, err
			}
			result[strings.ToLower(name)] = ColumnInfo{Name: strings.ToLower(name), Type: typ}
		}
	case "postgres", "postgresql":
		rows, err := client.DB.Query(`SELECT column_name, data_type FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = $1`, tableName)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var name, typ string
		for rows.Next() {
			if err := rows.Scan(&name, &typ); err != nil {
				return nil, err
			}
			result[strings.ToLower(name)] = ColumnInfo{Name: strings.ToLower(name), Type: typ}
		}
	default:
		// Fallback: try describing table; if fails, assume table missing.
		rows, err := client.DB.Query(fmt.Sprintf("SELECT * FROM %s LIMIT 0", tableName))
		if err != nil {
			return result, nil
		}
		defer rows.Close()
		cols, err := rows.Columns()
		if err != nil {
			return nil, err
		}
		for _, c := range cols {
			result[strings.ToLower(c)] = ColumnInfo{Name: strings.ToLower(c), Type: ""}
		}
	}

	return result, nil
}

func getRenameSQL(tableName, oldName, newName, newType string) (string, error) {
	quotedTable := quoteIdentifier(tableName)
	switch client.DBDriver {
	case "mysql":
		return fmt.Sprintf("ALTER TABLE %s CHANGE `%s` `%s` %s", quotedTable, oldName, newName, newType), nil
	case "postgres", "postgresql":
		return fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", quotedTable, quoteIdentifier(oldName), quoteIdentifier(newName)), nil
	default:
		return "", fmt.Errorf("rename not supported for driver %s", client.DBDriver)
	}
}

func getDropSQL(tableName, colName string) (string, error) {
	quotedTable := quoteIdentifier(tableName)
	switch client.DBDriver {
	case "mysql":
		return fmt.Sprintf("ALTER TABLE %s DROP COLUMN `%s`", quotedTable, colName), nil
	case "postgres", "postgresql":
		return fmt.Sprintf("ALTER TABLE %s DROP COLUMN \"%s\"", quotedTable, colName), nil
	default:
		return "", fmt.Errorf("drop not supported for driver %s", client.DBDriver)
	}
}
