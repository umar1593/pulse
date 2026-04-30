//go:build integration

package partitions

import (
	"context"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMaintainerTick_CreatesFuturePartitionsIdempotently(t *testing.T) {
	dsn := os.Getenv("PULSE_TEST_DSN")
	if dsn == "" {
		t.Skip("PULSE_TEST_DSN is not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	t.Cleanup(pool.Close)

	tableName := fmt.Sprintf("test_events_%d", time.Now().UnixNano())
	createParent := fmt.Sprintf(`
CREATE TABLE %s (
    id         bigserial,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at)
`, tableName)
	if _, err := pool.Exec(ctx, createParent); err != nil {
		t.Fatalf("create parent table: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", tableName)); err != nil {
			t.Errorf("drop parent table: %v", err)
		}
	})

	fixedNow := time.Date(2026, time.May, 1, 13, 0, 0, 0, time.UTC)
	m := NewMaintainerWithClock(pool, Config{
		TableName:  tableName,
		FutureDays: 5,
	}, func() time.Time {
		return fixedNow
	})

	if err := m.Tick(ctx); err != nil {
		t.Fatalf("first tick: %v", err)
	}

	got, err := listPartitions(ctx, pool, tableName)
	if err != nil {
		t.Fatalf("list partitions after first tick: %v", err)
	}

	want := []string{
		PartitionName(tableName, fixedNow),
		PartitionName(tableName, fixedNow.AddDate(0, 0, 1)),
		PartitionName(tableName, fixedNow.AddDate(0, 0, 2)),
		PartitionName(tableName, fixedNow.AddDate(0, 0, 3)),
		PartitionName(tableName, fixedNow.AddDate(0, 0, 4)),
	}
	if !slices.Equal(got, want) {
		t.Fatalf("partitions after first tick: got %v want %v", got, want)
	}

	if err := m.Tick(ctx); err != nil {
		t.Fatalf("second tick: %v", err)
	}

	gotAgain, err := listPartitions(ctx, pool, tableName)
	if err != nil {
		t.Fatalf("list partitions after second tick: %v", err)
	}
	if !slices.Equal(gotAgain, want) {
		t.Fatalf("partitions after second tick: got %v want %v", gotAgain, want)
	}
}

func listPartitions(ctx context.Context, pool *pgxpool.Pool, tableName string) ([]string, error) {
	const q = `
SELECT child.relname
FROM   pg_inherits
JOIN   pg_class parent ON pg_inherits.inhparent = parent.oid
JOIN   pg_class child  ON pg_inherits.inhrelid  = child.oid
WHERE  parent.relname = $1
ORDER BY child.relname
`
	rows, err := pool.Query(ctx, q, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}
