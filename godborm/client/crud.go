package client

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
)

// whereClause represents a WHERE condition with its arguments
type whereClause struct {
	condition string
	args      []interface{}
}

// DBBuilder allows for chaining query options
type DBBuilder struct {
	selectedCols      []string
	includedRelations []string
	autoAddedCols     []string
	whereClauses      []whereClause
	joins             []string
	orderBy           string
	limit             int
	offset            int
	executor          sqlExecutor
}



// Select specifies the columns to retrieve
func Select(cols ...string) *DBBuilder {
	return &DBBuilder{selectedCols: cols}
}

// Include specifies the relations to load
func (b *DBBuilder) Include(relations ...string) *DBBuilder {
	b.includedRelations = append(b.includedRelations, relations...)
	return b
}

func Include(relations ...string) *DBBuilder {
	return (&DBBuilder{}).Include(relations...)
}

// Where adds a WHERE clause to the query
func (b *DBBuilder) Where(condition string, args ...interface{}) *DBBuilder {
	b.whereClauses = append(b.whereClauses, whereClause{condition: condition, args: args})
	return b
}

func Where(condition string, args ...interface{}) *DBBuilder {
	return (&DBBuilder{}).Where(condition, args...)
}

// OrderBy adds an ORDER BY clause to the query. Supports multiple calls.
func (b *DBBuilder) OrderBy(orderBy string) *DBBuilder {
	if b.orderBy != "" {
		b.orderBy += ", " + orderBy
	} else {
		b.orderBy = orderBy
	}
	return b
}


func OrderBy(orderBy string) *DBBuilder {
	return (&DBBuilder{}).OrderBy(orderBy)
}

// Limit adds a LIMIT clause to the query
func (b *DBBuilder) Limit(limit int) *DBBuilder {
	b.limit = limit
	return b
}

func Limit(limit int) *DBBuilder {
	return (&DBBuilder{}).Limit(limit)
}

// Offset adds an OFFSET clause to the query
func (b *DBBuilder) Offset(offset int) *DBBuilder {
	b.offset = offset
	return b
}

func Offset(offset int) *DBBuilder {
	return (&DBBuilder{}).Offset(offset)
}

// Join adds a JOIN clause to the query.
// It can be a raw SQL join snippet or just a relation name.
func (b *DBBuilder) Join(join string) *DBBuilder {
	b.joins = append(b.joins, join)
	return b
}

func Join(join string) *DBBuilder {
	return (&DBBuilder{}).Join(join)
}



// Create inserts a new record into the database.
func Create(model interface{}) error {
	return createWithExecutor(DB, model)
}

// createWithExecutor is the internal implementation used by both Create and Tx.Create.
func createWithExecutor(exec sqlExecutor, model interface{}) error {
	v := reflect.ValueOf(model).Elem()
	t := v.Type()
	table := ToTableName(t.Name())

	var columns []string
	var placeholders []string
	var args []interface{}
	var autoIncrField string
	var autoIncrIdx int

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		if isRelationField(field) {
			continue
		}

		tag := field.Tag.Get("godb")

		// Skip auto-increment fields during insert
		if strings.Contains(tag, "autoincrement") {
			autoIncrField = field.Name
			autoIncrIdx = i
			continue
		}

		// Handle ID generation
		if strings.Contains(tag, "cuid") && fieldValue.String() == "" {
			fieldValue.SetString(generateCUID())
		} else if strings.Contains(tag, "uuid") && fieldValue.String() == "" {
			fieldValue.SetString(generateUUID())
		}

		dbTag := field.Tag.Get("db")
		colName := dbTag
		if colName == "" {
			colName = ToSnakeCase(field.Name)
		}

		columns = append(columns, quoteIdentifier(colName))
		placeholders = append(placeholders, getPlaceholder(len(args)+1))
		args = append(args, fieldValue.Interface())
	}

	sqlQuery := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		quoteIdentifier(table), strings.Join(columns, ", "), strings.Join(placeholders, ", "))

	if autoIncrField != "" && (DBDriver == "postgres" || DBDriver == "postgresql") {
		dbTag := t.Field(autoIncrIdx).Tag.Get("db")
		colName := dbTag
		if colName == "" {
			colName = ToSnakeCase(autoIncrField)
		}
		sqlQuery += fmt.Sprintf(" RETURNING %s", quoteIdentifier(colName))

		var lastID int64
		if err := exec.QueryRow(sqlQuery, args...).Scan(&lastID); err != nil {
			return fmt.Errorf("orm: create %s (postgres returning): %w", table, err)
		}
		v.Field(autoIncrIdx).SetInt(lastID)
	} else {
		res, err := exec.Exec(sqlQuery, args...)
		if err != nil {
			return fmt.Errorf("orm: create %s: %w", table, err)
		}
		if autoIncrField != "" {
			if lastID, err := res.LastInsertId(); err == nil {
				v.Field(autoIncrIdx).SetInt(lastID)
			}
		}
	}
	return nil
}


// FindAll retrieves records from the database into a slice.
func (b *DBBuilder) FindAll(dest interface{}) error {
	v := reflect.ValueOf(dest).Elem()
	t := v.Type().Elem()
	table := ToTableName(t.Name())
	b.ensureRelationFields(t)

	var queryCols string
	if len(b.selectedCols) > 0 {
		var quoted []string
		for _, c := range b.selectedCols {
			quoted = append(quoted, quoteIdentifier(c))
		}
		queryCols = strings.Join(quoted, ", ")
	} else {
		queryCols = getColumns(t, table)
	}

	sqlQuery := fmt.Sprintf("SELECT %s FROM %s", queryCols, quoteIdentifier(table))

	if len(b.joins) > 0 {
		for _, join := range b.joins {
			if !strings.Contains(strings.ToUpper(join), " JOIN ") && !strings.Contains(join, " ON ") {
				relSQL, err := b.resolveJoinRelation(t, join)
				if err == nil {
					sqlQuery += " " + relSQL
				} else {
					sqlQuery += " JOIN " + join
				}
			} else {
				if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(join)), "INNER ") &&
					!strings.HasPrefix(strings.ToUpper(strings.TrimSpace(join)), "LEFT ") &&
					!strings.HasPrefix(strings.ToUpper(strings.TrimSpace(join)), "RIGHT ") &&
					!strings.HasPrefix(strings.ToUpper(strings.TrimSpace(join)), "JOIN ") {
					sqlQuery += " JOIN " + join
				} else {
					sqlQuery += " " + join
				}
			}
		}
	}

	var args []interface{}
	if len(b.whereClauses) > 0 {
		var conditions []string
		for _, wc := range b.whereClauses {
			cond := wc.condition
			// Replace '?' with correct placeholders for the driver
			for _, arg := range wc.args {
				if strings.Contains(cond, "?") {
					placeholder := getPlaceholder(len(args) + 1)
					cond = strings.Replace(cond, "?", placeholder, 1)
					args = append(args, arg)
				}
			}
			conditions = append(conditions, cond)
		}
		sqlQuery += " WHERE " + strings.Join(conditions, " AND ")
	}

	if b.orderBy != "" {
		sqlQuery += " ORDER BY " + b.orderBy
	}

	if b.limit > 0 {
		sqlQuery += fmt.Sprintf(" LIMIT %d", b.limit)
	}

	if b.offset > 0 {
		sqlQuery += fmt.Sprintf(" OFFSET %d", b.offset)
	}

	exec := b.executor
	if exec == nil {
		exec = DB
	}
	rows, err := exec.Query(sqlQuery, args...)


	if err != nil {
		return fmt.Errorf("FindAll error: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		item := reflect.New(t).Elem()

		// Use the NULL-safe column-name-aware scanner
		if err := scanRowIntoStruct(rows, item); err != nil {
			return fmt.Errorf("orm: findAll %s: %w", table, err)
		}

		if len(b.includedRelations) > 0 {
			if err := b.loadRelations(item, b.includedRelations); err != nil {
				return err
			}
			// Clear auto-added fields so they don't show up in JSON (respecting Select intent)
			for _, col := range b.autoAddedCols {
				clearField(item, col)
			}
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
	b.ensureRelationFields(t)

	var queryCols string
	if len(b.selectedCols) > 0 {
		var quoted []string
		for _, c := range b.selectedCols {
			quoted = append(quoted, quoteIdentifier(c))
		}
		queryCols = strings.Join(quoted, ", ")
	} else {
		queryCols = getColumns(t, table)
	}

	sqlQuery := fmt.Sprintf("SELECT %s FROM %s", queryCols, quoteIdentifier(table))

	if len(b.joins) > 0 {
		for _, join := range b.joins {
			if !strings.Contains(strings.ToUpper(join), " JOIN ") && !strings.Contains(join, " ON ") {
				relSQL, err := b.resolveJoinRelation(t, join)
				if err == nil {
					sqlQuery += " " + relSQL
				} else {
					sqlQuery += " JOIN " + join
				}
			} else {
				if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(join)), "INNER ") &&
					!strings.HasPrefix(strings.ToUpper(strings.TrimSpace(join)), "LEFT ") &&
					!strings.HasPrefix(strings.ToUpper(strings.TrimSpace(join)), "RIGHT ") &&
					!strings.HasPrefix(strings.ToUpper(strings.TrimSpace(join)), "JOIN ") {
					sqlQuery += " JOIN " + join
				} else {
					sqlQuery += " " + join
				}
			}
		}
	}

	sqlQuery += fmt.Sprintf(" WHERE %s.id = %s", quoteIdentifier(table), getPlaceholder(1))
	var args []interface{}
	args = append(args, id)

	if len(b.whereClauses) > 0 {
		for _, wc := range b.whereClauses {
			cond := wc.condition
			// Replace '?' with correct placeholders for the driver
			for _, arg := range wc.args {
				if strings.Contains(cond, "?") {
					placeholder := getPlaceholder(len(args) + 1)
					cond = strings.Replace(cond, "?", placeholder, 1)
					args = append(args, arg)
				}
			}
			sqlQuery += " AND " + cond
		}
	}
	sqlQuery += " LIMIT 1"

	exec := b.executor
	if exec == nil {
		exec = DB
	}
	row := exec.QueryRow(sqlQuery, args...)



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
		for i := 0; i < t.NumField(); i++ {
			if isRelationField(t.Field(i)) {
				continue
			}
			scanTargets = append(scanTargets, v.Field(i).Addr().Interface())
		}
	}


	err := row.Scan(scanTargets...)
	if err != nil {
		return fmt.Errorf("Find error: %w", err)
	}

	if len(b.includedRelations) > 0 {
		if err := b.loadRelations(v, b.includedRelations); err != nil {
			return err
		}
		// Clear auto-added fields
		for _, col := range b.autoAddedCols {
			clearField(v, col)
		}
		return nil
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

// Raw Query support
type RawBuilder struct {
	sql  string
	args []interface{}
}

func Raw(query string, args ...interface{}) *RawBuilder {
	return &RawBuilder{sql: query, args: args}
}

func (rb *RawBuilder) Exec() (sql.Result, error) {
	return DB.Exec(rb.sql, rb.args...)
}

func (rb *RawBuilder) Scan(dest interface{}) error {
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Ptr {
		return fmt.Errorf("Raw.Scan: destination must be a pointer")
	}
	v = v.Elem()

	rows, err := DB.Query(rb.sql, rb.args...)
	if err != nil {
		return fmt.Errorf("Raw.Scan error: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("Raw.Scan columns error: %w", err)
	}

	isSlice := v.Kind() == reflect.Slice
	targetType := v.Type()
	if isSlice {
		targetType = targetType.Elem()
	}

	foundAny := false
	for rows.Next() {
		foundAny = true
		var item reflect.Value
		var targets []interface{}

		if targetType.Kind() == reflect.Struct {
			item = reflect.New(targetType).Elem()
			targets = make([]interface{}, len(cols))
			for i, col := range cols {
				found := false
				for j := 0; j < targetType.NumField(); j++ {
					field := targetType.Field(j)
					dbTag := field.Tag.Get("db")
					if strings.ToLower(dbTag) == strings.ToLower(col) || strings.ToLower(ToSnakeCase(field.Name)) == strings.ToLower(col) {
						targets[i] = item.Field(j).Addr().Interface()
						found = true
						break
					}
				}
				if !found {
					var dummy interface{}
					targets[i] = &dummy
				}
			}
		} else {
			// Basic type
			item = reflect.New(targetType).Elem()
			targets = []interface{}{item.Addr().Interface()}
		}

		if err := rows.Scan(targets...); err != nil {
			return fmt.Errorf("Raw.Scan rows.Scan error: %w", err)
		}

		if isSlice {
			v.Set(reflect.Append(v, item))
		} else {
			v.Set(item)
			return nil
		}
	}

	if !isSlice && !foundAny {
		return sql.ErrNoRows
	}

	return nil
}

// Update updates a record by ID.
func Update(model interface{}) error {
	return updateWithExecutor(DB, model)
}

// updateWithExecutor is the internal implementation used by both Update and Tx.Update.
func updateWithExecutor(exec sqlExecutor, model interface{}) error {
	t := reflect.TypeOf(model).Elem()
	v := reflect.ValueOf(model).Elem()
	table := ToTableName(t.Name())

	var sets []string
	var args []interface{}
	var id interface{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if isRelationField(field) {
			continue
		}

		fieldValue := v.Field(i).Interface()
		dbTag := field.Tag.Get("db")
		colName := dbTag
		if colName == "" {
			colName = ToSnakeCase(field.Name)
		}

		if colName == "id" {
			id = fieldValue
			continue
		}
		sets = append(sets, fmt.Sprintf("%s=%s", quoteIdentifier(colName), getPlaceholder(len(args)+1)))
		args = append(args, fieldValue)
	}

	if id == nil {
		return fmt.Errorf("orm: update %s: id field is required", table)
	}

	args = append(args, id)
	sqlQuery := fmt.Sprintf("UPDATE %s SET %s WHERE id=%s",
		quoteIdentifier(table), strings.Join(sets, ", "), getPlaceholder(len(args)))

	if _, err := exec.Exec(sqlQuery, args...); err != nil {
		return fmt.Errorf("orm: update %s: %w", table, err)
	}
	return nil
}

// Delete deletes a record by ID.
func Delete(model interface{}, id interface{}) error {
	return deleteWithExecutor(DB, model, id)
}

// deleteWithExecutor is the internal implementation used by both Delete and Tx.Delete.
func deleteWithExecutor(exec sqlExecutor, model interface{}, id interface{}) error {
	t := reflect.TypeOf(model).Elem()
	table := ToTableName(t.Name())
	sqlQuery := fmt.Sprintf("DELETE FROM %s WHERE id=%s", quoteIdentifier(table), getPlaceholder(1))
	if _, err := exec.Exec(sqlQuery, id); err != nil {
		return fmt.Errorf("orm: delete %s id=%v: %w", table, id, err)
	}
	return nil
}

// Helpers
func getColumns(t reflect.Type, tableName string) string {
	var cols []string
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if isRelationField(field) {
			continue
		}

		dbTag := field.Tag.Get("db")
		colName := dbTag
		if colName == "" {
			colName = ToSnakeCase(field.Name)
		}
		if tableName != "" {
			cols = append(cols, fmt.Sprintf("%s.%s", quoteIdentifier(tableName), quoteIdentifier(colName)))
		} else {
			cols = append(cols, quoteIdentifier(colName))
		}
	}
	return strings.Join(cols, ", ")
}

func isRelationField(field reflect.StructField) bool {
	godbTag := field.Tag.Get("godb")
	if strings.Contains(godbTag, "relation") {
		return true
	}
	// Also skip if it has no db tag and is a complex type (struct or slice)
	dbTag := field.Tag.Get("db")
	if dbTag == "" {
		k := field.Type.Kind()
		if k == reflect.Ptr {
			k = field.Type.Elem().Kind()
		}
		if k == reflect.Struct || k == reflect.Slice {
			return true
		}
	}
	return false
}


func getPlaceholder(n int) string {
	if DBDriver == "postgres" || DBDriver == "postgresql" {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

func (b *DBBuilder) loadRelations(v reflect.Value, relations []string) error {
	t := v.Type()
	for _, relStr := range relations {
		relName, relCols := parseInclude(relStr)
		field, ok := t.FieldByName(relName)
		if !ok {
			// Try finding by tag or lowercase?
			continue
		}

		tag := field.Tag.Get("godb")
		if !strings.Contains(tag, "relation") {
			continue
		}

		// fields=user_id,references=id
		relFields, relRefs := parseRelationMetadata(tag)
		if len(relFields) == 0 {
			// Back-relations not supported yet for simplicity
			continue
		}

		// Get values from the current object
		// Assume single field for now
		localCol := relFields[0]
		refCol := relRefs[0]

		var localVal interface{}
		found := false
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if strings.ToLower(f.Tag.Get("db")) == strings.ToLower(localCol) || strings.ToLower(ToSnakeCase(f.Name)) == strings.ToLower(localCol) {
				localVal = v.Field(i).Interface()
				found = true
				break
			}
		}

		if !found {
			continue
		}

		targetType := field.Type
		isSlice := targetType.Kind() == reflect.Slice
		if isSlice {
			targetType = targetType.Elem()
		} else if targetType.Kind() == reflect.Ptr {
			targetType = targetType.Elem()
		}

		targetTable := ToTableName(targetType.Name())
		
		if isSlice {
			// Load many
			sliceVal := reflect.New(field.Type).Elem()
			
			var queryCols string
			if len(relCols) > 0 {
				var quoted []string
				for _, c := range relCols {
					quoted = append(quoted, quoteIdentifier(c))
				}
				queryCols = strings.Join(quoted, ", ")
			} else {
				queryCols = getColumns(targetType, targetTable)
			}

			sql := fmt.Sprintf("SELECT %s FROM %s WHERE %s = %s", 
				queryCols, quoteIdentifier(targetTable), quoteIdentifier(refCol), getPlaceholder(1))
			
			rows, err := DB.Query(sql, localVal)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				item := reflect.New(targetType).Elem()
				var targets []interface{}
				if len(relCols) > 0 {
					targets = getSpecificScanTargets(item, relCols)
				} else {
					targets = getScanTargets(item)
				}

				if err := rows.Scan(targets...); err != nil {
					return err
				}
				sliceVal = reflect.Append(sliceVal, item)
			}
			v.FieldByName(relName).Set(sliceVal)
		} else {
			// Load one
			item := reflect.New(targetType).Elem()
			
			var queryCols string
			if len(relCols) > 0 {
				var quoted []string
				for _, c := range relCols {
					quoted = append(quoted, quoteIdentifier(c))
				}
				queryCols = strings.Join(quoted, ", ")
			} else {
				queryCols = getColumns(targetType, targetTable)
			}

			sql := fmt.Sprintf("SELECT %s FROM %s WHERE %s = %s LIMIT 1", 
				queryCols, quoteIdentifier(targetTable), quoteIdentifier(refCol), getPlaceholder(1))
			
			row := DB.QueryRow(sql, localVal)
			var targets []interface{}
			if len(relCols) > 0 {
				targets = getSpecificScanTargets(item, relCols)
			} else {
				targets = getScanTargets(item)
			}

			if err := row.Scan(targets...); err != nil {
				continue // Or return error? If optional, maybe skip.
			}
			
			if field.Type.Kind() == reflect.Ptr {
				v.FieldByName(relName).Set(item.Addr())
			} else {
				v.FieldByName(relName).Set(item)
			}
		}
	}
	return nil
}

func (b *DBBuilder) ensureRelationFields(t reflect.Type) {
	if len(b.includedRelations) == 0 || len(b.selectedCols) == 0 {
		return
	}

	selectedMap := make(map[string]bool)
	for _, col := range b.selectedCols {
		selectedMap[strings.ToLower(col)] = true
	}

	for _, relStr := range b.includedRelations {
		relName, _ := parseInclude(relStr)
		field, ok := t.FieldByName(relName)
		if !ok {
			continue
		}
		tag := field.Tag.Get("godb")
		if !strings.Contains(tag, "relation") {
			continue
		}
		fields, _ := parseRelationMetadata(tag)
		for _, f := range fields {
			if !selectedMap[strings.ToLower(f)] {
				b.selectedCols = append(b.selectedCols, f)
				selectedMap[strings.ToLower(f)] = true
				b.autoAddedCols = append(b.autoAddedCols, f)
			}
		}
	}
}

func clearField(v reflect.Value, colName string) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if strings.ToLower(f.Tag.Get("db")) == strings.ToLower(colName) || strings.ToLower(ToSnakeCase(f.Name)) == strings.ToLower(colName) {
			v.Field(i).Set(reflect.Zero(f.Type))
			return
		}
	}
}

func parseInclude(rel string) (name string, cols []string) {
	if strings.Contains(rel, ":") {
		parts := strings.SplitN(rel, ":", 2)
		name = parts[0]
		cols = strings.Split(parts[1], ",")
		for i := range cols {
			cols[i] = strings.TrimSpace(cols[i])
		}
		return
	}
	return rel, nil
}

func getSpecificScanTargets(v reflect.Value, cols []string) []interface{} {
	t := v.Type()
	var targets []interface{}
	for _, col := range cols {
		found := false
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if strings.ToLower(f.Tag.Get("db")) == strings.ToLower(col) || strings.ToLower(ToSnakeCase(f.Name)) == strings.ToLower(col) {
				targets = append(targets, v.Field(i).Addr().Interface())
				found = true
				break
			}
		}
		if !found {
			// Add a dummy target to avoid scan error if column not in struct
			var dummy interface{}
			targets = append(targets, &dummy)
		}
	}
	return targets
}

func getScanTargets(v reflect.Value) []interface{} {
	t := v.Type()
	var targets []interface{}
	for i := 0; i < t.NumField(); i++ {
		if isRelationField(t.Field(i)) {
			continue
		}
		targets = append(targets, v.Field(i).Addr().Interface())
	}
	return targets
}

func parseRelationMetadata(tag string) (fields []string, references []string) {
	parts := strings.Split(tag, ",")
	for _, p := range parts {
		if strings.HasPrefix(p, "fields=") {
			fields = strings.Split(strings.TrimPrefix(p, "fields="), ":")
		}
		if strings.HasPrefix(p, "references=") {
			references = strings.Split(strings.TrimPrefix(p, "references="), ":")
		}
	}
	return
}

func quoteIdentifier(s string) string {

	if DBDriver == "postgres" || DBDriver == "postgresql" {
		return fmt.Sprintf("\"%s\"", s)
	}
	return fmt.Sprintf("`%s`", s)
}

func (b *DBBuilder) resolveJoinRelation(t reflect.Type, relName string) (string, error) {
	field, ok := t.FieldByName(relName)
	if !ok {
		return "", fmt.Errorf("relation %s not found", relName)
	}

	tag := field.Tag.Get("godb")
	if !strings.Contains(tag, "relation") {
		return "", fmt.Errorf("field %s is not a relation", relName)
	}

	relFields, relRefs := parseRelationMetadata(tag)
	if len(relFields) == 0 || len(relRefs) == 0 {
		return "", fmt.Errorf("relation metadata missing for %s", relName)
	}

	targetType := field.Type
	if targetType.Kind() == reflect.Slice || targetType.Kind() == reflect.Ptr {
		targetType = targetType.Elem()
	}
	targetTable := ToTableName(targetType.Name())
	sourceTable := ToTableName(t.Name())

	sourceCol := b.getColumnName(t, relFields[0])
	targetCol := b.getColumnName(targetType, relRefs[0])

	return fmt.Sprintf("INNER JOIN %s ON %s.%s = %s.%s",
		quoteIdentifier(targetTable),
		quoteIdentifier(sourceTable), quoteIdentifier(sourceCol),
		quoteIdentifier(targetTable), quoteIdentifier(targetCol)), nil
}

func (b *DBBuilder) getColumnName(t reflect.Type, fieldNameOrTag string) string {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		dbTag := f.Tag.Get("db")
		if strings.ToLower(dbTag) == strings.ToLower(fieldNameOrTag) || strings.ToLower(f.Name) == strings.ToLower(fieldNameOrTag) {
			if dbTag != "" {
				return dbTag
			}
			return ToSnakeCase(f.Name)
		}
	}
	return ToSnakeCase(fieldNameOrTag)
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
