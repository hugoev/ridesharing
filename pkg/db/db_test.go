package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Compile-time check: mockDBTX must satisfy DBTX interface.
// If DBTX changes, this will produce a build error.
var _ DBTX = (*mockDBTX)(nil)

type mockDBTX struct{}

func (m *mockDBTX) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (m *mockDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}

func (m *mockDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return nil
}

func TestDBTX_MockSatisfiesInterface(t *testing.T) {
	// This test exists to verify the compile-time interface check above.
	// If DBTX's method signatures change, `var _ DBTX = (*mockDBTX)(nil)`
	// will cause a compilation error, making this file fail to build.
	t.Log("DBTX interface contract verified at compile time")
}
