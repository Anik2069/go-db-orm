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
	table := strings.ToLower(t.Name())

	var columns []string
	var values []string

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i).Interface()
		columns = append(columns, strings.ToLower(field.Name))
		values = append(values, fmt.Sprintf("'%v'", fieldValue))
	}

	sqlQuery := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		table, strings.Join(columns, ", "), strings.Join(values, ", "))

	_, err := DB.Exec(sqlQuery)
	if err != nil {
		return fmt.Errorf("Create error: %w", err)
	}

	return nil
}

// Find retrieves a single record by ID
func Find(model interface{}, id interface{}) error {
	t := reflect.TypeOf(model).Elem()
	table := strings.ToLower(t.Name())

	sqlQuery := fmt.Sprintf("SELECT * FROM %s WHERE id = '%v' LIMIT 1", table, id)
	row := DB.QueryRow(sqlQuery)

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
	table := strings.ToLower(t.Name())

	var sets []string
	var id interface{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i).Interface()
		if strings.ToLower(field.Name) == "id" {
			id = fieldValue
			continue
		}
		sets = append(sets, fmt.Sprintf("%s='%v'", strings.ToLower(field.Name), fieldValue))
	}

	if id == nil {
		return fmt.Errorf("Update error: ID field is required")
	}

	sqlQuery := fmt.Sprintf("UPDATE %s SET %s WHERE id='%v'", table, strings.Join(sets, ", "), id)
	_, err := DB.Exec(sqlQuery)
	if err != nil {
		return fmt.Errorf("Update error: %w", err)
	}

	return nil
}

// Delete deletes a record by ID
func Delete(model interface{}, id interface{}) error {
	t := reflect.TypeOf(model).Elem()
	table := strings.ToLower(t.Name())

	sqlQuery := fmt.Sprintf("DELETE FROM %s WHERE id='%v'", table, id)
	_, err := DB.Exec(sqlQuery)
	if err != nil {
		return fmt.Errorf("Delete error: %w", err)
	}

	return nil
}
