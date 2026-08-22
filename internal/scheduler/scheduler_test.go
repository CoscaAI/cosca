//
// Tests for the scheduler package (internal/scheduler/):
//   - ParseNL: interval/daily/cron em linguagem natural (pt-BR/EN) + inválidos
//   - NextAfter: interval, daily através da meia-noite, cron (básico, estrito,
//     sem ocorrência em 2 anos → erro)
//   - Schedule.Validate / CronLimitations
//   - Store: Add/Get/List/Remove/SetEnabled/UpdateNextRun/RecordRun + persistência
//   - Runner: tick executa jobs vencidos (comando fake), pula não-vencidos,
//     job com erro não derruba o runner
//
// A base SQLite vive em t.TempDir() — nunca no .cosca real. Nenhum daemon é
// iniciado: os testes chamam Runner.tick() diretamente.

package scheduler

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// =============================================================================
// ParseNL
// =============================================================================

func TestParseNL_Interval(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"every 30m", 30 * time.Minute},
		{"every 2h", 2 * time.Hour},
		{"every 15s", 15 * time.Second},
		{"every 5m30s", 5*time.Minute + 30*time.Second},
		{"a cada 30m", 30 * time.Minute},
		{"a cada 30 minutos", 30 * time.Minute},
		{"30m", 30 * time.Minute},
		{"2 hours", 2 * time.Hour},
		{"1 day", 24 * time.Hour},
		{"2 horas", 2 * time.Hour},
	}
	for _, c := range cases {
		sch, err := ParseNL(c.in)
		if err != nil {
			t.Errorf("ParseNL(%q): unexpected error: %v", c.in, err)
			continue
		}
		if sch.Kind != KindInterval {
			t.Errorf("ParseNL(%q) kind = %q, want %q", c.in, sch.Kind, KindInterval)
		}
		if sch.Interval != c.want {
			t.Errorf("ParseNL(%q) interval = %s, want %s", c.in, sch.Interval, c.want)
		}
		if sch.Raw != c.in {
			t.Errorf("ParseNL(%q) raw = %q, want %q", c.in, sch.Raw, c.in)
		}
	}
}

func TestParseNL_Daily(t *testing.T) {
	cases := []struct {
		in      string
		wantH   int
		wantMin int
	}{
		{"daily at 9am", 9, 0},
		{"daily at 9:30am", 9, 30},
		{"daily at 12pm", 12, 0},
		{"daily at 12am", 0, 0},
		{"daily at 9pm", 21, 0},
		{"diariamente às 9:00", 9, 0},
		{"diariamente às 17:45", 17, 45},
		{"every day at 08:30", 8, 30},
		{"todos os dias às 6h", 6, 0},
		{"todo dia às 17h45", 17, 45},
	}
	for _, c := range cases {
		sch, err := ParseNL(c.in)
		if err != nil {
			t.Errorf("ParseNL(%q): unexpected error: %v", c.in, err)
			continue
		}
		if sch.Kind != KindDaily {
			t.Errorf("ParseNL(%q) kind = %q, want %q", c.in, sch.Kind, KindDaily)
		}
		if sch.Hour != c.wantH || sch.Minute != c.wantMin {
			t.Errorf("ParseNL(%q) = %02d:%02d, want %02d:%02d",
				c.in, sch.Hour, sch.Minute, c.wantH, c.wantMin)
		}
	}
}

func TestParseNL_Cron(t *testing.T) {
	cases := []string{
		"0 9 * * *",
		"*/15 * * * *",
		"30 9 1,15 * 1-5",
		"0 0 */2 * *",
		"0 22 * * 0,6",
	}
	for _, in := range cases {
		sch, err := ParseNL(in)
		if err != nil {
			t.Errorf("ParseNL(%q): unexpected error: %v", in, err)
			continue
		}
		if sch.Kind != KindCron {
			t.Errorf("ParseNL(%q) kind = %q, want %q", in, sch.Kind, KindCron)
		}
		if sch.Cron != in {
			t.Errorf("ParseNL(%q) cron = %q, want %q", in, sch.Cron, in)
		}
	}
}

func TestParseNL_Invalid(t *testing.T) {
	cases := []string{
		"",
		"banana",
		"daily at 25:00",
		"daily at 99am",
		"every",
		"0 9 * *",
		"every day at foo",
		"a cada",
	}
	for _, in := range cases {
		if _, err := ParseNL(in); err == nil {
			t.Errorf("ParseNL(%q) expected error, got nil", in)
		}
	}
}

func TestParseNL_ErrorsArePtBR(t *testing.T) {
	if _, err := ParseNL("banana"); err == nil || !strings.Contains(err.Error(), "agenda") {
		t.Errorf("expected clear pt-BR error mentioning agenda, got: %v", err)
	}
	if _, err := ParseNL("daily at 25:00"); err == nil || !strings.Contains(err.Error(), "hora") {
		t.Errorf("expected error mentioning hora, got: %v", err)
	}
}

// =============================================================================
// NextAfter
// =============================================================================

func TestNextAfter_Interval(t *testing.T) {
	sch, err := ParseNL("every 30m")
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	next, err := sch.NextAfter(base)
	if err != nil {
		t.Fatal(err)
	}
	want := base.Add(30 * time.Minute)
	if !next.Equal(want) {
		t.Errorf("NextAfter = %v, want %v", next, want)
	}
	if !next.After(base) {
		t.Error("next must be strictly after t")
	}
}

func TestNextAfter_Daily(t *testing.T) {
	sch, err := ParseNL("daily at 9am")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("later same day", func(t *testing.T) {
		base := time.Date(2026, 8, 2, 8, 0, 0, 0, time.UTC)
		next, _ := sch.NextAfter(base)
		want := time.Date(2026, 8, 2, 9, 0, 0, 0, time.UTC)
		if !next.Equal(want) {
			t.Errorf("next = %v, want %v", next, want)
		}
	})

	t.Run("across midnight", func(t *testing.T) {
		base := time.Date(2026, 8, 2, 23, 0, 0, 0, time.UTC)
		next, _ := sch.NextAfter(base)
		want := time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC)
		if !next.Equal(want) {
			t.Errorf("next = %v, want %v", next, want)
		}
	})

	t.Run("strictly after exact time", func(t *testing.T) {
		base := time.Date(2026, 8, 2, 9, 0, 0, 0, time.UTC)
		next, _ := sch.NextAfter(base)
		want := time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC)
		if !next.Equal(want) {
			t.Errorf("next = %v, want %v", next, want)
		}
	})
}

func TestNextAfter_Cron(t *testing.T) {
	t.Run("same day", func(t *testing.T) {
		sch, err := ParseNL("0 9 * * *")
		if err != nil {
			t.Fatal(err)
		}
		base := time.Date(2026, 8, 2, 8, 0, 0, 0, time.UTC)
		next, err := sch.NextAfter(base)
		if err != nil {
			t.Fatal(err)
		}
		want := time.Date(2026, 8, 2, 9, 0, 0, 0, time.UTC)
		if !next.Equal(want) {
			t.Errorf("next = %v, want %v", next, want)
		}
	})

	t.Run("next day", func(t *testing.T) {
		sch, _ := ParseNL("0 9 * * *")
		base := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
		next, _ := sch.NextAfter(base)
		want := time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC)
		if !next.Equal(want) {
			t.Errorf("next = %v, want %v", next, want)
		}
	})

	t.Run("strictly after exact minute", func(t *testing.T) {
		sch, _ := ParseNL("0 9 * * *")
		base := time.Date(2026, 8, 2, 9, 0, 0, 0, time.UTC)
		next, _ := sch.NextAfter(base)
		want := time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC)
		if !next.Equal(want) {
			t.Errorf("next = %v, want %v", next, want)
		}
	})

	t.Run("step minutes", func(t *testing.T) {
		sch, _ := ParseNL("*/15 * * * *")
		base := time.Date(2026, 8, 2, 10, 3, 0, 0, time.UTC)
		next, _ := sch.NextAfter(base)
		want := time.Date(2026, 8, 2, 10, 15, 0, 0, time.UTC)
		if !next.Equal(want) {
			t.Errorf("next = %v, want %v", next, want)
		}
	})

	t.Run("dow 7 is sunday", func(t *testing.T) {
		// 2026-08-02 é domingo (time.Weekday() == 0).
		sch, err := ParseNL("0 12 * * 7")
		if err != nil {
			t.Fatal(err)
		}
		base := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
		next, err := sch.NextAfter(base)
		if err != nil {
			t.Fatal(err)
		}
		if next.Day() != 2 {
			t.Errorf("expected sunday 2026-08-02, got %v", next)
		}
	})

	t.Run("never matches", func(t *testing.T) {
		// 31 de fevereiro não existe.
		sch, err := ParseNL("0 0 31 2 *")
		if err != nil {
			t.Fatal(err)
		}
		base := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
		if _, err := sch.NextAfter(base); err == nil {
			t.Error("expected error for cron that never matches within 2 years")
		}
	})
}

// =============================================================================
// Validate / CronLimitations
// =============================================================================

func TestSchedule_Validate(t *testing.T) {
	valid, _ := ParseNL("every 30m")
	if err := valid.Validate(); err != nil {
		t.Errorf("interval validate: %v", err)
	}
	daily, _ := ParseNL("daily at 9am")
	if err := daily.Validate(); err != nil {
		t.Errorf("daily validate: %v", err)
	}
	cron, _ := ParseNL("0 9 * * *")
	if err := cron.Validate(); err != nil {
		t.Errorf("cron validate: %v", err)
	}

	bad := &Schedule{Kind: KindInterval, Interval: 0}
	if err := bad.Validate(); err == nil {
		t.Error("expected error for zero interval")
	}
	bad = &Schedule{Kind: KindDaily, Hour: 25}
	if err := bad.Validate(); err == nil {
		t.Error("expected error for invalid daily hour")
	}
	bad = &Schedule{Kind: "weird"}
	if err := bad.Validate(); err == nil {
		t.Error("expected error for unknown kind")
	}
}

func TestCronLimitations_Documented(t *testing.T) {
	doc := CronLimitations()
	if !strings.Contains(doc, "5 campos") {
		t.Error("limitations doc must mention the 5-field subset")
	}
	if !strings.Contains(doc, "NÃO suportado") {
		t.Error("limitations doc must list what is not supported")
	}
}

// =============================================================================
// Store
// =============================================================================

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(filepath.Join(t.TempDir(), "cron.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func mustSchedule(t *testing.T, in string) Schedule {
	t.Helper()
	sch, err := ParseNL(in)
	if err != nil {
		t.Fatalf("ParseNL(%q): %v", in, err)
	}
	return *sch
}

func TestStore_AddAssignsIDAndNextRun(t *testing.T) {
	store := newTestStore(t)

	id, err := store.Add(Job{Name: "backup", Command: "true", Schedule: mustSchedule(t, "every 30m"), Enabled: true})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if id != "C-0001" {
		t.Errorf("first id = %q, want C-0001", id)
	}

	job, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if job == nil {
		t.Fatal("Get returned nil")
	}
	if job.NextRun.IsZero() {
		t.Error("Add must compute NextRun")
	}
	if !job.NextRun.After(time.Now()) {
		t.Errorf("NextRun should be in the future, got %v", job.NextRun)
	}
	if job.Runs != 0 || !job.Enabled {
		t.Errorf("unexpected defaults: runs=%d enabled=%v", job.Runs, job.Enabled)
	}
}

func TestStore_SequentialIDs(t *testing.T) {
	store := newTestStore(t)
	id1, _ := store.Add(Job{Name: "a", Command: "true", Schedule: mustSchedule(t, "every 30m"), Enabled: true})
	id2, _ := store.Add(Job{Name: "b", Command: "true", Schedule: mustSchedule(t, "every 2h"), Enabled: true})
	if id1 != "C-0001" || id2 != "C-0002" {
		t.Errorf("ids = %q, %q; want C-0001, C-0002", id1, id2)
	}
}

func TestStore_AddValidation(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.Add(Job{Name: "", Command: "true", Schedule: mustSchedule(t, "every 30m")}); err == nil {
		t.Error("expected error for empty name")
	}
	if _, err := store.Add(Job{Name: "x", Command: "", Schedule: mustSchedule(t, "every 30m")}); err == nil {
		t.Error("expected error for empty command")
	}
	bad := &Schedule{Kind: "weird"}
	if _, err := store.Add(Job{Name: "x", Command: "true", Schedule: *bad}); err == nil {
		t.Error("expected error for invalid schedule")
	}
}

func TestStore_GetNotFound(t *testing.T) {
	store := newTestStore(t)
	job, err := store.Get("C-9999")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if job != nil {
		t.Errorf("expected nil for missing job, got %+v", job)
	}
}

func TestStore_ListAndRemove(t *testing.T) {
	store := newTestStore(t)
	_, _ = store.Add(Job{Name: "a", Command: "true", Schedule: mustSchedule(t, "every 30m"), Enabled: true})
	_, _ = store.Add(Job{Name: "b", Command: "true", Schedule: mustSchedule(t, "daily at 9am"), Enabled: true})

	jobs, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("List len = %d, want 2", len(jobs))
	}
	if jobs[0].ID != "C-0001" || jobs[1].ID != "C-0002" {
		t.Errorf("List ordering = %s, %s; want C-0001, C-0002", jobs[0].ID, jobs[1].ID)
	}

	if err := store.Remove("C-0001"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	jobs, _ = store.List()
	if len(jobs) != 1 {
		t.Fatalf("List after remove len = %d, want 1", len(jobs))
	}
	if err := store.Remove("C-0001"); err == nil {
		t.Error("removing a missing job should error")
	}
	if err := store.Remove("banana"); err == nil {
		t.Error("removing an invalid id should error")
	}
}

func TestStore_SetEnabled(t *testing.T) {
	store := newTestStore(t)
	id, _ := store.Add(Job{Name: "a", Command: "true", Schedule: mustSchedule(t, "every 30m"), Enabled: true})

	if err := store.SetEnabled(id, false); err != nil {
		t.Fatalf("SetEnabled(false): %v", err)
	}
	job, _ := store.Get(id)
	if job.Enabled {
		t.Error("job should be disabled after SetEnabled(false)")
	}

	if err := store.SetEnabled(id, true); err != nil {
		t.Fatalf("SetEnabled(true): %v", err)
	}
	job, _ = store.Get(id)
	if !job.Enabled {
		t.Error("job should be enabled after SetEnabled(true)")
	}
}

func TestStore_UpdateNextRunAndRecordRun(t *testing.T) {
	store := newTestStore(t)
	id, _ := store.Add(Job{Name: "a", Command: "true", Schedule: mustSchedule(t, "every 30m"), Enabled: true})

	future := time.Now().Add(2 * time.Hour)
	if err := store.UpdateNextRun(id, future); err != nil {
		t.Fatalf("UpdateNextRun: %v", err)
	}
	job, _ := store.Get(id)
	if !job.NextRun.Equal(future.UTC()) {
		t.Errorf("NextRun = %v, want %v", job.NextRun, future.UTC())
	}

	last := time.Now().Add(-time.Minute)
	next := time.Now().Add(30 * time.Minute)
	if err := store.RecordRun(id, last, next, 3); err != nil {
		t.Fatalf("RecordRun: %v", err)
	}
	job, _ = store.Get(id)
	if job.Runs != 3 || job.LastRun.IsZero() {
		t.Errorf("after RecordRun: runs=%d lastRun=%v", job.Runs, job.LastRun)
	}
}

func TestStore_PersistenceAcrossReopen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "cron.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	id, err := store.Add(Job{Name: "persist", Command: "true", Schedule: mustSchedule(t, "every 30m"), Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store2, err := NewStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store2.Close() }()

	job, err := store2.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if job == nil || job.Name != "persist" || !job.Enabled {
		t.Errorf("persisted job not recovered: %+v", job)
	}
}

func TestNormalizeID(t *testing.T) {
	for in, want := range map[string]string{
		"C-0001": "C-0001", "c-1": "C-0001", "C-42": "C-0042", "42": "C-0042",
	} {
		got, err := NormalizeID(in)
		if err != nil {
			t.Errorf("NormalizeID(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("NormalizeID(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := NormalizeID("banana"); err == nil {
		t.Error("expected error for non-numeric id")
	}
	if NextID(7) != "C-0007" {
		t.Errorf("NextID(7) = %q", NextID(7))
	}
}

// =============================================================================
// Runner (tick direto — nenhum daemon é iniciado nos testes)
// =============================================================================

func newTestRunner(t *testing.T, store *Store) *Runner {
	t.Helper()
	return NewRunner(store, zerolog.Nop())
}

func TestRunner_TickExecutesDueJob(t *testing.T) {
	store := newTestStore(t)
	runner := newTestRunner(t, store)

	// NextRun no passado → vence no primeiro tick.
	id, err := store.Add(Job{
		Name:     "due",
		Command:  "true",
		Schedule: mustSchedule(t, "every 30m"),
		Enabled:  true,
		NextRun:  time.Now().Add(-time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}

	runner.tick()

	job, _ := store.Get(id)
	if job.Runs != 1 {
		t.Errorf("Runs = %d, want 1 (job vencido deve executar)", job.Runs)
	}
	if job.LastRun.IsZero() {
		t.Error("LastRun should be recorded")
	}
	if !job.NextRun.After(time.Now()) {
		t.Errorf("NextRun should be recomputed into the future, got %v", job.NextRun)
	}
}

func TestRunner_TickSkipsNonDueAndDisabledJobs(t *testing.T) {
	store := newTestStore(t)
	runner := newTestRunner(t, store)

	futureID, _ := store.Add(Job{
		Name: "future", Command: "true", Schedule: mustSchedule(t, "every 30m"),
		Enabled: true, NextRun: time.Now().Add(time.Hour),
	})
	disabledID, _ := store.Add(Job{
		Name: "disabled", Command: "true", Schedule: mustSchedule(t, "every 30m"),
		Enabled: false, NextRun: time.Now().Add(-time.Minute),
	})

	runner.tick()

	for _, id := range []string{futureID, disabledID} {
		job, _ := store.Get(id)
		if job.Runs != 0 {
			t.Errorf("job %s should not run: Runs = %d", id, job.Runs)
		}
	}
}

func TestRunner_TickErrorJobDoesNotKillRunner(t *testing.T) {
	store := newTestStore(t)
	runner := newTestRunner(t, store)

	failID, _ := store.Add(Job{
		Name: "fail", Command: "false", Schedule: mustSchedule(t, "every 30m"),
		Enabled: true, NextRun: time.Now().Add(-time.Minute),
	})
	okID, _ := store.Add(Job{
		Name: "ok", Command: "true", Schedule: mustSchedule(t, "every 30m"),
		Enabled: true, NextRun: time.Now().Add(-time.Minute),
	})

	runner.tick()

	failJob, _ := store.Get(failID)
	if failJob.Runs != 1 {
		t.Errorf("failing job should still be recorded: Runs = %d", failJob.Runs)
	}
	okJob, _ := store.Get(okID)
	if okJob.Runs != 1 {
		t.Errorf("job after the failing one must still run: Runs = %d", okJob.Runs)
	}
}

func TestRunner_TickZeroNextRunSchedulesWithoutRunning(t *testing.T) {
	store := newTestStore(t)
	runner := newTestRunner(t, store)

	// Add calcula NextRun automaticamente; zeramos para simular job antigo.
	id, _ := store.Add(Job{
		Name: "zero", Command: "true", Schedule: mustSchedule(t, "every 30m"), Enabled: true,
	})
	if err := store.UpdateNextRun(id, time.Time{}); err != nil {
		t.Fatal(err)
	}

	runner.tick()

	job, _ := store.Get(id)
	if job.Runs != 0 {
		t.Errorf("zero-NextRun job should not execute on first tick: Runs = %d", job.Runs)
	}
	if job.NextRun.IsZero() {
		t.Error("zero-NextRun job should get a NextRun after tick")
	}
}

// ── Human / DBPath / Count ───────────────────────────────────────────────

func TestSchedule_Human(t *testing.T) {
	tests := []struct {
		name string
		sch  Schedule
		want string
	}{
		{"interval with raw", Schedule{Kind: KindInterval, Raw: "every 30m", Interval: 30 * time.Minute}, "every 30m"},
		{"interval no raw", Schedule{Kind: KindInterval, Interval: 2 * time.Hour}, "every 2h0m0s"},
		{"daily no raw", Schedule{Kind: KindDaily, Hour: 9, Minute: 0}, "daily at 09:00"},
		{"cron no raw", Schedule{Kind: KindCron, Cron: "0 9 * * *"}, "0 9 * * *"},
		{"unknown kind", Schedule{Kind: "banana"}, "banana"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sch.Human()
			if got != tt.want {
				t.Errorf("Human() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStore_DBPath(t *testing.T) {
	store := newTestStore(t)
	p := store.DBPath()
	if p == "" {
		t.Error("DBPath should not be empty")
	}
}

func TestStore_Count(t *testing.T) {
	store := newTestStore(t)
	n, _ := store.Count()
	if n != 0 {
		t.Errorf("Count = %d, want 0", n)
	}
	store.Add(Job{Name: "a", Command: "true", Schedule: mustSchedule(t, "every 30m"), Enabled: true})
	store.Add(Job{Name: "b", Command: "true", Schedule: mustSchedule(t, "daily at 9am"), Enabled: true})
	n, _ = store.Count()
	if n != 2 {
		t.Errorf("Count = %d, want 2", n)
	}
}

// ── RunCommand ───────────────────────────────────────────────────────────

func TestRunCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("RunCommand executa via sh -c (SafeShellExec) e os comandos 'true'/'false' são shell builtins POSIX — não disponíveis no Windows nativo")
	}
	ctx := context.Background()
	_, err := RunCommand(ctx, "true", 5*time.Second)
	if err != nil {
		t.Errorf("RunCommand(true): %v", err)
	}
	_, err = RunCommand(ctx, "false", 5*time.Second)
	if err == nil {
		t.Error("RunCommand(false) should error")
	}
}

func TestRunCommand_Invalid(t *testing.T) {
	ctx := context.Background()
	_, err := RunCommand(ctx, "nonexistent-command-xyz-12345", 1*time.Second)
	if err == nil {
		t.Error("expected error for invalid command")
	}
}

// ── Additional edge cases ───────────────────────────────────────────────

func TestParseCron_Never(t *testing.T) {
	// Test the cron that never matches is properly rejected
	sch, err := ParseNL("0 0 31 2 *")
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	if _, err := sch.NextAfter(base); err == nil {
		t.Error("expected error for cron that never matches")
	}
}

func TestStore_SetEnabled_InvalidID(t *testing.T) {
	store := newTestStore(t)
	if err := store.SetEnabled("banana", false); err == nil {
		t.Error("expected error for invalid ID")
	}
}

func TestStore_UpdateNextRun_InvalidID(t *testing.T) {
	store := newTestStore(t)
	if err := store.UpdateNextRun("C-9999", time.Now()); err == nil {
		t.Error("expected error for non-existent job")
	}
}

func TestStore_RecordRun_InvalidID(t *testing.T) {
	store := newTestStore(t)
	now := time.Now()
	if err := store.RecordRun("C-9999", now, now.Add(time.Hour), 1); err == nil {
		t.Error("expected error for non-existent job")
	}
}
