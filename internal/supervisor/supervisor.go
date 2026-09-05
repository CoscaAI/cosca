package supervisor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Supervisor is the central control system for all sub-kernels.
// It monitors heartbeats, detects anomalies, interrupts tasks,
// diagnoses problems, adjusts and resumes, and escalates critical
// situations to the Don.
type Supervisor struct {
	tracker   *HeartbeatTracker
	interrupt *InterruptManager
	escalate  *EscalationManager
	cfg       Config
	logger    zerolog.Logger

	// Dependency tracking.
	dependencies map[string]*DependencyRequest // requestID → dep
	depMu        sync.RWMutex

	// Running control.
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a new Supervisor with the given configuration.
func New(cfg Config) *Supervisor {
	if cfg.HeartbeatInterval == 0 {
		cfg = DefaultConfig()
	}
	logger := log.With().Str("component", "supervisor").Logger()

	ctx, cancel := context.WithCancel(context.Background())

	s := &Supervisor{
		tracker:      NewHeartbeatTracker(cfg),
		interrupt:    NewInterruptManager(cfg),
		escalate:     NewEscalationManager(),
		cfg:          cfg,
		logger:       logger,
		dependencies: make(map[string]*DependencyRequest),
		ctx:          ctx,
		cancel:       cancel,
	}

	// Set up health change callback.
	s.tracker.SetOnHealthChange(func(id KernelID, old, new Health) {
		s.logger.Warn().
			Str("kernel", string(id)).
			Str("from", string(old)).
			Str("to", string(new)).
			Msg("kernel health changed")
	})

	// Set up timeout callback.
	s.tracker.SetOnTimeout(func(id KernelID) {
		s.logger.Warn().
			Str("kernel", string(id)).
			Msg("kernel unresponsive — sending ping")
		// Auto-ping slow kernels.
		req := s.interrupt.BuildPingRequest()
		s.logger.Debug().Str("kernel", string(id)).Str("action", string(req.Action)).Msg("auto-ping sent")
	})

	// Set up dead callback.
	s.tracker.SetOnDead(func(id KernelID) {
		s.logger.Error().
			Str("kernel", string(id)).
			Msg("kernel dead — escalating")
		s.escalate.Escalate(id, "", "kernel morto — sem heartbeat por 60s",
			"O kernel parou de responder completamente. Precisa ser reiniciado.",
			LevelEmergency)
	})

	return s
}

// RegisterKernel registers a sub-kernel for monitoring.
func (s *Supervisor) RegisterKernel(id KernelID) {
	s.tracker.Register(id)
	s.logger.Info().Str("kernel", string(id)).Msg("kernel registered")
}

// ReceiveHeartbeat processes a heartbeat from a sub-kernel.
func (s *Supervisor) ReceiveHeartbeat(hb Heartbeat) (Health, bool) {
	return s.tracker.ReceiveHeartbeat(hb)
}

// Start begins the supervisor's monitoring loop.
// Runs in the background until the context is cancelled.
func (s *Supervisor) Start() {
	s.wg.Add(1)
	go s.loop()
	s.logger.Info().
		Dur("interval", s.cfg.HeartbeatInterval).
		Msg("supervisor monitoring loop started")
}

// loop is the main monitoring loop.
func (s *Supervisor) loop() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.cfg.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			timedOut := s.tracker.CheckTimeouts(time.Now())
			for _, id := range timedOut {
				s.handleTimeout(id)
			}

		case <-s.ctx.Done():
			s.logger.Info().Msg("supervisor monitoring loop stopped")
			return
		}
	}
}

// handleTimeout processes a kernel that has timed out.
func (s *Supervisor) handleTimeout(id KernelID) {
	ks, err := s.tracker.Status(id)
	if err != nil {
		return
	}

	status := ks.Health
	s.logger.Warn().
		Str("kernel", string(id)).
		Str("health", string(status)).
		Dur("elapsed", time.Since(ks.LastHeartbeat)).
		Msg("kernel timeout — handling")

	switch status {
	case HealthDead:
		// Kill and escalate.
		req := s.interrupt.BuildKillRequest(id, "heartbeat timeout > 60s")
		s.logger.Error().Str("kernel", string(id)).Str("action", string(req.Action)).Msg("killing dead kernel")
		s.escalate.Escalate(id, ks.CurrentTask,
			"kernel morto — sem heartbeat por mais de 60 segundos",
			"O kernel precisa ser reiniciado manualmente ou pelo Supervisor.",
			LevelEmergency)

	case HealthUnresponsive:
		// Stop and diagnose.
		s.logger.Warn().Str("kernel", string(id)).Msg("stopping unresponsive kernel")
		s.escalate.Escalate(id, ks.CurrentTask,
			"kernel não respondeu — interrompido",
			"O kernel está há mais de 20s sem heartbeat. Interrompido para diagnóstico.",
			LevelRestart)

	case HealthSlow:
		// Just log — auto-ping already sent by callback.
		s.logger.Debug().Str("kernel", string(id)).Msg("kernel slow — monitoring")
	}
}

// Stop gracefully shuts down the supervisor.
func (s *Supervisor) Stop() {
	s.cancel()
	s.wg.Wait()
	s.logger.Info().Msg("supervisor stopped")
}

// InterruptKernel sends a stop request to a kernel and saves a checkpoint.
func (s *Supervisor) InterruptKernel(id KernelID, taskID, reason string, filesModified []string, testsRun, testsPassed int) InterruptRequest {
	// Save checkpoint.
	_ = s.interrupt.Checkpoint(id, taskID, filesModified, testsRun, testsPassed)

	// Build stop request.
	req := s.interrupt.BuildStopRequest(id, taskID, reason, true)

	// Update tracker state.
	s.tracker.Reset(id)

	s.logger.Warn().
		Str("kernel", string(id)).
		Str("task", taskID).
		Str("reason", reason).
		Msg("kernel interrupted")

	return req
}

// ResumeKernel builds a resume request with adjustment.
func (s *Supervisor) ResumeKernel(id KernelID, taskID string, adj *Adjustment) InterruptRequest {
	req := s.interrupt.BuildResumeRequest(id, taskID, adj)
	adjDesc := ""
	if adj != nil {
		adjDesc = adj.Description
	}
	s.logger.Info().
		Str("kernel", string(id)).
		Str("task", taskID).
		Str("adjustment", adjDesc).
		Msg("kernel resumed")
	return req
}

// DiagnoseAndDecide analyzes an interrupt response and decides the next action.
func (s *Supervisor) DiagnoseAndDecide(id KernelID, resp InterruptResponse) DiagnoseResult {
	ks, err := s.tracker.Status(id)
	if err != nil || ks == nil {
		return DiagnoseResult{
			Problem: fmt.Sprintf("kernel %q status unavailable: %v", id, err),
			Level:   LevelConflict,
			IsFatal: true,
		}
	}
	result := Diagnose(ks, &resp)

	s.logger.Info().
		Str("kernel", string(id)).
		Str("problem", result.Problem).
		Str("level", result.Level.String()).
		Msg("diagnosis complete")

	if result.ShouldAutoFix() {
		s.logger.Info().Str("kernel", string(id)).Msg("auto-correcting")
		return result
	}

	// Escalate if needed.
	if result.Level >= LevelRollback {
		s.escalate.Escalate(id, "", result.Problem, result.Cause, result.Level)
	}

	return result
}

// RequestDependency registers a cross-kernel dependency request.
func (s *Supervisor) RequestDependency(req DependencyRequest) {
	s.depMu.Lock()
	defer s.depMu.Unlock()
	req.Status = "pending"
	req.CreatedAt = time.Now()
	s.dependencies[req.ID] = &req
	s.logger.Debug().
		Str("from", string(req.FromKernel)).
		Str("to", string(req.ToKernel)).
		Str("request", req.Request).
		Msg("dependency requested")
}

// ResolveDependency marks a dependency as resolved with a response.
func (s *Supervisor) ResolveDependency(id string, resp DependencyResponse) {
	s.depMu.Lock()
	defer s.depMu.Unlock()
	if dep, ok := s.dependencies[id]; ok {
		dep.Status = "resolved"
		dep.Response = &resp
		s.logger.Debug().
			Str("id", id).
			Str("status", resp.Status).
			Msg("dependency resolved")
	}
}

// PendingDependencies returns all unresolved dependencies.
func (s *Supervisor) PendingDependencies() []DependencyRequest {
	s.depMu.RLock()
	defer s.depMu.RUnlock()
	var pending []DependencyRequest
	for _, dep := range s.dependencies {
		if dep.Status == "pending" {
			pending = append(pending, *dep)
		}
	}
	return pending
}

// Status returns a summary of the supervisor's state.
func (s *Supervisor) Status() SupervisorStatus {
	all := s.tracker.AllStatuses()
	escalations := s.escalate.PendingApproval()

	return SupervisorStatus{
		Kernels:             all,
		ActiveKernels:       s.tracker.ActiveKernels(),
		PendingEscalations:  len(escalations),
		EscalationsByLevel:  s.escalate.CountByLevel(),
		PendingDependencies: len(s.PendingDependencies()),
		Config:              s.cfg,
	}
}

// SupervisorStatus is a snapshot of the supervisor's current state.
type SupervisorStatus struct {
	Kernels             map[KernelID]*KernelStatus `json:"kernels"`
	ActiveKernels       int                        `json:"active_kernels"`
	PendingEscalations  int                        `json:"pending_escalations"`
	EscalationsByLevel  map[EscalationLevel]int    `json:"escalations_by_level"`
	PendingDependencies int                        `json:"pending_dependencies"`
	Config              Config                     `json:"config"`
}
