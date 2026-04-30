// Package partitions plans and applies partition maintenance on the events
// table: creating future daily partitions and (later) dropping old ones to
// enforce retention.
//
// The planner is pure: given the current time, the policy, and the existing
// partition list, it returns the list of CREATE/DROP actions to execute. The
// applier (maintainer.go) executes them against PostgreSQL. This split is
// what lets us unit-test all the date arithmetic without spinning up a
// database.
package partitions

import (
	"fmt"
	"sort"
	"time"
)

// Config is the maintenance policy.
type Config struct {
	// FutureDays is how many daily partitions ahead we keep ready, counting
	// today. 14 means today + 13 future days exist after each tick.
	FutureDays int

	// RetentionDays is how long to keep old partitions before dropping. 0
	// disables drops entirely. Drops get turned on in week 5.
	RetentionDays int

	// TableName is the parent partitioned table.
	TableName string
}

// Op is the kind of maintenance action.
type Op int

const (
	OpCreate Op = iota
	OpDrop
)

// Action is a single piece of work the maintainer needs to execute.
type Action struct {
	Op   Op
	Name string    // partition table name, e.g. "events_20260501"
	From time.Time // inclusive (UTC midnight)
	To   time.Time // exclusive (UTC midnight)
}

// Plan computes the set of CREATE/DROP actions needed to bring `existing`
// in line with `cfg` at time `now`.
//
// `existing` is the list of partition table names already attached to the
// parent. Plan trusts that names follow `<table>_YYYYMMDD`; anything else is
// ignored (so we never drop a partition we don't recognize).
func Plan(now time.Time, cfg Config, existing []string) []Action {
	now = now.UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	have := make(map[string]struct{}, len(existing))
	for _, n := range existing {
		have[n] = struct{}{}
	}

	var out []Action

	// Creates: today through today+FutureDays-1.
	for i := 0; i < cfg.FutureDays; i++ {
		from := today.AddDate(0, 0, i)
		name := PartitionName(cfg.TableName, from)
		if _, ok := have[name]; ok {
			continue
		}
		out = append(out, Action{
			Op:   OpCreate,
			Name: name,
			From: from,
			To:   from.AddDate(0, 0, 1),
		})
	}

	// Drops: anything older than today-RetentionDays.
	if cfg.RetentionDays > 0 {
		threshold := today.AddDate(0, 0, -cfg.RetentionDays)
		for name := range have {
			d, ok := parsePartitionDate(cfg.TableName, name)
			if !ok {
				continue
			}
			if d.Before(threshold) {
				out = append(out, Action{
					Op:   OpDrop,
					Name: name,
					From: d,
					To:   d.AddDate(0, 0, 1),
				})
			}
		}
	}

	// Stable order: creates before drops, lexicographic within. Helps tests
	// and makes log output less confusing during incidents.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Op != out[j].Op {
			return out[i].Op < out[j].Op
		}
		return out[i].Name < out[j].Name
	})

	return out
}

// PartitionName returns the canonical partition name for a given table+date.
func PartitionName(table string, day time.Time) string {
	return fmt.Sprintf("%s_%s", table, day.UTC().Format("20060102"))
}

func parsePartitionDate(table, name string) (time.Time, bool) {
	prefix := table + "_"
	if len(name) != len(prefix)+8 || name[:len(prefix)] != prefix {
		return time.Time{}, false
	}
	t, err := time.Parse("20060102", name[len(prefix):])
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}
