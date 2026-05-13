package godborm

import (
	"bufio"
	"fmt"
	"go-db-orm/godborm/client"
	"os"
	"strings"
)

// Field represents one column in a schema model.
// ... (rest of the file until the removed functions)
type Field struct {
	Name          string
	Type          string
	IsID          bool
	IsNullable    bool
	IsList        bool
	ForeignTable  string
	ForeignColumn string
	DefaultValue  string // e.g. "cuid()", "uuid()", "autoincrement()"
	Relation      *RelationInfo
}

// IsRelation returns true if the field is a model relation (not a DB column).
func (f Field) IsRelation() bool {
	if f.Relation != nil || f.IsList {
		return true
	}
	t := strings.ToLower(f.Type)
	switch t {
	case "int", "string", "datetime", "decimal", "float", "bool", "text":
		return false
	}
	return true
}


// RelationInfo holds metadata for model relations.
type RelationInfo struct {
	Fields     []string
	References []string
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

		isList := false
		if strings.HasSuffix(typeName, "[]") {
			isList = true
			typeName = strings.TrimSuffix(typeName, "[]")
		}

		fmt.Printf("  Field: %s, Type: %s (Nullable: %v, List: %v)\n", fieldParts[0], typeName, isNullable, isList)

		field := Field{
			Name:       fieldParts[0],
			Type:       typeName,
			IsNullable: isNullable,
			IsList:     isList,
		}


		// Join the rest of the line as attributes
		if len(fieldParts) > 2 {
			attrStr := strings.Join(fieldParts[2:], " ")
			// Split by '@' but skip the first empty result
			attrs := strings.Split(attrStr, "@")
			for _, attr := range attrs {
				attr = strings.TrimSpace(attr)
				if attr == "" {
					continue
				}
				part := "@" + attr

				if part == "@id" {
					field.IsID = true
				} else if part == "@uuid" {
					field.DefaultValue = "uuid()"
				} else if part == "@cuid" {
					field.DefaultValue = "cuid()"
				} else if part == "@autoincrement" {
					field.DefaultValue = "autoincrement()"
				} else if strings.HasPrefix(part, "@default(") {
					field.DefaultValue = strings.TrimSuffix(strings.TrimPrefix(part, "@default("), ")")
				} else if strings.HasPrefix(part, "@foreign(") {
					// Parse @foreign(User.id)
					raw := strings.TrimPrefix(part, "@foreign(")
					raw = strings.TrimSuffix(raw, ")")
					subParts := strings.Split(raw, ".")
					if len(subParts) == 2 {
						field.ForeignTable = client.ToTableName(subParts[0])
						field.ForeignColumn = client.ToSnakeCase(subParts[1])
					}
				} else if strings.HasPrefix(part, "@relation(") {
					// Parse @relation(fields: [authorId], references: [id])
					rel := &RelationInfo{}
					raw := strings.TrimPrefix(part, "@relation(")
					raw = strings.TrimSuffix(raw, ")")

					// Split by commas, but be careful with brackets
					if strings.Contains(raw, "fields:") {
						fPart := extractBracketContent(raw, "fields:")
						rel.Fields = splitAndTrim(fPart)
					}
					if strings.Contains(raw, "references:") {
						rPart := extractBracketContent(raw, "references:")
						rel.References = splitAndTrim(rPart)
					}
					field.Relation = rel
				}
			}
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

func extractBracketContent(s, prefix string) string {
	idx := strings.Index(s, prefix)
	if idx == -1 {
		return ""
	}
	sub := s[idx+len(prefix):]
	start := strings.Index(sub, "[")
	end := strings.Index(sub, "]")
	if start != -1 && end != -1 && end > start {
		return sub[start+1 : end]
	}
	return ""
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
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
	case "text":
		if driver == "postgres" || driver == "postgresql" {
			return "TEXT"
		}
		return "LONGTEXT"
	case "decimal":
		return "DECIMAL(10,2)"
	case "float":
		return "DOUBLE"
	case "bool":
		if driver == "postgres" || driver == "postgresql" {
			return "BOOLEAN"
		}
		return "TINYINT(1)"
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
	switch strings.ToLower(field.Type) {
	case "int":
		goType = "int"
	case "string", "text":
		goType = "string"
	case "decimal", "float":
		goType = "float64"
	case "bool":
		goType = "bool"
	case "datetime":
		goType = "time.Time"
	default:
		// Assume it's a relation to another model
		goType = exportName(field.Type)
	}


	if field.IsList {
		return "[]" + goType
	}

	// Relations are always pointers
	if field.IsNullable || field.Relation != nil || (goType != "int" && goType != "string" && goType != "time.Time" && goType != "float64" && goType != "bool") {
		return "*" + goType
	}
	return goType
}



// exportName converts snake_case or other names to PascalCase for Go exporting (e.g. created_at -> CreatedAt).
func exportName(s string) string {
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "")
}
