// Package events owns event ingestion: HTTP handler, repository, validation.
// Keeps Postgres-facing code (SQL, pgx types) inside the repository so the
// rest of the codebase deals with plain Go structs.
package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrDuplicateEvent = errors.New("duplicate event")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

type Event struct {
	ID         uuid.UUID
	UserID     string
	EventType  string
	Properties json.RawMessage
	CreatedAt  time.Time
}

const insertSQL = `
INSERT INTO events (id, user_id, event_type, properties, created_at)
VALUES ($1, $2, $3, $4, $5)
`

// Insert writes a single event. In week 5 we'll add a batched variant using
// pgx.Batch / COPY FROM that ingest-api will use behind a small in-memory
// buffer to amortize per-row overhead.
func (r *Repository) Insert(ctx context.Context, e Event) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	if len(e.Properties) == 0 {
		e.Properties = json.RawMessage(`{}`)
	}

	_, err := r.pool.Exec(ctx, insertSQL,
		e.ID, e.UserID, e.EventType, e.Properties, e.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("%w: %s", ErrDuplicateEvent, pgErr.ConstraintName)
		}
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}
