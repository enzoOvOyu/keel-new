package cmdb

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// One projection owns one transaction; counters never escape its invocation.
type engineeringSQLCounter struct {
	pgx.Tx
	Statements int
}

func (t *engineeringSQLCounter) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	t.Statements++
	return t.Tx.Exec(ctx, sql, args...)
}
func (t *engineeringSQLCounter) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	t.Statements++
	return t.Tx.Query(ctx, sql, args...)
}
func (t *engineeringSQLCounter) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	t.Statements++
	return t.Tx.QueryRow(ctx, sql, args...)
}
func (t *engineeringSQLCounter) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	t.Statements += b.Len()
	return t.Tx.SendBatch(ctx, b)
}
func (t *engineeringSQLCounter) CopyFrom(ctx context.Context, table pgx.Identifier, columns []string, rows pgx.CopyFromSource) (int64, error) {
	t.Statements++
	return t.Tx.CopyFrom(ctx, table, columns, rows)
}
