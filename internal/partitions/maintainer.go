package partitions

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Maintainer applies partition maintenance against PostgreSQL on a schedule.
type Maintainer struct {
	pool *pgxpool.Pool
	cfg  Config
	now  func() time.Time // injectable for tests
}

func NewMaintainer(pool *pgxpool.Pool, cfg Config) *Maintainer {
	return NewMaintainerWithClock(pool, cfg, func() time.Time { return time.Now().UTC() })
}

func NewMaintainerWithClock(pool *pgxpool.Pool, cfg Config, clock func() time.Time) *Maintainer {
	if cfg.TableName == "" {
		cfg.TableName = "events"
	}
	if cfg.FutureDays == 0 {
		cfg.FutureDays = 14
	}
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	return &Maintainer{
		pool: pool,
		cfg:  cfg,
		now:  clock,
	}
}

// Tick runs one maintenance pass.
func (m *Maintainer) Tick(ctx context.Context) error {
	existing, err := m.listExisting(ctx)
	if err != nil {
		return fmt.Errorf("list partitions: %w", err)
	}

	plan := Plan(m.now(), m.cfg, existing)
	if len(plan) == 0 {
		return nil
	}

	for _, a := range plan {
		if err := m.apply(ctx, a); err != nil {
			return fmt.Errorf("apply %s %s: %w", opString(a.Op), a.Name, err)
		}
		slog.Info("partition maintenance",
			"op", opString(a.Op),
			"name", a.Name,
			"from", a.From.Format(time.RFC3339),
			"to", a.To.Format(time.RFC3339),
		)
	}
	return nil
}

// Run loops Tick on `interval` until ctx is cancelled.
func (m *Maintainer) Run(ctx context.Context, interval time.Duration) error {
	// Run once immediately so a freshly-started service does not wait an
	// entire interval before its first tick.
	if err := m.Tick(ctx); err != nil {
		slog.Error("partition tick failed", "err", err)
	}

	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			if err := m.Tick(ctx); err != nil {
				slog.Error("partition tick failed", "err", err)
			}
		}
	}
}

// listExisting returns the names of all partitions currently attached to
// the parent table. It uses pg_inherits, which is the canonical way to
// enumerate partitions in PostgreSQL.
func (m *Maintainer) listExisting(ctx context.Context) ([]string, error) {
	const q = `
SELECT child.relname
FROM   pg_inherits
JOIN   pg_class parent ON pg_inherits.inhparent = parent.oid
JOIN   pg_class child  ON pg_inherits.inhrelid  = child.oid
WHERE  parent.relname = $1
`
	rows, err := m.pool.Query(ctx, q, m.cfg.TableName)
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

// apply executes a single CREATE or DROP. We cannot use bind parameters for
// identifiers in DDL, so we let PostgreSQL's format() compose the statement
// safely (%I quotes identifiers, %L quotes literals). Two round trips per
// action — fine for a job that runs hourly.
func (m *Maintainer) apply(ctx context.Context, a Action) error {
	switch a.Op {
	case OpCreate:
		const buildQ = `
SELECT format(
    'CREATE TABLE IF NOT EXISTS %I PARTITION OF %I FOR VALUES FROM (%L) TO (%L)',
    $1::text, $2::text, $3::timestamptz, $4::timestamptz
)`
		var stmt string
		if err := m.pool.QueryRow(ctx, buildQ,
			a.Name, m.cfg.TableName, a.From, a.To,
		).Scan(&stmt); err != nil {
			return err
		}
		_, err := m.pool.Exec(ctx, stmt)
		return err

	case OpDrop:
		const buildQ = `SELECT format('DROP TABLE IF EXISTS %I', $1::text)`
		var stmt string
		if err := m.pool.QueryRow(ctx, buildQ, a.Name).Scan(&stmt); err != nil {
			return err
		}
		_, err := m.pool.Exec(ctx, stmt)
		return err
	}
	return fmt.Errorf("unknown op %v", a.Op)
}

func opString(op Op) string {
	switch op {
	case OpCreate:
		return "create"
	case OpDrop:
		return "drop"
	default:
		return "unknown"
	}
}
