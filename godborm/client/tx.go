package client

import (
	"database/sql"
	"fmt"
)

// Tx is a type-safe transaction wrapper.
// Use client.Begin() to start, then Commit() or Rollback().
// A deferred Rollback() after Commit() is safe — it becomes a no-op.
type Tx struct {
	tx *sql.Tx
}

// Begin starts a new database transaction.
//
// Recommended usage pattern:
//
//	tx, err := client.Begin()
//	if err != nil { ... }
//	defer tx.Rollback() // safe no-op after Commit
//
//	tx.Create(&user)
//	tx.Update(&post)
//
//	if err := tx.Commit(); err != nil { ... }
func Begin() (*Tx, error) {
	tx, err := DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("orm: begin transaction failed: %w", err)
	}
	return &Tx{tx: tx}, nil
}

// Commit commits the transaction.
func (t *Tx) Commit() error {
	if err := t.tx.Commit(); err != nil {
		return fmt.Errorf("orm: commit failed: %w", err)
	}
	return nil
}

// Rollback rolls back the transaction. Safe to call even after Commit (becomes a no-op).
func (t *Tx) Rollback() error {
	err := t.tx.Rollback()
	if err != nil && err != sql.ErrTxDone {
		return fmt.Errorf("orm: rollback failed: %w", err)
	}
	return nil
}

// Create inserts a record inside the transaction.
func (t *Tx) Create(model interface{}) error {
	return createWithExecutor(t.tx, model)
}

// Update updates a record inside the transaction.
func (t *Tx) Update(model interface{}) error {
	return updateWithExecutor(t.tx, model)
}

// Delete deletes a record by ID inside the transaction.
func (t *Tx) Delete(model interface{}, id interface{}) error {
	return deleteWithExecutor(t.tx, model, id)
}

// Exec runs a raw SQL statement inside the transaction.
func (t *Tx) Exec(query string, args ...interface{}) error {
	_, err := t.tx.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("orm: tx.Exec failed: %w", err)
	}
	return nil
}

// WithTx runs fn inside a transaction, automatically committing on success
// and rolling back on any error or panic.
//
// Example:
//
//	err := client.WithTx(func(tx *client.Tx) error {
//	    tx.Create(&user)
//	    tx.Update(&post)
//	    return nil
//	})
func WithTx(fn func(tx *Tx) error) (err error) {
	tx, err := Begin()
	if err != nil {
		return err
	}

	// Rollback on panic — restores DB integrity
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p) // re-panic after rollback
		}
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	err = fn(tx)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Model starts a type-safe query inside the transaction.
func (t *Tx) Model(model interface{}) *ModelQueryProxy {
	return &ModelQueryProxy{
		tx:    t,
		model: model,
	}
}

// sqlExecutor is a common interface for *sql.DB and *sql.Tx so internal helpers
// can operate in both transactional and non-transactional contexts.
type sqlExecutor interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

// ModelQueryProxy is a helper to allow tx.Model(&User{}) syntax while maintaining generics.
// Since Go doesn't support generic methods on structs yet (like tx.Model[T]),
// we use this proxy or top-level functions.
type ModelQueryProxy struct {
	tx    *Tx
	model interface{}
}

