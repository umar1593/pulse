// partition-worker runs the partition Maintainer on a ticker. It is its own
// process so we can scale it independently and so a bug in maintenance can
// never take down the ingest path. In a small deployment it could equally
// run as a goroutine inside ingest-api — the trade-off is operational
// (fewer processes) versus blast radius. We chose the separate-process route.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/umar1593/pulse/internal/config"
	"github.com/umar1593/pulse/internal/db"
	"github.com/umar1593/pulse/internal/partitions"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	if err := run(context.Background()); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(parent context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DB)
	if err != nil {
		return err
	}
	defer pool.Close()

	m := partitions.NewMaintainer(pool, partitions.Config{
		FutureDays:    14,
		RetentionDays: 0, // turned on in week 5 once retention policy is decided
	})

	slog.Info("partition-worker starting",
		"interval", time.Hour.String(),
		"future_days", 14,
	)
	return m.Run(ctx, time.Hour)
}
