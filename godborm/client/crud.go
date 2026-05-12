package client

import (
	"fmt"
	"reflect"
	"strings"
)

// Create inserts a new record into the database
func Create(model interface{}) error {
	t := reflect.TypeOf(model).Elem()
	v := reflect.ValueOf(model).Elem()
	table := ToTableName(t.Name())

	var columns []string
	var placeholders []string
	var args []interface{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i).Interface()
		colName := ToSnakeCase(field.Name)
		columns = append(columns, quoteIdentifier(colName))
		placeholders = append(placeholders, getPlaceholder(len(args)+1))
		args = append(args, fieldValue)
	}

	sqlQuery := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		quoteIdentifier(table), strings.Join(columns, ", "), strings.Join(placeholders, ", "))

	_, err := DB.Exec(sqlQuery, args...)
	if err != nil {
		return fmt.Errorf("Create error: %w", err)
	}

	return nil
}

// Find retrieves a single record by ID
func Find(model interface{}, id interface{}) error {
	t := reflect.TypeOf(model).Elem()
	table := ToTableName(t.Name())

	sqlQuery := fmt.Sprintf("SELECT * FROM %s WHERE id = %s LIMIT 1", quoteIdentifier(table), getPlaceholder(1))
	row := DB.QueryRow(sqlQuery, id)

	// Dynamically scan fields into struct
	v := reflect.ValueOf(model).Elem()
	fields := make([]interface{}, v.NumField())
	for i := 0; i < v.NumField(); i++ {
		fields[i] = v.Field(i).Addr().Interface()
	}

	err := row.Scan(fields...)
	if err != nil {
		return fmt.Errorf("Find error: %w", err)
	}

	return nil
}

// Update updates a record by ID (assumes model has ID field)
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

	// Add ID as the last argument
	args = append(args, id)
	sqlQuery := fmt.Sprintf("UPDATE %s SET %s WHERE id=%s", quoteIdentifier(table), strings.Join(sets, ", "), getPlaceholder(len(args)))
	_, err := DB.Exec(sqlQuery, args...)
	if err != nil {
		return fmt.Errorf("Update error: %w", err)
	}

	return nil
}

// Delete deletes a record by ID
func Delete(model interface{}, id interface{}) error {
	t := reflect.TypeOf(model).Elem()
	table := ToTableName(t.Name())

	sqlQuery := fmt.Sprintf("DELETE FROM %s WHERE id=%s", quoteIdentifier(table), getPlaceholder(1))
	_, err := DB.Exec(sqlQuery, id)
	if err != nil {
		return fmt.Errorf("Delete error: %w", err)
	}

	return nil
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
