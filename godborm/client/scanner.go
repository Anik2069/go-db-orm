package client

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
)

// scanRowIntoStruct maps a *sql.Row or *sql.Rows column result into a struct value
// using db:"column" tags (falling back to snake_case of field name).
// Handles NULLable columns safely by scanning into sql.Null* wrappers when needed.
func scanRowIntoStruct(rows *sql.Rows, dest reflect.Value) error {
	t := dest.Type()

	cols, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("orm: scanner: could not read columns: %w", err)
	}

	targets := make([]interface{}, len(cols))
	fieldIndexes := make([]int, len(cols)) // -1 means no matching field

	for i, col := range cols {
		fieldIndexes[i] = -1
		colLower := strings.ToLower(col)
		for j := 0; j < t.NumField(); j++ {
			f := t.Field(j)
			if isRelationField(f) {
				continue
			}
			dbTag := strings.ToLower(f.Tag.Get("db"))
			snake := strings.ToLower(ToSnakeCase(f.Name))
			if dbTag == colLower || snake == colLower {
				fieldIndexes[i] = j
				break
			}
		}

		if fieldIndexes[i] >= 0 {
			fv := dest.Field(fieldIndexes[i])
			targets[i] = nullableTarget(fv)
		} else {
			// Column exists in SQL but not in struct — use a discard sink
			var discard interface{}
			targets[i] = &discard
		}
	}

	if err := rows.Scan(targets...); err != nil {
		return fmt.Errorf("orm: scanner: scan failed: %w", err)
	}

	// Write sql.Null* values back into the struct field
	for i, idx := range fieldIndexes {
		if idx < 0 {
			continue
		}
		if err := writeNullable(dest.Field(idx), targets[i]); err != nil {
			return fmt.Errorf("orm: scanner: field write failed for col %s: %w", cols[i], err)
		}
	}

	return nil
}

// nullableTarget returns an appropriate scan target for the field.
// For pointer fields it returns a **T so nil can be represented.
// For value fields it returns a *sql.NullX wrapper for safety.
func nullableTarget(fv reflect.Value) interface{} {
	ft := fv.Type()

	// Pointer type — scan into **underlying so nil DBs NULL becomes nil pointer
	if ft.Kind() == reflect.Ptr {
		ptr := reflect.New(ft)
		return ptr.Interface()
	}

	// Use sql.Null wrappers for common scalar types
	switch ft.Kind() {
	case reflect.String:
		return &sql.NullString{}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &sql.NullInt64{}
	case reflect.Float32, reflect.Float64:
		return &sql.NullFloat64{}
	case reflect.Bool:
		return &sql.NullBool{}
	default:
		// Fallback: scan directly (covers time.Time, []byte, etc.)
		return fv.Addr().Interface()
	}
}

// writeNullable writes the scanned sql.Null* value back into the struct field.
func writeNullable(fv reflect.Value, target interface{}) error {
	switch v := target.(type) {
	case *sql.NullString:
		if v.Valid {
			fv.SetString(v.String)
		}
	case *sql.NullInt64:
		if v.Valid {
			fv.SetInt(v.Int64)
		}
	case *sql.NullFloat64:
		if v.Valid {
			fv.SetFloat(v.Float64)
		}
	case *sql.NullBool:
		if v.Valid {
			fv.SetBool(v.Bool)
		}
	// Pointer targets — target is **T
	default:
		if fv.Type().Kind() == reflect.Ptr {
			rv := reflect.ValueOf(target)
			// rv is **T, rv.Elem() is *T
			inner := rv.Elem()
			if !inner.IsNil() {
				fv.Set(inner)
			}
			// else leave as nil pointer (NULL in DB)
		}
		// Fallback types (time.Time etc.) were scanned directly — nothing to write back
	}
	return nil
}

// scanSingleRowIntoStruct is a convenience for *sql.Row (which doesn't expose Columns()).
// It uses the struct field order matching the query column order.
func scanStructTargets(dest reflect.Value) []interface{} {
	t := dest.Type()
	var targets []interface{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if isRelationField(f) {
			continue
		}
		targets = append(targets, nullableTarget(dest.Field(i)))
	}
	return targets
}

// applyNullableTargets writes sql.Null* scan results back for scanStructTargets().
func applyNullableTargets(dest reflect.Value, targets []interface{}) error {
	t := dest.Type()
	idx := 0
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if isRelationField(f) {
			continue
		}
		if idx >= len(targets) {
			break
		}
		if err := writeNullable(dest.Field(i), targets[idx]); err != nil {
			return fmt.Errorf("orm: applyNullable field %s: %w", f.Name, err)
		}
		idx++
	}
	return nil
}
