package circadian

import "time"

// Activity identifies a class of work the engine may be asked to run.
// The ORC gates decide whether an activity is permitted in the current state.
type Activity string

// The activities the ORC gates evaluate.
const (
	// ActivityRunAgent is an agent call (LLM-backed work). In the lower
	// states only "light" agent calls are acceptable; the gate cannot
	// distinguish weight, so callers must keep idle-time agent calls
	// lightweight.
	ActivityRunAgent Activity = "run_agent"

	// ActivityBenchmark is a performance benchmark run.
	ActivityBenchmark Activity = "benchmark"

	// ActivityAudit is a long-running audit (knowledge, security,
	// architecture).
	ActivityAudit Activity = "audit"

	// ActivityMaintenance is a scheduled maintenance task.
	ActivityMaintenance Activity = "maintenance"

	// ActivityORC is the deep, unattended maintenance window of the ORC
	// cycle. It only runs while the engine is in the ORC window.
	ActivityORC Activity = "orc"

	// ActivityHeavyIndex is a full/heavy index rebuild.
	ActivityHeavyIndex Activity = "heavy_index"

	// ActivityLightMaintenance is cheap housekeeping (GC, small cleanups,
	// temp-file sweeps) that is safe even in the deepest ORC states.
	ActivityLightMaintenance Activity = "light_maintenance"
)

// allActivities lists every known activity, used by tests and tooling to walk
// the full gate matrix.
var allActivities = []Activity{
	ActivityRunAgent,
	ActivityBenchmark,
	ActivityAudit,
	ActivityMaintenance,
	ActivityORC,
	ActivityHeavyIndex,
	ActivityLightMaintenance,
}

// activityAllowlist is the gate table. An activity is permitted in a state
// only if it is explicitly listed; anything else is denied (deny by default).
//
//	                run  bench  audit  maint  orc  heavy  light
//	awake            x    x      x      x      x    x      x
//	focused          x    x      x      .      .    x      .
//	idle             x    .      .      x      .    .      .
//	resting          .    .      .      .      .    .      x
//	sleeping         .    .      .      .      x    .      x
//	maintenance      .    .      .      .      x    .      x
var activityAllowlist = map[CircadianState]map[Activity]bool{
	StateAwake: {
		ActivityRunAgent:         true,
		ActivityBenchmark:        true,
		ActivityAudit:            true,
		ActivityMaintenance:      true,
		ActivityORC:              true,
		ActivityHeavyIndex:       true,
		ActivityLightMaintenance: true,
	},
	StateFocused: {
		ActivityRunAgent:   true,
		ActivityBenchmark:  true,
		ActivityAudit:      true,
		ActivityHeavyIndex: true,
	},
	StateIdle: {
		ActivityRunAgent:    true,
		ActivityMaintenance: true,
	},
	StateResting: {
		ActivityLightMaintenance: true,
	},
	StateSleeping: {
		ActivityLightMaintenance: true,
		ActivityORC:              true,
	},
	StateMaintenance: {
		ActivityLightMaintenance: true,
		ActivityORC:              true,
	},
}

// allowed reports whether activity is permitted in state. It is the pure gate
// used by CanRun; callers must hold the engine lock or use CanRun.
func (e *Engine) allowed(activity Activity, state CircadianState) bool {
	return activityAllowlist[state][activity]
}

// ShouldRest reports whether the engine has been idle long enough to warrant
// winding down: the idle duration is beyond the IdleToResting threshold. It is
// a hint for the scheduler to start the awake -> idle -> resting descent.
func (e *Engine) ShouldRest() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return time.Since(e.lastActivity) > e.config.IdleToResting
}
