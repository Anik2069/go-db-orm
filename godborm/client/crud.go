package client

import (
	"crypto/rand"
	"fmt"
	"reflect"
	"strings"
)

// DBBuilder allows for chaining query options
type DBBuilder struct {
	selectedCols []string
}

// Select specifies the columns to retrieve
func Select(cols ...string) *DBBuilder {
	return &DBBuilder{selectedCols: cols}
}

// Create inserts a new record into the database
func Create(model interface{}) error {
	v := reflect.ValueOf(model).Elem()
	t := v.Type()
	table := ToTableName(t.Name())

	var columns []string
	var placeholders []string
	var args []interface{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)
		
		// Handle godb tags for ID generation
		tag := field.Tag.Get("godb")
		if strings.Contains(tag, "cuid") && fieldValue.String() == "" {
			cuid := generateCUID()
			fieldValue.SetString(cuid)
		} else if strings.Contains(tag, "uuid") && fieldValue.String() == "" {
			uuid := generateUUID()
			fieldValue.SetString(uuid)
		}

		colName := ToSnakeCase(field.Name)
		columns = append(columns, quoteIdentifier(colName))
		placeholders = append(placeholders, getPlaceholder(len(args)+1))
		args = append(args, fieldValue.Interface())
	}

	sqlQuery := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		quoteIdentifier(table), strings.Join(columns, ", "), strings.Join(placeholders, ", "))

	_, err := DB.Exec(sqlQuery, args...)
	if err != nil {
		return fmt.Errorf("Create error: %w", err)
	}

	return nil
}

// FindAll retrieves records from the database into a slice.
func (b *DBBuilder) FindAll(dest interface{}) error {
	v := reflect.ValueOf(dest).Elem()
	t := v.Type().Elem()
	table := ToTableName(t.Name())

	var queryCols string
	if len(b.selectedCols) > 0 {
		var quoted []string
		for _, c := range b.selectedCols {
			quoted = append(quoted, quoteIdentifier(c))
		}
		queryCols = strings.Join(quoted, ", ")
	} else {
		queryCols = getColumns(t)
	}

	sqlQuery := fmt.Sprintf("SELECT %s FROM %s", queryCols, quoteIdentifier(table))
	rows, err := DB.Query(sqlQuery)
	if err != nil {
		return fmt.Errorf("FindAll error: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		item := reflect.New(t).Elem()
		var scanTargets []interface{}

		if len(b.selectedCols) > 0 {
			for _, col := range b.selectedCols {
				found := false
				for i := 0; i < t.NumField(); i++ {
					if strings.ToLower(ToSnakeCase(t.Field(i).Name)) == strings.ToLower(col) {
						scanTargets = append(scanTargets, item.Field(i).Addr().Interface())
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("column %s not found in struct %s", col, t.Name())
				}
			}
		} else {
			for i := 0; i < item.NumField(); i++ {
				scanTargets = append(scanTargets, item.Field(i).Addr().Interface())
			}
		}

		if err := rows.Scan(scanTargets...); err != nil {
			return fmt.Errorf("FindAll scan error: %w", err)
		}
		v.Set(reflect.Append(v, item))
	}

	return nil
}

// Find retrieves a single record by ID.
func (b *DBBuilder) Find(model interface{}, id interface{}) error {
	v := reflect.ValueOf(model).Elem()
	t := v.Type()
	table := ToTableName(t.Name())

	var queryCols string
	if len(b.selectedCols) > 0 {
		var quoted []string
		for _, c := range b.selectedCols {
			quoted = append(quoted, quoteIdentifier(c))
		}
		queryCols = strings.Join(quoted, ", ")
	} else {
		queryCols = getColumns(t)
	}

	sqlQuery := fmt.Sprintf("SELECT %s FROM %s WHERE id = %s LIMIT 1", queryCols, quoteIdentifier(table), getPlaceholder(1))
	row := DB.QueryRow(sqlQuery, id)

	var scanTargets []interface{}
	if len(b.selectedCols) > 0 {
		for _, col := range b.selectedCols {
			found := false
			for i := 0; i < t.NumField(); i++ {
				if strings.ToLower(ToSnakeCase(t.Field(i).Name)) == strings.ToLower(col) {
					scanTargets = append(scanTargets, v.Field(i).Addr().Interface())
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("column %s not found in struct %s", col, t.Name())
			}
		}
	} else {
		for i := 0; i < v.NumField(); i++ {
			scanTargets = append(scanTargets, v.Field(i).Addr().Interface())
		}
	}

	err := row.Scan(scanTargets...)
	if err != nil {
		return fmt.Errorf("Find error: %w", err)
	}

	return nil
}

// Global Shortcuts
func FindAll(dest interface{}) error {
	return (&DBBuilder{}).FindAll(dest)
}

func Find(model interface{}, id interface{}) error {
	return (&DBBuilder{}).Find(model, id)
}

// Update updates a record by ID
func Update(model interface{}) error {
	t := reflect.TypeOf(model).Elem()
	v := reflect.ValueOf(model).Elem()
	table := ToTableName(t.Name())

	var sets []string
	var args []interface{}
	var id interface{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i).Interface()
		colName := ToSnakeCase(field.Name)
		if colName == "id" {
			id = fieldValue
			continue
		}
		sets = append(sets, fmt.Sprintf("%s=%s", quoteIdentifier(colName), getPlaceholder(len(args)+1)))
		args = append(args, fieldValue)
	}

	if id == nil {
		return fmt.Errorf("Update error: ID field is required")
	}

	args = append(args, id)
	sqlQuery := fmt.Sprintf("UPDATE %s SET %s WHERE id=%s", quoteIdentifier(table), strings.Join(sets, ", "), getPlaceholder(len(args)))
	_, err := DB.Exec(sqlQuery, args...)
	return err
}

// Delete deletes a record by ID
func Delete(model interface{}, id interface{}) error {
	t := reflect.TypeOf(model).Elem()
	table := ToTableName(t.Name())
	sqlQuery := fmt.Sprintf("DELETE FROM %s WHERE id=%s", quoteIdentifier(table), getPlaceholder(1))
	_, err := DB.Exec(sqlQuery, id)
	return err
}

// Helpers
func getColumns(t reflect.Type) string {
	var cols []string
	for i := 0; i < t.NumField(); i++ {
		cols = append(cols, quoteIdentifier(ToSnakeCase(t.Field(i).Name)))
	}
	return strings.Join(cols, ", ")
}

func getPlaceholder(n int) string {
	if DBDriver == "postgres" || DBDriver == "postgresql" {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

func quoteIdentifier(s string) string {
	if DBDriver == "postgres" || DBDriver == "postgresql" {
		return fmt.Sprintf("\"%s\"", s)
	}
	return fmt.Sprintf("`%s`", s)
}

func generateCUID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return fmt.Sprintf("c%x", b)
}

func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
