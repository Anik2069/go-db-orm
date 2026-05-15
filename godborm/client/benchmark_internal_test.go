package client

import (
	"database/sql"
	"reflect"
	"testing"
)

// User is a sample struct for benchmarking
type BenchUser struct {
	ID    int    `db:"id"`
	Name  string `db:"name"`
	Email string `db:"email"`
	Age   int    `db:"age"`
}

// simulateRows is a mock that mimics what sql.Rows.Scan does
type mockScanner struct {
	values []interface{}
}

func (m *mockScanner) Scan(dest ...interface{}) error {
	for i := range dest {
		switch v := dest[i].(type) {
		case *sql.NullInt64:
			v.Int64 = 1
			v.Valid = true
		case *sql.NullString:
			v.String = "test"
			v.Valid = true
		case *interface{}:
			// discard
		default:
			// Direct scan
			rv := reflect.ValueOf(v).Elem()
			if rv.Kind() == reflect.Int {
				rv.SetInt(1)
			} else if rv.Kind() == reflect.String {
				rv.SetString("test")
			}
		}
	}
	return nil
}

func BenchmarkManualScan(b *testing.B) {
	mock := &mockScanner{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var u BenchUser
		_ = mock.Scan(&u.ID, &u.Name, &u.Email, &u.Age)
	}
}

func BenchmarkGoDBORMScanner(b *testing.B) {
	// We benchmark the internal logic of scanning a row into a struct
	// This measures the overhead of reflection-based mapping and NULL handling
	mock := &mockScanner{}
	u := BenchUser{}
	v := reflect.ValueOf(&u).Elem()
	
	// Pre-calculated targets to simulate the optimized path
	targets := scanStructTargets(v)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mock.Scan(targets...)
		_ = applyNullableTargets(v, targets)
	}
}

func BenchmarkFullInternalMapping(b *testing.B) {
	// This measures the overhead of calculating targets every time (what happens in FindAll)
	mock := &mockScanner{}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var u BenchUser
		v := reflect.ValueOf(&u).Elem()
		targets := scanStructTargets(v)
		_ = mock.Scan(targets...)
		_ = applyNullableTargets(v, targets)
	}
}
