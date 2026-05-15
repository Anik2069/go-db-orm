package client

// ============================================================
// Top-level generic functions — the primary public API
// ============================================================

// FindAll[T] returns all rows of type T, with optional query builder options.
//
//	users, err := client.FindAll[User]()
//	users, err := client.Query[User]().Where("age > ?", 18).All()
func FindAllG[T any]() ([]T, error) {
	return Query[T]().All()
}

// First[T] returns the first row of type T matching the current conditions.
//
//	user, err := client.First[User]()
func FirstG[T any]() (*T, error) {
	return Query[T]().First()
}

// CreateG[T] inserts model into the database and populates auto-generated fields.
//
//	user := &User{Name: "Alice"}
//	err := client.CreateG(user) // user.ID is now set
func CreateG[T any](model *T) error {
	return Create(model)
}

// UpdateG[T] updates the database row for model using its id field.
//
//	err := client.UpdateG(user)
func UpdateG[T any](model *T) error {
	return Update(model)
}

// DeleteG[T] deletes the row of type T with the given primary key.
//
//	err := client.DeleteG[User](42)
func DeleteG[T any](id interface{}) error {
	var zero T
	return Delete(&zero, id)
}

// ============================================================
// TypedBuilder — chainable, generic query builder
// ============================================================

// Query returns a generic, type-safe query builder for model type T.
//
//	users, err := client.Query[User]().Where("city = ?", "Dhaka").All()
//	user,  err := client.Query[User]().Find(1)
func Query[T any]() *TypedBuilder[T] {
	return &TypedBuilder[T]{b: &DBBuilder{}}
}

// TypedBuilder is a compile-time-safe query builder.
// All methods return *TypedBuilder[T] for fluent chaining.
// Terminal methods return concrete typed values — never interface{}.
type TypedBuilder[T any] struct {
	b *DBBuilder
}

// Select restricts which columns are fetched.
func (q *TypedBuilder[T]) Select(cols ...string) *TypedBuilder[T] {
	q.b.selectedCols = cols
	return q
}

// Include eager-loads named relations.
func (q *TypedBuilder[T]) Include(relations ...string) *TypedBuilder[T] {
	q.b.includedRelations = append(q.b.includedRelations, relations...)
	return q
}

// Join adds an INNER JOIN clause to the query.
func (q *TypedBuilder[T]) Join(join string, on ...string) *TypedBuilder[T] {
	q.b.Join(join, on...)
	return q
}

// LeftJoin adds a LEFT JOIN clause to the query.
func (q *TypedBuilder[T]) LeftJoin(join string, on ...string) *TypedBuilder[T] {
	q.b.LeftJoin(join, on...)
	return q
}

// RightJoin adds a RIGHT JOIN clause to the query.
func (q *TypedBuilder[T]) RightJoin(join string, on ...string) *TypedBuilder[T] {
	q.b.RightJoin(join, on...)
	return q
}

// Where adds a WHERE condition. Multiple calls are joined with AND.
// Use ? as placeholder regardless of driver; the ORM rewrites to $N for PostgreSQL.
func (q *TypedBuilder[T]) Where(condition string, args ...interface{}) *TypedBuilder[T] {
	q.b.whereClauses = append(q.b.whereClauses, whereClause{condition: condition, args: args})
	return q
}

// OrderBy adds an ORDER BY clause. Multiple calls are comma-joined.
//
//	.OrderBy("created_at DESC").OrderBy("name ASC")
//	→ ORDER BY created_at DESC, name ASC
func (q *TypedBuilder[T]) OrderBy(col string) *TypedBuilder[T] {
	if q.b.orderBy != "" {
		q.b.orderBy += ", " + col
	} else {
		q.b.orderBy = col
	}
	return q
}

// Limit restricts the number of returned rows.
func (q *TypedBuilder[T]) Limit(n int) *TypedBuilder[T] {
	q.b.limit = n
	return q
}

// Offset skips the first n rows.
func (q *TypedBuilder[T]) Offset(n int) *TypedBuilder[T] {
	q.b.offset = n
	return q
}

// All executes the query and returns a typed []T — no interface{}, no casting.
func (q *TypedBuilder[T]) All() ([]T, error) {
	var dest []T
	if err := q.b.FindAll(&dest); err != nil {
		return nil, err
	}
	return dest, nil
}

// Find retrieves a single record by primary key and returns *T.
// Returns (nil, sql.ErrNoRows) if not found.
func (q *TypedBuilder[T]) Find(id interface{}) (*T, error) {
	var dest T
	if err := q.b.Find(&dest, id); err != nil {
		return nil, err
	}
	return &dest, nil
}

// First returns the first matching row as *T, or nil if no rows match.
func (q *TypedBuilder[T]) First() (*T, error) {
	q.b.limit = 1
	results, err := q.All()
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return &results[0], nil
}

// ============================================================
// Typed Tx helpers — transactional generic CRUD
// ============================================================

// CreateT is the generic version of Tx.Create.
//
//	err := client.CreateT(tx, &user)
func CreateT[T any](tx *Tx, model *T) error {
	return tx.Create(model)
}

// UpdateT is the generic version of Tx.Update.
func UpdateT[T any](tx *Tx, model *T) error {
	return tx.Update(model)
}

// DeleteT is the generic version of Tx.Delete.
func DeleteT[T any](tx *Tx, id interface{}) error {
	var zero T
	return tx.Delete(&zero, id)
}
