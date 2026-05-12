package godborm

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Field represents one column in a schema model.
type Field struct {
	Name       string
	Type       string
	IsID       bool
	IsNullable bool
}

// Model represents a parsed schema model.
type Model struct {
	Name   string
	Fields []Field
}

// isSchemaFile returns true when the filename ends with .godb or .schema.
func isSchemaFile(name string) bool {
	return strings.HasSuffix(name, ".godb") || strings.HasSuffix(name, ".schema")
}

// parseSchemaFile reads a .godb or .schema file and returns a list of Models.
func parseSchemaFile(filePath string) ([]Model, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot open file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var models []Model
	var currentModel *Model

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// Start of model
		if strings.HasPrefix(line, "model") {
			// If we were building a model, it's done (nested models not supported)
			if currentModel != nil {
				models = append(models, *currentModel)
			}

			currentModel = &Model{}
			// Remove { and any other non-alphanumeric chars from the name line
			cleanLine := strings.ReplaceAll(line, "{", " ")
			parts := strings.Fields(cleanLine)
			if len(parts) >= 2 {
				currentModel.Name = strings.Trim(parts[1], " \t\n\r{}")
				fmt.Printf("Parsing model: %s\n", currentModel.Name)
			}
			continue
		}

		// End of model (optional if next model starts, but good for explicit termination)
		if line == "}" {
			if currentModel != nil {
				models = append(models, *currentModel)
				currentModel = nil
			}
			continue
		}

		if currentModel == nil {
			continue
		}

		// Parse field
		fieldParts := strings.Fields(line)
		if len(fieldParts) < 2 {
			continue
		}

		typeName := strings.TrimSpace(fieldParts[1])
		isNullable := false
		if strings.HasSuffix(typeName, "?") {
			isNullable = true
			typeName = strings.TrimSuffix(typeName, "?")
		}

		fmt.Printf("  Field: %s, Type: %s (Nullable: %v)\n", fieldParts[0], typeName, isNullable)

		field := Field{
			Name:       fieldParts[0],
			Type:       typeName,
			IsNullable: isNullable,
		}

		if len(fieldParts) > 2 && fieldParts[2] == "@id" {
			field.IsID = true
		}

		currentModel.Fields = append(currentModel.Fields, field)
	}

	// Catch last model if it didn't end with }
	if currentModel != nil {
		models = append(models, *currentModel)
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("no models found in %s", filePath)
	}

	return models, nil
}

// mapTypeToSQL converts schema type to SQL type.
func mapTypeToSQL(typ, driver string) string {
	switch strings.ToLower(typ) {
	case "int":
		if driver == "postgres" || driver == "postgresql" {
			return "INTEGER"
		}
		return "INT"
	case "string":
		return "VARCHAR(255)"
	case "datetime":
		if driver == "postgres" || driver == "postgresql" {
			return "TIMESTAMP"
		}
		return "DATETIME"
	default:
		return "VARCHAR(255)"
	}
}

// mapTypeToGo converts schema type to Go type.
func mapTypeToGo(field Field) string {
	var goType string
	switch field.Type {
	case "int":
		goType = "int"
	case "string":
		goType = "string"
	case "datetime":
		goType = "time.Time"
	default:
		goType = "string"
	}

	if field.IsNullable {
		return "*" + goType
	}
	return goType
}

// toSnakeCase converts Go/Schema names to snake_case for DB.
func toSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, r)
	}
	return strings.ToLower(string(result))
}

// exportName makes the first character upper-case so the identifier is exported.
func exportName(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
