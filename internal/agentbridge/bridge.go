package agentbridge

import (
	"sync"
	"time"
)

// BridgeState is the operational state of the bridge process.
type BridgeState string

const (
	// BridgeOffline: no bridge process is running.
	BridgeOffline BridgeState = "offline"
	// BridgeStarting: the bridge is booting (markBridgeStarting).
	BridgeStarting BridgeState = "starting"
	// BridgeOnline: the bridge is listening and ready.
	BridgeOnline BridgeState = "online"
	// BridgeStopping: the bridge is being torn down.
	BridgeStopping BridgeState = "stopping"
)

// ResumeStrategy is how a session is re-attached after a suspend.
// attach → the bridge is still alive, reconnect to the same coords.
// rerun  → the bridge died, spawn a new one and replay the event log.
type ResumeStrategy string

const (
	// ResumeAttach: reconnect to the still-living bridge.
	ResumeAttach ResumeStrategy = "attach"
	// ResumeRerun: spawn a new bridge and replay from disk.
	ResumeRerun ResumeStrategy = "rerun"
	// ResumeReplay: replay buffered events with seq > lastSeenEventId.
	ResumeReplay ResumeStrategy = "replay"
)

// BridgeMonitor tracks the bridge process serving agent sessions.
type BridgeMonitor struct {
	mu             sync.RWMutex
	state          BridgeState
	pid            int
	wsPort         int
	channelToken   string // randomBytes(32); masked in JSON output
	sandbox        string
	startedAt      time.Time
	lastSeenEvent  int64
	resumeStrategy ResumeStrategy
	eventLogPath   string
}

// NewBridgeMonitor creates an offline monitor.
func NewBridgeMonitor() *BridgeMonitor {
	return &BridgeMonitor{state: BridgeOffline, resumeStrategy: ResumeAttach}
}

// MarkStarting transitions the bridge to starting.
func (bm *BridgeMonitor) MarkStarting() {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.state = BridgeStarting
	bm.startedAt = time.Now().UTC()
}

// MarkOnline marks the bridge as ready with its connection coordinates.
// The token is stored but always masked in JSON output.
func (bm *BridgeMonitor) MarkOnline(pid, port int, token, sandbox, eventLogPath string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.state = BridgeOnline
	bm.pid = pid
	bm.wsPort = port
	bm.channelToken = token
	bm.sandbox = sandbox
	bm.eventLogPath = eventLogPath
}

// MarkOffline transitions the bridge to offline.
func (bm *BridgeMonitor) MarkOffline() {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.state = BridgeOffline
}

// MarkStopping transitions the bridge to stopping.
func (bm *BridgeMonitor) MarkStopping() {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.state = BridgeStopping
}

// SetLastSeenEvent records the latest replayed event id.
func (bm *BridgeMonitor) SetLastSeenEvent(seq int64) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if seq > bm.lastSeenEvent {
		bm.lastSeenEvent = seq
	}
}

// SetResumeStrategy records which strategy a resume used.
func (bm *BridgeMonitor) SetResumeStrategy(s ResumeStrategy) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.resumeStrategy = s
}

// State returns a snapshot of the bridge monitor.
func (bm *BridgeMonitor) State() BridgeSnapshot {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	return BridgeSnapshot{
		State:          bm.state,
		PID:            bm.pid,
		WSPort:         bm.wsPort,
		ChannelToken:   maskToken(bm.channelToken),
		Sandbox:        bm.sandbox,
		StartedAt:      bm.startedAt,
		LastSeenEvent:  bm.lastSeenEvent,
		ResumeStrategy: string(bm.resumeStrategy),
		EventLogPath:   bm.eventLogPath,
	}
}

// BridgeSnapshot is the JSON-safe view of the bridge monitor.
type BridgeSnapshot struct {
	State          BridgeState `json:"state"`
	PID            int         `json:"pid"`
	WSPort         int         `json:"ws_port"`
	ChannelToken   string      `json:"channel_token"` // always masked
	Sandbox        string      `json:"sandbox"`
	StartedAt      time.Time   `json:"started_at"`
	LastSeenEvent  int64       `json:"last_seen_event"`
	ResumeStrategy string      `json:"resume_strategy"`
	EventLogPath   string      `json:"event_log_path"`
}

// maskToken returns a masked representation of a sensitive token:
// first 4 chars visible, rest replaced — never the full value.
func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return "••••"
	}
	return token[:4] + "••••••••••••••••"
}
