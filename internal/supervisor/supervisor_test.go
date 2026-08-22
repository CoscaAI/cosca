package supervisor

import (
	"testing"
	"time"
)

// ── DefaultConfig ────────────────────────────────────────────────────────

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if cfg.HeartbeatInterval != 5*time.Second {
		t.Errorf("HeartbeatInterval = %v, want 5s", cfg.HeartbeatInterval)
	}
	if cfg.HeartbeatTimeout != 10*time.Second {
		t.Errorf("HeartbeatTimeout = %v", cfg.HeartbeatTimeout)
	}
	if cfg.UnresponsiveTimeout != 20*time.Second {
		t.Errorf("UnresponsiveTimeout = %v", cfg.UnresponsiveTimeout)
	}
	if cfg.DeadTimeout != 60*time.Second {
		t.Errorf("DeadTimeout = %v", cfg.DeadTimeout)
	}
	if cfg.DependencyTimeout != 30*time.Second {
		t.Errorf("DependencyTimeout = %v", cfg.DependencyTimeout)
	}
	if cfg.MaxCheckpoints != 10 {
		t.Errorf("MaxCheckpoints = %d", cfg.MaxCheckpoints)
	}
	if !cfg.AutoCorrect {
		t.Error("AutoCorrect should default to true")
	}
}

// ── EscalationLevel.String ───────────────────────────────────────────────

func TestEscalationLevel_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		level EscalationLevel
		want  string
	}{
		{LevelAutoCorrect, "AUTO-CORRECT"},
		{LevelRestart, "RESTART"},
		{LevelRollback, "ROLLBACK"},
		{LevelConflict, "CONFLICT"},
		{LevelEmergency, "EMERGENCY"},
		{EscalationLevel(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		got := tt.level.String()
		if got != tt.want {
			t.Errorf("EscalationLevel(%d).String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}

// ── HeartbeatTracker ─────────────────────────────────────────────────────

func TestHeartbeatTracker_RegisterAndStatus(t *testing.T) {
	tracker := NewHeartbeatTracker(DefaultConfig())
	ks := tracker.Register(KernelBackend)
	if ks.KernelID != KernelBackend {
		t.Errorf("KernelID = %q", ks.KernelID)
	}
	if ks.Health != HealthHealthy {
		t.Errorf("initial Health = %q, want HEALTHY", ks.Health)
	}

	// Get status.
	status, err := tracker.Status(KernelBackend)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.KernelID != KernelBackend {
		t.Errorf("Status.KernelID = %q", status.KernelID)
	}
}

func TestHeartbeatTracker_NotFound(t *testing.T) {
	tracker := NewHeartbeatTracker(DefaultConfig())
	_, err := tracker.Status("nonexistent")
	if err == nil {
		t.Error("expected error for unregistered kernel")
	}
}

func TestHeartbeatTracker_ReceiveHeartbeat(t *testing.T) {
	tracker := NewHeartbeatTracker(DefaultConfig())

	// First heartbeat from an unregistered kernel auto-registers.
	hb := Heartbeat{
		KernelID:   KernelBackend,
		TaskID:     "T-0001",
		Status:     "running",
		Progress:   50,
		Errors:     0,
		MemoryMB:   45,
		Goroutines: 12,
		Timestamp:  time.Now(),
	}

	health, changed := tracker.ReceiveHeartbeat(hb)
	if health != HealthHealthy {
		t.Errorf("health = %q, want HEALTHY", health)
	}
	if !changed {
		t.Error("first heartbeat from new kernel should trigger change")
	}

	// Second heartbeat with same healthy status → no change.
	hb2 := Heartbeat{
		KernelID:  KernelBackend,
		Status:    "running",
		Timestamp: time.Now(),
	}
	health2, changed2 := tracker.ReceiveHeartbeat(hb2)
	if health2 != HealthHealthy {
		t.Errorf("health = %q", health2)
	}
	if changed2 {
		t.Error("same healthy status should not trigger change")
	}
}

func TestHeartbeatTracker_DegradedOnErrors(t *testing.T) {
	tracker := NewHeartbeatTracker(DefaultConfig())

	hb := Heartbeat{
		KernelID:  KernelFrontend,
		Status:    "running",
		Errors:    3,
		Timestamp: time.Now(),
	}
	health, changed := tracker.ReceiveHeartbeat(hb)
	if health != HealthDegraded {
		t.Errorf("health with errors = %q, want DEGRADED", health)
	}
	if !changed {
		t.Error("degraded state should trigger change")
	}
}

func TestHeartbeatTracker_DegradedOnHighMemory(t *testing.T) {
	tracker := NewHeartbeatTracker(DefaultConfig())
	tracker.Register(KernelBackend)

	hb := Heartbeat{
		KernelID:  KernelBackend,
		Status:    "running",
		MemoryMB:  500,
		Timestamp: time.Now(),
	}
	health, _ := tracker.ReceiveHeartbeat(hb)
	if health != HealthDegraded {
		t.Errorf("health with high memory = %q, want DEGRADED", health)
	}
}

func TestHeartbeatTracker_DegradedOnHighGoroutines(t *testing.T) {
	tracker := NewHeartbeatTracker(DefaultConfig())
	tracker.Register(KernelBackend)

	hb := Heartbeat{
		KernelID:   KernelBackend,
		Status:     "running",
		Goroutines: 300,
		Timestamp:  time.Now(),
	}
	health, _ := tracker.ReceiveHeartbeat(hb)
	if health != HealthDegraded {
		t.Errorf("health with high goroutines = %q, want DEGRADED", health)
	}
}

func TestHeartbeatTracker_CheckTimeouts(t *testing.T) {
	cfg := DefaultConfig()
	cfg.HeartbeatTimeout = 50 * time.Millisecond
	cfg.UnresponsiveTimeout = 100 * time.Millisecond
	cfg.DeadTimeout = 200 * time.Millisecond
	tracker := NewHeartbeatTracker(cfg)
	tracker.Register(KernelBackend)

	// Send a heartbeat in the past.
	hb := Heartbeat{
		KernelID:  KernelBackend,
		Status:    "running",
		Timestamp: time.Now().Add(-150 * time.Millisecond),
	}
	tracker.ReceiveHeartbeat(hb)

	timeoutIDs := tracker.CheckTimeouts(time.Now())
	if len(timeoutIDs) != 1 {
		t.Errorf("expected 1 timeout, got %v", timeoutIDs)
	}
	if timeoutIDs[0] != KernelBackend {
		t.Errorf("timeout kernel = %q", timeoutIDs[0])
	}

	status, _ := tracker.Status(KernelBackend)
	if status.Health != HealthUnresponsive {
		t.Errorf("health after timeout = %q, want UNRESPONSIVE", status.Health)
	}
}

func TestHeartbeatTracker_HealthChangeCallback(t *testing.T) {
	tracker := NewHeartbeatTracker(DefaultConfig())
	tracker.Register(KernelBackend)

	var called bool
	var oldHealth, newHealth Health
	tracker.SetOnHealthChange(func(id KernelID, old, new Health) {
		called = true
		oldHealth = old
		newHealth = new
	})

	hb := Heartbeat{
		KernelID:  KernelBackend,
		Status:    "running",
		Errors:    2,
		Timestamp: time.Now(),
	}
	tracker.ReceiveHeartbeat(hb)

	if !called {
		t.Error("health change callback not called")
	}
	if oldHealth != HealthHealthy || newHealth != HealthDegraded {
		t.Errorf("callback: old=%q new=%q", oldHealth, newHealth)
	}
}

func TestHeartbeatTracker_ActiveKernels(t *testing.T) {
	tracker := NewHeartbeatTracker(DefaultConfig())
	if tracker.ActiveKernels() != 0 {
		t.Errorf("ActiveKernels = %d, want 0", tracker.ActiveKernels())
	}
	tracker.Register(KernelBackend)
	tracker.Register(KernelFrontend)
	if tracker.ActiveKernels() != 2 {
		t.Errorf("ActiveKernels = %d, want 2", tracker.ActiveKernels())
	}
}

func TestHeartbeatTracker_Reset(t *testing.T) {
	tracker := NewHeartbeatTracker(DefaultConfig())
	tracker.Register(KernelBackend)

	// Degrade it.
	hb := Heartbeat{
		KernelID:  KernelBackend,
		Status:    "running",
		Errors:    5,
		Timestamp: time.Now(),
	}
	tracker.ReceiveHeartbeat(hb)
	status, _ := tracker.Status(KernelBackend)
	if status.Health != HealthDegraded {
		t.Fatal("should be degraded")
	}

	// Reset.
	tracker.Reset(KernelBackend)
	status, _ = tracker.Status(KernelBackend)
	if status.Health != HealthHealthy {
		t.Errorf("after reset: health = %q, want HEALTHY", status.Health)
	}
	if status.MissedBeats != 0 {
		t.Errorf("after reset: MissedBeats = %d", status.MissedBeats)
	}
}

// ── InterruptManager ─────────────────────────────────────────────────────

func TestInterruptManager_Checkpoint(t *testing.T) {
	cfg := DefaultConfig()
	im := NewInterruptManager(cfg)

	ckpt := im.Checkpoint(KernelBackend, "T-0042", []string{"handler.go", "handler_test.go"}, 12, 10)
	if ckpt.KernelID != KernelBackend {
		t.Errorf("KernelID = %q", ckpt.KernelID)
	}
	if ckpt.TaskID != "T-0042" {
		t.Errorf("TaskID = %q", ckpt.TaskID)
	}
	if len(ckpt.FilesModified) != 2 {
		t.Errorf("FilesModified = %d", len(ckpt.FilesModified))
	}

	// Retrieve latest.
	latest, err := im.LatestCheckpoint(KernelBackend)
	if err != nil {
		t.Fatalf("LatestCheckpoint: %v", err)
	}
	if latest.ID != ckpt.ID {
		t.Errorf("latest ID = %q, want %q", latest.ID, ckpt.ID)
	}
}

func TestInterruptManager_LatestCheckpoint_NotFound(t *testing.T) {
	im := NewInterruptManager(DefaultConfig())
	_, err := im.LatestCheckpoint("nonexistent")
	if err == nil {
		t.Error("expected error for kernel with no checkpoints")
	}
}

func TestInterruptManager_AllCheckpoints(t *testing.T) {
	im := NewInterruptManager(DefaultConfig())
	im.Checkpoint(KernelBackend, "T-0001", nil, 0, 0)
	im.Checkpoint(KernelBackend, "T-0002", nil, 0, 0)
	all := im.AllCheckpoints(KernelBackend)
	if len(all) != 2 {
		t.Errorf("AllCheckpoints = %d, want 2", len(all))
	}
}

func TestInterruptManager_BuildRequests(t *testing.T) {
	im := NewInterruptManager(DefaultConfig())

	stop := im.BuildStopRequest(KernelBackend, "T-0001", "test failure", true)
	if stop.Action != ActionStop {
		t.Errorf("Action = %q", stop.Action)
	}
	if !stop.SaveState {
		t.Error("SaveState should be true")
	}

	resume := im.BuildResumeRequest(KernelBackend, "T-0001", &Adjustment{Description: "fix test"})
	if resume.Action != ActionResume {
		t.Errorf("Action = %q", resume.Action)
	}
	if resume.Adjustment == nil || resume.Adjustment.Description != "fix test" {
		t.Error("adjustment not set correctly")
	}

	kill := im.BuildKillRequest(KernelBackend, "dead")
	if kill.Action != ActionKill {
		t.Errorf("Action = %q", kill.Action)
	}

	ping := im.BuildPingRequest()
	if ping.Action != ActionPing {
		t.Errorf("Action = %q", ping.Action)
	}
}

func TestInterruptManager_IsRecoverable(t *testing.T) {
	im := NewInterruptManager(DefaultConfig())
	if !im.IsRecoverable(InterruptResponse{Status: "STOPPED", CheckpointID: "ckpt-1"}) {
		t.Error("stopped with checkpoint should be recoverable")
	}
	if !im.IsRecoverable(InterruptResponse{TestsRun: 10, TestsPassed: 8, CheckpointID: "ckpt-2"}) {
		t.Error("test failure with checkpoint should be recoverable")
	}
	if im.IsRecoverable(InterruptResponse{Status: "ERROR"}) {
		t.Error("error without checkpoint should not be recoverable")
	}
}

func TestInterruptManager_NeedsRollback(t *testing.T) {
	im := NewInterruptManager(DefaultConfig())
	if !im.NeedsRollback(InterruptResponse{TestsRun: 10, TestsPassed: 2, FilesModified: []string{"a.go"}}) {
		t.Error("catastrophic test failure should need rollback")
	}
	if im.NeedsRollback(InterruptResponse{TestsRun: 10, TestsPassed: 9, FilesModified: []string{"a.go"}}) {
		t.Error("minor test failure should not need rollback")
	}
}

// ── Diagnose ─────────────────────────────────────────────────────────────

func TestDiagnose_NilResponse(t *testing.T) {
	status := &KernelStatus{KernelID: KernelBackend, LastHeartbeat: time.Now()}
	result := Diagnose(status, nil)
	if result.Level != LevelRestart {
		t.Errorf("level = %s, want RESTART", result.Level)
	}
	if !result.IsFatal {
		t.Error("nil response should be fatal")
	}
}

func TestDiagnose_SyntaxError(t *testing.T) {
	status := &KernelStatus{KernelID: KernelBackend}
	resp := &InterruptResponse{Error: "syntax error: unexpected } at line 42"}
	result := Diagnose(status, resp)
	if result.Level != LevelAutoCorrect {
		t.Errorf("level = %s, want AUTO-CORRECT", result.Level)
	}
	if result.Adjustment == nil {
		t.Error("syntax error should have adjustment")
	}
}

func TestDiagnose_Panic(t *testing.T) {
	status := &KernelStatus{KernelID: KernelBackend}
	resp := &InterruptResponse{Error: "panic: runtime error: invalid memory address"}
	result := Diagnose(status, resp)
	if result.Level != LevelRollback {
		t.Errorf("level = %s, want ROLLBACK", result.Level)
	}
	if !result.IsFatal {
		t.Error("panic should be fatal")
	}
}

func TestDiagnose_TestFailureMinor(t *testing.T) {
	status := &KernelStatus{KernelID: KernelBackend}
	resp := &InterruptResponse{TestsRun: 10, TestsPassed: 8}
	result := Diagnose(status, resp)
	if result.Level != LevelAutoCorrect {
		t.Errorf("level = %s, want AUTO-CORRECT for minor failures", result.Level)
	}
}

func TestDiagnose_TestFailureCatastrophic(t *testing.T) {
	status := &KernelStatus{KernelID: KernelBackend}
	resp := &InterruptResponse{TestsRun: 10, TestsPassed: 2}
	result := Diagnose(status, resp)
	if result.Level != LevelRollback {
		t.Errorf("level = %s, want ROLLBACK for catastrophic failure", result.Level)
	}
}

func TestDiagnose_StoppedCleanly(t *testing.T) {
	status := &KernelStatus{KernelID: KernelBackend}
	resp := &InterruptResponse{Status: "STOPPED", CheckpointID: "ckpt-1"}
	result := Diagnose(status, resp)
	if result.Level != LevelAutoCorrect {
		t.Errorf("level = %s, want AUTO-CORRECT for clean stop", result.Level)
	}
}

func TestDiagnose_UnknownError(t *testing.T) {
	status := &KernelStatus{KernelID: KernelBackend}
	resp := &InterruptResponse{Error: "something weird happened"}
	result := Diagnose(status, resp)
	if result.Level != LevelConflict {
		t.Errorf("level = %s, want CONFLICT for unknown error", result.Level)
	}
}

// ── Full coverage gap tests ─────────────────────────────────────────────

func TestHeartbeatTracker_DeadTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.HeartbeatTimeout = 10 * time.Millisecond
	cfg.UnresponsiveTimeout = 20 * time.Millisecond
	cfg.DeadTimeout = 30 * time.Millisecond
	tracker := NewHeartbeatTracker(cfg)
	tracker.Register(KernelBackend)

	hb := Heartbeat{
		KernelID:  KernelBackend,
		Status:    "running",
		Timestamp: time.Now().Add(-50 * time.Millisecond),
	}
	tracker.ReceiveHeartbeat(hb)

	timeoutIDs := tracker.CheckTimeouts(time.Now())
	if len(timeoutIDs) != 1 {
		t.Fatalf("expected 1 dead kernel, got %v", timeoutIDs)
	}
	status, _ := tracker.Status(KernelBackend)
	if status.Health != HealthDead {
		t.Errorf("health = %q, want DEAD", status.Health)
	}
}

func TestHeartbeatTracker_DeadCallback(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DeadTimeout = 1 * time.Millisecond
	tracker := NewHeartbeatTracker(cfg)
	tracker.Register(KernelBackend)

	var deadCalled bool
	tracker.SetOnDead(func(id KernelID) {
		deadCalled = true
	})

	hb := Heartbeat{
		KernelID:  KernelBackend,
		Status:    "running",
		Timestamp: time.Now().Add(-10 * time.Millisecond),
	}
	tracker.ReceiveHeartbeat(hb)

	tracker.CheckTimeouts(time.Now())
	if !deadCalled {
		t.Error("onDead callback should have been called")
	}
}

func TestHeartbeatTracker_StoppedSkipped(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DeadTimeout = 1 * time.Millisecond
	tracker := NewHeartbeatTracker(cfg)

	// Manually set a kernel to STOPPED — don't send heartbeat.
	ks := tracker.Register(KernelBackend)
	ks.Health = HealthStopped
	ks.LastHeartbeat = time.Now().Add(-10 * time.Millisecond)

	timeoutIDs := tracker.CheckTimeouts(time.Now())
	for _, id := range timeoutIDs {
		if id == KernelBackend {
			t.Error("stopped kernel should not appear in timeouts")
		}
	}
}

func TestSupervisor_HandleTimeout(t *testing.T) {
	cfg := DefaultConfig()
	s := New(cfg)
	defer s.Stop()
	s.RegisterKernel(KernelBackend)

	// Set kernel directly to DEAD to trigger the DEAD path in handleTimeout.
	ks := s.tracker.Register(KernelBackend)
	ks.Health = HealthDead

	s.handleTimeout(KernelBackend)

	// Verify escalation was created.
	recent := s.escalate.Recent(5)
	found := false
	for _, e := range recent {
		if e.Level == LevelEmergency && e.KernelID == KernelBackend {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected LevelEmergency escalation for dead kernel")
	}
}

func TestSupervisor_HandleTimeoutUnresponsive(t *testing.T) {
	cfg := DefaultConfig()
	s := New(cfg)
	defer s.Stop()

	// Set kernel directly to UNRESPONSIVE to trigger that path.
	ks := s.tracker.Register(KernelBackend)
	ks.Health = HealthUnresponsive

	s.handleTimeout(KernelBackend)

	// Should have escalated as LevelRestart.
	recent := s.escalate.Recent(5)
	found := false
	for _, e := range recent {
		if e.Level == LevelRestart && e.KernelID == KernelBackend {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected LevelRestart escalation for unresponsive")
	}
}

func TestSupervisor_DiagnoseAndDecideEscalate(t *testing.T) {
	s := New(DefaultConfig())
	defer s.Stop()
	s.RegisterKernel(KernelBackend)

	// Panic error → LevelRollback → must escalate.
	resp := InterruptResponse{
		Status: "ERROR",
		Error:  "panic: runtime error",
	}
	result := s.DiagnoseAndDecide(KernelBackend, resp)
	if result.Level != LevelRollback {
		t.Errorf("level = %s, want ROLLBACK", result.Level)
	}

	pending := s.escalate.PendingApproval()
	if len(pending) == 0 {
		t.Error("expected escalation for LevelRollback")
	}
}

func TestMessageForDon_Default(t *testing.T) {
	e := Escalation{
		Level:    LevelAutoCorrect,
		KernelID: KernelBackend,
		Summary:  "update",
	}
	msg := MessageForDon(e)
	if msg == "" {
		t.Error("default message should not be empty")
	}
}

func TestSupervisor_LoopWithTimeout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.HeartbeatInterval = 10 * time.Millisecond
	cfg.UnresponsiveTimeout = 15 * time.Millisecond
	cfg.DeadTimeout = 20 * time.Millisecond
	s := New(cfg)

	// Register and send old heartbeat.
	s.RegisterKernel(KernelBackend)
	hb := Heartbeat{
		KernelID:  KernelBackend,
		Status:    "running",
		Timestamp: time.Now().Add(-50 * time.Millisecond),
	}
	s.ReceiveHeartbeat(hb)

	s.Start()
	time.Sleep(30 * time.Millisecond)
	s.Stop()

	// The loop should have detected the timeout.
	ks, _ := s.tracker.Status(KernelBackend)
	if ks.Health == HealthHealthy {
		t.Error("loop should have detected timeout")
	}
}

// ── Diagnose type error ─────────────────────────────────────────────────

func TestDiagnose_TypeError(t *testing.T) {
	status := &KernelStatus{KernelID: KernelBackend}
	resp := &InterruptResponse{Error: "type mismatch: int vs string in assignment"}
	result := Diagnose(status, resp)
	if result.Level != LevelRollback {
		t.Errorf("type error level = %s, want ROLLBACK", result.Level)
	}
}

func TestDiagnose_NilPointer(t *testing.T) {
	status := &KernelStatus{KernelID: KernelBackend}
	resp := &InterruptResponse{Error: "runtime error: index out of range [5]"}
	result := Diagnose(status, resp)
	if result.Level != LevelRollback {
		t.Errorf("index out of range level = %s, want ROLLBACK", result.Level)
	}
	if !result.IsFatal {
		t.Error("index out of range should be fatal")
	}
}

func TestDiagnose_StoppedWithoutCheckpoint(t *testing.T) {
	status := &KernelStatus{KernelID: KernelBackend}
	resp := &InterruptResponse{Status: "STOPPED"}
	result := Diagnose(status, resp)
	// Should fall through to unknown state.
	if result.Problem == "" {
		t.Error("should have a problem diagnosis")
	}
}

func TestDiagnose_ModerateTestFailure(t *testing.T) {
	status := &KernelStatus{KernelID: KernelBackend}
	resp := &InterruptResponse{TestsRun: 10, TestsPassed: 6}
	result := Diagnose(status, resp)
	if result.Level != LevelRollback {
		t.Errorf("30-50%% failure level = %s, want ROLLBACK", result.Level)
	}
}

// ── Escalation duplicate message ────────────────────────────────────────

func TestEscalationManager_MultipleSame(t *testing.T) {
	em := NewEscalationManager()
	em.Escalate(KernelBackend, "T-0001", "a", "d", LevelAutoCorrect)
	em.Escalate(KernelBackend, "T-0001", "a", "d", LevelAutoCorrect)
	em.Escalate(KernelBackend, "T-0001", "a", "d", LevelAutoCorrect)

	recent := em.Recent(10)
	if len(recent) != 3 {
		t.Errorf("expected 3 escalations, got %d", len(recent))
	}
}

func TestEscalationManager_RecentLimit(t *testing.T) {
	em := NewEscalationManager()
	for i := 0; i < 5; i++ {
		em.Escalate(KernelBackend, "T-0001", "a", "d", LevelAutoCorrect)
	}
	recent := em.Recent(2)
	if len(recent) != 2 {
		t.Errorf("Recent(2) = %d, want 2", len(recent))
	}
}

// ── InterruptManager empty checkpoints ──────────────────────────────────

func TestInterruptManager_AllCheckpoints_Empty(t *testing.T) {
	im := NewInterruptManager(DefaultConfig())
	all := im.AllCheckpoints("nonexistent")
	if len(all) != 0 {
		t.Errorf("expected 0 checkpoints, got %d", len(all))
	}
}

// ── DiagnoseShouldAutoFix boundary ─────────────────────────────────────

func TestShouldAutoFix_FalseWithoutAdjustment(t *testing.T) {
	d := DiagnoseResult{Level: LevelAutoCorrect, Adjustment: nil}
	if d.ShouldAutoFix() {
		t.Error("should not auto-fix without adjustment")
	}
}

// ── Close remaining coverage gaps ───────────────────────────────────────

func TestHeartbeatTracker_ActiveKernelsWithDead(t *testing.T) {
	tracker := NewHeartbeatTracker(DefaultConfig())
	tracker.Register(KernelBackend)
	ks := tracker.Register(KernelFrontend)
	ks.Health = HealthDead

	if tracker.ActiveKernels() != 1 {
		t.Errorf("ActiveKernels = %d, want 1 (dead not counted)", tracker.ActiveKernels())
	}
}

func TestHeartbeatTracker_CheckTimeoutsSlow(t *testing.T) {
	cfg := DefaultConfig()
	cfg.HeartbeatTimeout = 10 * time.Millisecond
	cfg.UnresponsiveTimeout = 100 * time.Millisecond
	cfg.DeadTimeout = 200 * time.Millisecond
	tracker := NewHeartbeatTracker(cfg)
	tracker.Register(KernelBackend)

	hb := Heartbeat{
		KernelID:  KernelBackend,
		Timestamp: time.Now().Add(-30 * time.Millisecond),
	}
	tracker.ReceiveHeartbeat(hb)

	ids := tracker.CheckTimeouts(time.Now())
	// Should be SLOW (not unresponsive yet), so no IDs returned.
	if len(ids) != 0 {
		t.Errorf("slow kernel should not be in timeout list, got %v", ids)
	}
	status, _ := tracker.Status(KernelBackend)
	if status.Health != HealthSlow {
		t.Errorf("slow kernel health = %q, want SLOW", status.Health)
	}
}

func TestSupervisor_HandleTimeoutSlow(t *testing.T) {
	cfg := DefaultConfig()
	s := New(cfg)
	defer s.Stop()

	ks := s.tracker.Register(KernelBackend)
	ks.Health = HealthSlow

	s.handleTimeout(KernelBackend)
	// Slow just logs — no escalation.
	recent := s.escalate.Recent(5)
	if len(recent) != 0 {
		t.Errorf("slow should not escalate, got %d", len(recent))
	}
}

func TestDiagnose_DefaultUnknownState(t *testing.T) {
	status := &KernelStatus{KernelID: KernelBackend}
	// Response with no error, not stopped, not failed tests.
	resp := &InterruptResponse{Status: "RUNNING"}
	result := Diagnose(status, resp)
	if result.Level != LevelRestart {
		t.Errorf("default unknown state level = %s, want RESTART", result.Level)
	}
}

func TestEscalate_LevelRestartDecision(t *testing.T) {
	em := NewEscalationManager()
	e := em.Escalate(KernelBackend, "T-0001", "precisa reiniciar", "detail", LevelRestart)
	if e.NeedsDon {
		t.Error("LevelRestart should not need Don")
	}
	if e.Decision == "" {
		t.Error("LevelRestart should have a decision")
	}
}

func TestEscalate_Pruning(t *testing.T) {
	em := NewEscalationManager()
	// Add 150 escalations to trigger pruning (keeps last 100).
	for i := 0; i < 150; i++ {
		em.Escalate(KernelBackend, "T-0001", "test", "d", LevelAutoCorrect)
	}
	recent := em.Recent(200)
	if len(recent) > 100 {
		t.Errorf("pruned length = %d, want <= 100", len(recent))
	}
}

func TestHeartbeatTracker_CheckTimeoutsSlowIncrementsMissed(t *testing.T) {
	cfg := DefaultConfig()
	cfg.HeartbeatTimeout = 5 * time.Millisecond
	cfg.UnresponsiveTimeout = 100 * time.Millisecond
	cfg.DeadTimeout = 200 * time.Millisecond
	tracker := NewHeartbeatTracker(cfg)
	ks := tracker.Register(KernelBackend)
	ks.LastHeartbeat = time.Now().Add(-20 * time.Millisecond)

	tracker.CheckTimeouts(time.Now())
	status, _ := tracker.Status(KernelBackend)
	if status.MissedBeats == 0 {
		t.Error("MissedBeats should be incremented for slow kernel")
	}
}

// ── EscalationManager ────────────────────────────────────────────────────

func TestEscalationManager_Escalate(t *testing.T) {
	em := NewEscalationManager()
	e := em.Escalate(KernelBackend, "T-0001", "teste falhou", "detalhes do erro", LevelRollback)
	if !e.NeedsDon {
		t.Error("LevelRollback should need Don")
	}
	if e.Level != LevelRollback {
		t.Errorf("Level = %s", e.Level)
	}
}

func TestEscalationManager_AutoCorrectNoDon(t *testing.T) {
	em := NewEscalationManager()
	e := em.Escalate(KernelBackend, "T-0001", "erro simples", "detail", LevelAutoCorrect)
	if e.NeedsDon {
		t.Error("LevelAutoCorrect should NOT need Don")
	}
}

func TestEscalationManager_Recent(t *testing.T) {
	em := NewEscalationManager()
	em.Escalate(KernelBackend, "T-0001", "a", "d", LevelAutoCorrect)
	em.Escalate(KernelFrontend, "T-0002", "b", "d", LevelRestart)

	recent := em.Recent(5)
	if len(recent) != 2 {
		t.Errorf("Recent = %d, want 2", len(recent))
	}
	// Newest first.
	if recent[0].KernelID != KernelFrontend {
		t.Errorf("most recent kernel = %q, want %q", recent[0].KernelID, KernelFrontend)
	}
}

func TestEscalationManager_PendingApproval(t *testing.T) {
	em := NewEscalationManager()
	em.Escalate(KernelBackend, "T-0001", "minor", "d", LevelAutoCorrect)
	em.Escalate(KernelBackend, "T-0002", "critical", "d", LevelRollback)
	em.Escalate(KernelFrontend, "T-0003", "emergency", "d", LevelEmergency)

	pending := em.PendingApproval()
	if len(pending) != 2 {
		t.Errorf("PendingApproval = %d, want 2", len(pending))
	}
}

func TestEscalationManager_CountByLevel(t *testing.T) {
	em := NewEscalationManager()
	em.Escalate(KernelBackend, "T-0001", "a", "d", LevelAutoCorrect)
	em.Escalate(KernelBackend, "T-0002", "b", "d", LevelAutoCorrect)
	em.Escalate(KernelFrontend, "T-0003", "c", "d", LevelRestart)

	counts := em.CountByLevel()
	if counts[LevelAutoCorrect] != 2 {
		t.Errorf("AutoCorrect count = %d, want 2", counts[LevelAutoCorrect])
	}
	if counts[LevelRestart] != 1 {
		t.Errorf("Restart count = %d, want 1", counts[LevelRestart])
	}
}

func TestMessageForDon(t *testing.T) {
	tests := []struct {
		level   EscalationLevel
		contain string
	}{
		{LevelRollback, "rollback"},
		{LevelConflict, "conflito"},
		{LevelEmergency, "EMERGÊNCIA"},
	}
	for _, tt := range tests {
		e := Escalation{
			Level:    tt.level,
			KernelID: KernelBackend,
			TaskID:   "T-0001",
			Summary:  "problema",
			Detail:   "detalhe",
		}
		msg := MessageForDon(e)
		if msg == "" {
			t.Errorf("MessageForDon empty for level %s", tt.level)
		}
	}
}

// ── Supervisor ───────────────────────────────────────────────────────────

func TestSupervisor_New(t *testing.T) {
	s := New(DefaultConfig())
	if s == nil {
		t.Fatal("New returned nil")
	}
	defer s.Stop()
}

func TestSupervisor_RegisterAndHeartbeat(t *testing.T) {
	s := New(DefaultConfig())
	defer s.Stop()

	s.RegisterKernel(KernelBackend)
	s.RegisterKernel(KernelFrontend)

	hb := Heartbeat{
		KernelID:  KernelBackend,
		Status:    "running",
		Timestamp: time.Now(),
	}
	health, _ := s.ReceiveHeartbeat(hb)
	if health != HealthHealthy {
		t.Errorf("health = %q", health)
	}
}

func TestSupervisor_StartStop(t *testing.T) {
	s := New(DefaultConfig())
	s.Start()
	// Let it run for a tick.
	time.Sleep(20 * time.Millisecond)
	s.Stop()
}

func TestSupervisor_InterruptAndResume(t *testing.T) {
	s := New(DefaultConfig())
	defer s.Stop()
	s.RegisterKernel(KernelBackend)

	req := s.InterruptKernel(KernelBackend, "T-0001", "testing interrupt",
		[]string{"handler.go"}, 5, 5)
	if req.Action != ActionStop {
		t.Errorf("Action = %q, want STOP", req.Action)
	}

	resume := s.ResumeKernel(KernelBackend, "T-0001",
		&Adjustment{Description: "fix applied"})
	if resume.Action != ActionResume {
		t.Errorf("Action = %q, want RESUME", resume.Action)
	}
}

func TestSupervisor_DiagnoseAndDecide(t *testing.T) {
	s := New(DefaultConfig())
	defer s.Stop()
	s.RegisterKernel(KernelBackend)

	resp := InterruptResponse{
		Status:       "STOPPED",
		CheckpointID: "ckpt-1",
		TestsRun:     10,
		TestsPassed:  9,
	}
	result := s.DiagnoseAndDecide(KernelBackend, resp)
	if result.Level != LevelAutoCorrect {
		t.Errorf("level = %s, want AUTO-CORRECT", result.Level)
	}
}

func TestSupervisor_DependencyRequest(t *testing.T) {
	s := New(DefaultConfig())
	defer s.Stop()

	req := DependencyRequest{
		ID:         "DEP-001",
		FromKernel: KernelFrontend,
		ToKernel:   KernelBackend,
		Request:    "contract",
		Endpoint:   "/v1/users",
		Timeout:    30 * time.Second,
	}
	s.RequestDependency(req)

	pending := s.PendingDependencies()
	if len(pending) != 1 {
		t.Errorf("PendingDependencies = %d, want 1", len(pending))
	}

	s.ResolveDependency("DEP-001", DependencyResponse{
		Status: "ready",
	})

	pending = s.PendingDependencies()
	if len(pending) != 0 {
		t.Errorf("after resolve: PendingDependencies = %d, want 0", len(pending))
	}
}

func TestSupervisor_Status(t *testing.T) {
	s := New(DefaultConfig())
	defer s.Stop()
	s.RegisterKernel(KernelBackend)

	status := s.Status()
	if status.ActiveKernels != 1 {
		t.Errorf("ActiveKernels = %d, want 1", status.ActiveKernels)
	}
	if status.PendingEscalations != 0 {
		t.Errorf("PendingEscalations = %d, want 0", status.PendingEscalations)
	}
	if status.Kernels == nil {
		t.Error("Kernels map should not be nil")
	}
}

func TestSupervisor_AutoDegradeOnErrors(t *testing.T) {
	s := New(DefaultConfig())
	defer s.Stop()
	s.RegisterKernel(KernelBackend)

	hb := Heartbeat{
		KernelID:  KernelBackend,
		Status:    "running",
		Errors:    5,
		Timestamp: time.Now(),
	}
	health, changed := s.ReceiveHeartbeat(hb)
	if !changed {
		t.Error("expected health change")
	}
	if health != HealthDegraded {
		t.Errorf("health = %q, want DEGRADED", health)
	}
}

// ── InterruptManager_CheckpointPruning ────────────────────────────────────

func TestInterruptManager_CheckpointPruning(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxCheckpoints = 2
	im := NewInterruptManager(cfg)

	im.Checkpoint(KernelBackend, "T-0001", nil, 0, 0)
	im.Checkpoint(KernelBackend, "T-0002", nil, 0, 0)
	im.Checkpoint(KernelBackend, "T-0003", nil, 0, 0) // should prune T-0001

	all := im.AllCheckpoints(KernelBackend)
	if len(all) != 2 {
		t.Errorf("after pruning: got %d, want 2", len(all))
	}
	// Oldest should be gone, newest two should remain.
	if all[0].TaskID != "T-0002" || all[1].TaskID != "T-0003" {
		t.Errorf("checkpoints = %v", all)
	}
}

// ── AllStatuses ──────────────────────────────────────────────────────────

func TestHeartbeatTracker_AllStatuses(t *testing.T) {
	tracker := NewHeartbeatTracker(DefaultConfig())
	tracker.Register(KernelBackend)
	tracker.Register(KernelFrontend)

	all := tracker.AllStatuses()
	if len(all) != 2 {
		t.Errorf("AllStatuses = %d, want 2", len(all))
	}
}

func TestHeartbeatTracker_IdleHealthy(t *testing.T) {
	tracker := NewHeartbeatTracker(DefaultConfig())
	tracker.Register(KernelBackend)

	hb := Heartbeat{
		KernelID:  KernelBackend,
		Status:    "idle",
		Timestamp: time.Now(),
	}
	health, _ := tracker.ReceiveHeartbeat(hb)
	if health != HealthHealthy {
		t.Errorf("idle kernel health = %q, want HEALTHY", health)
	}
}

// ── Diagnose dependency errors ───────────────────────────────────────────

func TestDiagnose_DependencyError(t *testing.T) {
	status := &KernelStatus{KernelID: KernelBackend}
	resp := &InterruptResponse{Error: "missing go.sum entry for module"}
	result := Diagnose(status, resp)
	if result.Level != LevelAutoCorrect {
		t.Errorf("level = %s, want AUTO-CORRECT for dependency error", result.Level)
	}
	if result.Adjustment == nil {
		t.Fatal("expected adjustment for dependency error")
	}
}
