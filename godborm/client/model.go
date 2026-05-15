package client

import (
	"database/sql"
	"fmt"
	"reflect"
)

// ============================================================
// Model() — primary ORM entry point
//
// Type T is inferred automatically from the argument.
// You never write Model[User]; you write Model(&User{}).
//
// Usage:
//
//	users, err := client.Model(&User{}).Where("city = ?", "Dhaka").Find()
//	user,  err := client.Model(&User{}).FindOne(1)
//	first, err := client.Model(&User{}).OrderBy("id DESC").First()
//
//	user := &User{Name: "Alice"}
//	err  := client.Model(user).Save()   // Create (ID == 0) or Update
// ============================================================

// ModelQuery is the fluent, type-safe query builder.
// T is inferred from the pointer passed to Model().
// Every method returns *ModelQuery[T] so chains are fully immutable-style.
type ModelQuery[T any] struct {
	model *T
	b     *DBBuilder
}

// Model creates a new ModelQuery for the given model pointer.
// Go infers T from the argument — no explicit type parameter needed.
//
//	client.Model(&User{})
//	client.Model(&Post{})
func Model[T any](model *T) *ModelQuery[T] {
	return &ModelQuery[T]{
		model: model,
		b:     &DBBuilder{executor: DB},
	}
}

// ── Chainable query methods (Immutable) ──────────────────────

func (q *ModelQuery[T]) clone() *ModelQuery[T] {
	newB := *q.b // shallow copy
	// Clone slices to ensure true immutability
	if q.b.selectedCols != nil {
		newB.selectedCols = append([]string{}, q.b.selectedCols...)
	}
	if q.b.includedRelations != nil {
		newB.includedRelations = append([]string{}, q.b.includedRelations...)
	}
	if q.b.whereClauses != nil {
		newB.whereClauses = append([]whereClause{}, q.b.whereClauses...)
	}
	if q.b.joins != nil {
		newB.joins = append([]string{}, q.b.joins...)
	}
	return &ModelQuery[T]{
		model: q.model,
		b:     &newB,
	}
}

// Select restricts the columns fetched from the database.
func (q *ModelQuery[T]) Select(cols ...string) *ModelQuery[T] {
	newQ := q.clone()
	newQ.b.selectedCols = cols
	return newQ
}

// Include eager-loads the named relations.
func (q *ModelQuery[T]) Include(relations ...string) *ModelQuery[T] {
	newQ := q.clone()
	newQ.b.includedRelations = append(newQ.b.includedRelations, relations...)
	return newQ
}

// Join adds an INNER JOIN clause to the query.
func (q *ModelQuery[T]) Join(join string, on ...string) *ModelQuery[T] {
	newQ := q.clone()
	newQ.b.Join(join, on...)
	return newQ
}

// LeftJoin adds a LEFT JOIN clause to the query.
func (q *ModelQuery[T]) LeftJoin(join string, on ...string) *ModelQuery[T] {
	newQ := q.clone()
	newQ.b.LeftJoin(join, on...)
	return newQ
}

// RightJoin adds a RIGHT JOIN clause to the query.
func (q *ModelQuery[T]) RightJoin(join string, on ...string) *ModelQuery[T] {
	newQ := q.clone()
	newQ.b.RightJoin(join, on...)
	return newQ
}

// Where adds a WHERE condition. Multiple calls are AND-joined.
// Use ? for placeholders; the ORM rewrites them to $N for PostgreSQL.
func (q *ModelQuery[T]) Where(condition string, args ...interface{}) *ModelQuery[T] {
	newQ := q.clone()
	newQ.b.whereClauses = append(newQ.b.whereClauses, whereClause{condition: condition, args: args})
	return newQ
}

// OrderBy appends an ORDER BY expression. Multiple calls are comma-joined.
func (q *ModelQuery[T]) OrderBy(col string) *ModelQuery[T] {
	newQ := q.clone()
	if newQ.b.orderBy != "" {
		newQ.b.orderBy += ", " + col
	} else {
		newQ.b.orderBy = col
	}
	return newQ
}

// Limit restricts the number of returned rows.
func (q *ModelQuery[T]) Limit(n int) *ModelQuery[T] {
	newQ := q.clone()
	newQ.b.limit = n
	return newQ
}

// Offset skips the first n rows.
func (q *ModelQuery[T]) Offset(n int) *ModelQuery[T] {
	newQ := q.clone()
	newQ.b.offset = n
	return newQ
}


// ── Terminal (execution) methods ─────────────────────────────

// Find executes the query and returns all matching rows as []T.
func (q *ModelQuery[T]) Find() ([]T, error) {
	var dest []T
	if err := q.b.FindAll(&dest); err != nil {
		return nil, err
	}
	return dest, nil
}

// FindOne fetches a single record by primary key and returns *T.
func (q *ModelQuery[T]) FindOne(id interface{}) (*T, error) {
	var dest T
	if err := q.b.Find(&dest, id); err != nil {
		return nil, err
	}
	return &dest, nil
}

// First returns the first row matching the current conditions as *T.
func (q *ModelQuery[T]) First() (*T, error) {
	q.b.limit = 1
	rows, err := q.Find()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

// Count executes a COUNT(*) for the current WHERE conditions and returns the total.
func (q *ModelQuery[T]) Count() (int64, error) {
	var zero T
	t := reflect.TypeOf(zero)
	table := ToTableName(t.Name())

	sqlQuery := "SELECT COUNT(*) FROM " + quoteIdentifier(table)
	var args []interface{}
	if len(q.b.whereClauses) > 0 {
		var conditions []string
		for _, wc := range q.b.whereClauses {
			cond := wc.condition
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

	exec := q.b.executor
	if exec == nil {
		exec = DB
	}

	var count int64
	if err := exec.QueryRow(sqlQuery, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("orm: count %s: %w", table, err)
	}
	return count, nil
}

// Save inserts the model if its id field is zero, or updates it otherwise.
func (q *ModelQuery[T]) Save() error {
	v := reflect.ValueOf(q.model).Elem()
	t := v.Type()

	exec := q.b.executor
	if exec == nil {
		exec = DB
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		dbTag := f.Tag.Get("db")
		colName := dbTag
		if colName == "" {
			colName = ToSnakeCase(f.Name)
		}
		if colName == "id" {
			fv := v.Field(i)
			isZero := reflect.DeepEqual(fv.Interface(), reflect.Zero(f.Type).Interface())
			if isZero {
				return createWithExecutor(exec, q.model)
			}
			return updateWithExecutor(exec, q.model)
		}
	}

	return createWithExecutor(exec, q.model)
}

// Delete deletes the model's row using the value of its id field.
func (q *ModelQuery[T]) Delete() error {
	v := reflect.ValueOf(q.model).Elem()
	t := v.Type()

	exec := q.b.executor
	if exec == nil {
		exec = DB
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		dbTag := f.Tag.Get("db")
		colName := dbTag
		if colName == "" {
			colName = ToSnakeCase(f.Name)
		}
		if colName == "id" {
			return deleteWithExecutor(exec, q.model, v.Field(i).Interface())
		}
	}
	return fmt.Errorf("orm: delete: no id field found on %T", q.model)
}

