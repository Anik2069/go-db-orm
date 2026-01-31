package godborm

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Field represents one column in a schema model.
type Field struct {
	Name string
	Type string
	IsID bool
}

// Model represents a parsed schema model.
type Model struct {
	Name   string
	Fields []Field
}

// isSchemaFile returns true when the filename ends with .godb (case-sensitive).
func isSchemaFile(name string) bool {
	return strings.HasSuffix(name, ".godb")
}

// parseSchemaFile reads a .godb file and returns a Model.
func parseSchemaFile(filePath string) (Model, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return Model{}, fmt.Errorf("cannot open file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var model Model

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Start of model
		if strings.HasPrefix(line, "model") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				model.Name = parts[1]
			}
			continue
		}

		// End of model
		if line == "}" {
			continue
		}

		// Parse field
		fieldParts := strings.Fields(line)
		if len(fieldParts) < 2 {
			continue
		}

		field := Field{
			Name: fieldParts[0],
			Type: fieldParts[1],
		}

		if len(fieldParts) > 2 && fieldParts[2] == "@id" {
			field.IsID = true
		}

		model.Fields = append(model.Fields, field)
	}

	if model.Name == "" {
		return Model{}, fmt.Errorf("no model found in %s", filePath)
	}

	return model, nil
}

// mapTypeToSQL converts schema type to SQL type.
func mapTypeToSQL(typ string) string {
	switch typ {
	case "int":
		return "INT"
	case "string":
		return "VARCHAR(255)"
	case "datetime":
		return "DATETIME"
	default:
		return "VARCHAR(255)"
	}
}

// mapTypeToGo converts schema type to Go type.
func mapTypeToGo(typ string) string {
	switch typ {
	case "int":
		return "int"
	case "string":
		return "string"
	case "datetime":
		return "time.Time"
	default:
		return "string"
	}
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
