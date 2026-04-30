package partitions

import (
	"testing"
	"time"
)

func ts(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t.UTC()
}

func TestPlan_CreatesMissingFutureDays(t *testing.T) {
	now := ts("2026-05-01").Add(10 * time.Hour) // mid-day
	cfg := Config{FutureDays: 3, TableName: "events"}

	got := Plan(now, cfg, nil)

	want := []string{"events_20260501", "events_20260502", "events_20260503"}
	if len(got) != len(want) {
		t.Fatalf("got %d actions, want %d: %+v", len(got), len(want), got)
	}
	for i, a := range got {
		if a.Op != OpCreate || a.Name != want[i] {
			t.Errorf("action %d: got %+v want create %s", i, a, want[i])
		}
	}
}

func TestPlan_SkipsExistingPartitions(t *testing.T) {
	now := ts("2026-05-01")
	cfg := Config{FutureDays: 3, TableName: "events"}

	existing := []string{"events_20260501", "events_20260502"}
	got := Plan(now, cfg, existing)

	if len(got) != 1 {
		t.Fatalf("want 1 create, got %d: %+v", len(got), got)
	}
	if got[0].Name != "events_20260503" {
		t.Errorf("expected events_20260503, got %s", got[0].Name)
	}
}

func TestPlan_DropsOldWhenRetentionSet(t *testing.T) {
	now := ts("2026-05-10")
	cfg := Config{FutureDays: 1, RetentionDays: 7, TableName: "events"}

	existing := []string{
		"events_20260501", // 9 days old → drop
		"events_20260502", // 8 days old → drop
		"events_20260503", // 7 days old → keep (boundary inclusive)
		"events_20260510", // today → keep
	}
	got := Plan(now, cfg, existing)

	var drops, creates int
	for _, a := range got {
		switch a.Op {
		case OpCreate:
			creates++
		case OpDrop:
			drops++
		}
	}
	if drops != 2 || creates != 0 {
		t.Fatalf("want 2 drops, 0 creates; got drops=%d creates=%d (%+v)", drops, creates, got)
	}
}

func TestPlan_IgnoresUnknownPartitionNames(t *testing.T) {
	now := ts("2026-05-10")
	cfg := Config{FutureDays: 1, RetentionDays: 1, TableName: "events"}

	existing := []string{
		"events_default",
		"events_archive_20200101",
		"events_20260510",
	}
	got := Plan(now, cfg, existing)
	for _, a := range got {
		if a.Op == OpDrop {
			t.Fatalf("unexpected drop of unknown-shape partition: %+v", a)
		}
	}
}
