package cli

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/circadian"
)

// NewCircadianCommand creates the `cosca circadian` command tree: it exposes
// the Circadian Engine — the Operational Rest Cycle (ORC) state machine — and
// its scheduler via CLI.
func NewCircadianCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "circadian",
		Short: "Operate the Circadian Engine (Operational Rest Cycle)",
		Long: `Inspect and operate the Circadian Engine — the Cosca Operational Rest Cycle (ORC).

The ORC is a maintenance optimization model: a quiet system progressively
winds down expensive operations until it opens a deep maintenance window (the
ORC window) where only unattended housekeeping runs. When the Don interacts
again, the engine wakes and returns to full availability. Nomenclature: the
ORC is a rest cycle — never "the AI sleeps".

Subcommands:
  status    Show engine state, idle duration and the last ORC run
  sleep     Enter the ORC window and run the rest cycle synchronously
  wake      Run the wake ritual (memory, knowledge, constitution, clock)
  watch     Run the ORC scheduler in the foreground until Ctrl+C`,
	}

	cmd.AddCommand(
		NewCircadianStatusCommand(),
		NewCircadianSleepCommand(),
		NewCircadianWakeCommand(),
		NewCircadianWatchCommand(),
	)
	return cmd
}

// CircadianStatus is the JSON payload of `cosca circadian status`.
type CircadianStatus struct {
	State       string          `json:"state"`
	Proposed    string          `json:"proposed_state"`
	Idle        string          `json:"idle"`
	IdleSeconds float64         `json:"idle_seconds"`
	ShouldRest  bool            `json:"should_rest"`
	CoscaDir    string          `json:"cosca_dir"`
	LastORC     *LastORCSummary `json:"last_orc,omitempty"`
}

// LastORCSummary describes the most recent rest cycle report found on disk.
type LastORCSummary struct {
	ReportPath string    `json:"report_path"`
	ModifiedAt time.Time `json:"modified_at"`
	Age        string    `json:"age"`
}

// NewCircadianStatusCommand creates `cosca circadian status`.
func NewCircadianStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Circadian Engine state, idle duration and last ORC summary",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			e := circadian.New()
			coscaDir := resolveCoscaDir()
			idle := e.IdleDuration()

			status := CircadianStatus{
				State:       string(e.State()),
				Proposed:    string(e.Evaluate()),
				Idle:        idle.Round(time.Second).String(),
				IdleSeconds: idle.Seconds(),
				ShouldRest:  e.ShouldRest(),
				CoscaDir:    coscaDir,
				LastORC:     latestORCSummary(coscaDir),
			}

			if useJSON {
				return printJSON(cmd, status)
			}

			formatter.Header("Circadian Engine — Status")
			formatter.KeyValue("State", status.State)
			formatter.KeyValue("Proposed", fmt.Sprintf("%s (idle %s)", status.Proposed, status.Idle))
			formatter.KeyValue("Idle", status.Idle)
			formatter.KeyValue("ShouldRest", fmt.Sprintf("%v", status.ShouldRest))
			formatter.KeyValue("Cosca Dir", status.CoscaDir)
			if status.LastORC != nil {
				formatter.KeyValue("Last ORC", fmt.Sprintf("%s (%s ago)",
					filepath.Base(status.LastORC.ReportPath), status.LastORC.Age))
			} else {
				formatter.KeyValue("Last ORC", "none — no rest cycle report found")
			}
			return nil
		},
	}
}

// NewCircadianSleepCommand creates `cosca circadian sleep`.
func NewCircadianSleepCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "sleep",
		Short: "Enter the ORC window and run the rest cycle synchronously",
		Long: `Manually enter the ORC window and run the Operational Rest Cycle (ORC)
maintenance pipeline synchronously, printing the cycle report.

The engine is walked through the transition matrix (awake -> idle -> resting
-> sleeping) so the manual sleep is always a valid transition.`,
		Example: `  cosca circadian sleep          # enter the ORC window and run the cycle
  cosca circadian sleep --json   # machine-readable cycle report`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			e := circadian.New()
			coscaDir := resolveCoscaDir()

			// Walk the descent so the transition matrix is respected.
			for _, next := range []circadian.CircadianState{
				circadian.StateIdle, circadian.StateResting, circadian.StateSleeping,
			} {
				if e.State() == next {
					continue
				}
				if err := e.TransitionTo(next, "manual"); err != nil {
					break
				}
			}
			if e.State() != circadian.StateSleeping {
				return fmt.Errorf("cannot enter the ORC window from state %q", e.State())
			}

			result, err := circadian.RunORC(cmd.Context(), coscaDir)
			if err != nil {
				return err
			}
			if result == nil {
				return fmt.Errorf("ORC returned a nil result")
			}

			if useJSON {
				return printJSON(cmd, result)
			}

			formatter.Header("Operational Rest Cycle — Completed")
			formatter.KeyValue("Started", result.StartedAt.UTC().Format(time.RFC3339))
			formatter.KeyValue("Ended", result.EndedAt.UTC().Format(time.RFC3339))
			formatter.KeyValue("Duration", result.Duration.Round(time.Millisecond).String())
			formatter.KeyValue("Items Processed", fmt.Sprint(result.ItemsProcessed))
			formatter.KeyValue("Duplicates Removed", fmt.Sprint(result.DuplicatesRemoved))
			formatter.KeyValue("Deprecated", fmt.Sprint(result.DeprecatedCount))
			if result.ReportPath != "" {
				formatter.KeyValue("Report", result.ReportPath)
			}

			rows := make([][]string, 0, len(result.Steps))
			for _, s := range result.Steps {
				rows = append(rows, []string{s.Name, s.Status, fmt.Sprintf("%dms", s.DurationMs), s.Detail})
			}
			formatter.Header("Steps")
			formatter.Table([]string{"Step", "Status", "Duration", "Detail"}, rows)

			if len(result.Errors) > 0 {
				formatter.Header("Errors")
				for _, errMsg := range result.Errors {
					formatter.Bullet(errMsg)
				}
			}
			return nil
		},
	}
}

// NewCircadianWakeCommand creates `cosca circadian wake`.
func NewCircadianWakeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "wake",
		Short: "Run the wake ritual and return the engine to the awake state",
		Long: `Run the six-step wake ritual: load kernel memory, validate the knowledge
base, read the constitution, sync the clock, render the summary and accept
commands again (transitioning the engine back to awake).`,
		Example: `  cosca circadian wake          # run the wake ritual
  cosca circadian wake --json   # machine-readable ritual result`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			e := circadian.New()
			coscaDir := resolveCoscaDir()

			result, err := e.Wake(cmd.Context(), coscaDir)
			if err != nil {
				return err
			}
			if result == nil {
				return fmt.Errorf("wake ritual returned a nil result")
			}

			if useJSON {
				return printJSON(cmd, result)
			}

			formatter.Header("Wake Ritual — Summary")
			formatter.KeyValue("Summary", result.Summary)
			formatter.KeyValue("At", result.At.UTC().Format(time.RFC3339))
			formatter.KeyValue("Knowledge Valid", fmt.Sprintf("%v", result.KnowledgeValid))
			formatter.KeyValue("Constitution", fmt.Sprintf("%v", result.ConstitutionLoaded))

			rows := make([][]string, 0, len(result.Steps))
			for _, s := range result.Steps {
				rows = append(rows, []string{s.Name, s.Status, s.Detail})
			}
			formatter.Header("Steps")
			formatter.Table([]string{"Step", "Status", "Detail"}, rows)
			return nil
		},
	}
}

// NewCircadianWatchCommand creates `cosca circadian watch`.
func NewCircadianWatchCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "watch",
		Short: "Run the ORC scheduler in the foreground until interrupted",
		Long: `Start the Circadian Engine scheduler: every 30 seconds the engine is
evaluated against the idle clock, the state machine descends through the ORC
states, and the rest cycle runs exactly once per ORC window. Blocks until
Ctrl+C.`,
		Example: `  cosca circadian watch    # run the scheduler in the foreground`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)

			e := circadian.New()
			coscaDir := resolveCoscaDir()
			sched := circadian.NewScheduler(e, coscaDir)

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			sched.Start(ctx)
			defer sched.Stop()

			formatter.Println("ORC scheduler running (interval 30s) — press Ctrl+C to stop")
			formatter.KeyValue("State", string(e.State()))
			formatter.KeyValue("Cosca Dir", coscaDir)

			<-ctx.Done()
			formatter.Println("scheduler stopped")
			return nil
		},
	}
}

// resolveCoscaDir returns the .cosca directory of the current working
// directory. The directory may not exist yet; callers must degrade
// gracefully (RunORC and Wake both handle a missing directory).
func resolveCoscaDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return filepath.Join(".", ".cosca")
	}
	return filepath.Join(dir, ".cosca")
}

// latestORCSummary finds the most recently written rest cycle report in
// <coscaDir>/memory/audit, if any.
func latestORCSummary(coscaDir string) *LastORCSummary {
	matches, err := filepath.Glob(filepath.Join(coscaDir, "memory", "audit", "rest-cycle-*.md"))
	if err != nil || len(matches) == 0 {
		return nil
	}
	sort.Slice(matches, func(i, j int) bool {
		mi, erri := os.Stat(matches[i])
		mj, errj := os.Stat(matches[j])
		if erri != nil || errj != nil {
			return matches[i] < matches[j]
		}
		return mi.ModTime().After(mj.ModTime())
	})
	info, err := os.Stat(matches[0])
	if err != nil {
		return nil
	}
	return &LastORCSummary{
		ReportPath: matches[0],
		ModifiedAt: info.ModTime(),
		Age:        time.Since(info.ModTime()).Round(time.Second).String(),
	}
}
