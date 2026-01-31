package godborm

import (
	"fmt"
	"go-db-orm/godborm/client"
	"os"
	"strings"
)

// ColumnInfo holds DB column metadata we need for migrations.
type ColumnInfo struct {
	Name string
	Type string
}

// Migrate reads schema files and creates tables in the DB.
func Migrate(schemaPath string) error {
	// Read all files in schemaPath
	files, err := os.ReadDir(schemaPath)
	if err != nil {
		return fmt.Errorf("cannot read schema folder: %w", err)
	}

	for _, file := range files {
		if !isSchemaFile(file.Name()) {
			continue
		}

		filePath := schemaPath + "/" + file.Name()
		model, err := parseSchemaFile(filePath)
		if err != nil {
			return err
		}

		if err := createTable(model); err != nil {
			return err
		}
	}

	return nil
}

func createTable(model Model) error {
	tableName := toSnakeCase(model.Name)

	existingCols, err := existingColumns(tableName)
	if err != nil {
		return err
	}

	// If table doesn't exist, create it from scratch.
	if len(existingCols) == 0 {
		var columns []string
		for _, f := range model.Fields {
			colType := mapTypeToSQL(f.Type)
			if f.IsID {
				colType += " PRIMARY KEY AUTO_INCREMENT"
			}
			columns = append(columns, fmt.Sprintf("%s %s", toSnakeCase(f.Name), colType))
		}

		sqlQuery := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", tableName, strings.Join(columns, ", "))
		_, err := client.DB.Exec(sqlQuery)
		if err != nil {
			return fmt.Errorf("cannot create table %s: %w", model.Name, err)
		}
		fmt.Printf("Table %s created successfully!\n", tableName)
		return nil
	}

	// Otherwise, add missing columns or rename when there is a single rename.
	desiredCols := make(map[string]Field)
	for _, f := range model.Fields {
		desiredCols[strings.ToLower(toSnakeCase(f.Name))] = f
	}

	missing := []Field{}
	for name, f := range desiredCols {
		if _, ok := existingCols[name]; !ok {
			missing = append(missing, f)
		}
	}

	extra := []ColumnInfo{}
	for name, info := range existingCols {
		if _, ok := desiredCols[name]; !ok {
			extra = append(extra, info)
		}
	}

	// Heuristic rename: if exactly one missing and one extra, rename column.
	if len(missing) == 1 && len(extra) == 1 {
		newCol := missing[0]
		oldCol := extra[0]
		if err := renameColumn(tableName, oldCol.Name, toSnakeCase(newCol.Name), mapTypeToSQL(newCol.Type)); err == nil {
			fmt.Printf("Renamed column %s to %s on %s\n", oldCol.Name, toSnakeCase(newCol.Name), tableName)
			// mark rename handled
			missing = nil
			extra = nil
		} else {
			// fall through to add path
			fmt.Printf("Rename attempt failed (%v); adding new column instead\n", err)
		}
	}

	for _, f := range missing {
		colType := mapTypeToSQL(f.Type)
		if f.IsID {
			colType += " PRIMARY KEY AUTO_INCREMENT"
		}
		col := fmt.Sprintf("%s %s", toSnakeCase(f.Name), colType)
		sqlQuery := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", tableName, col)
		if _, err := client.DB.Exec(sqlQuery); err != nil {
			return fmt.Errorf("cannot add column to table %s: %w", model.Name, err)
		}
		fmt.Printf("Added column %s to %s\n", col, tableName)
	}

	// Drop columns that are no longer in schema (after rename heuristic).
	for _, col := range extra {
		if err := dropColumn(tableName, col.Name); err != nil {
			return fmt.Errorf("cannot drop column %s from %s: %w", col.Name, tableName, err)
		}
		fmt.Printf("Dropped column %s from %s\n", col.Name, tableName)
	}
	return nil
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
	case "postgres":
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

func renameColumn(tableName, oldName, newName, newType string) error {
	switch client.DBDriver {
	case "mysql":
		_, err := client.DB.Exec(fmt.Sprintf("ALTER TABLE %s CHANGE `%s` `%s` %s", tableName, oldName, newName, newType))
		return err
	case "postgres":
		_, err := client.DB.Exec(fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", tableName, oldName, newName))
		return err
	default:
		return fmt.Errorf("rename not supported for driver %s", client.DBDriver)
	}
}

func dropColumn(tableName, colName string) error {
	switch client.DBDriver {
	case "mysql":
		_, err := client.DB.Exec(fmt.Sprintf("ALTER TABLE %s DROP COLUMN `%s`", tableName, colName))
		return err
	case "postgres":
		_, err := client.DB.Exec(fmt.Sprintf("ALTER TABLE %s DROP COLUMN \"%s\"", tableName, colName))
		return err
	default:
		return fmt.Errorf("drop not supported for driver %s", client.DBDriver)
	}
}
