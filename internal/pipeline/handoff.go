package pipeline

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var evidenceIDRe = regexp.MustCompile(`^EV-\d{8}-[0-9A-Fa-f]{8}$`)

var handoffIDRe = regexp.MustCompile(`^HO-\d{8}-[0-9A-Fa-f]{8}$`)

// NewHandoffID generates a new handoff artifact ID: HO-YYYYMMDD-XXXXXXXX.
func NewHandoffID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		n := uint32(time.Now().UnixNano() & 0xFFFFFFFF)
		b[0] = byte(n >> 24)
		b[1] = byte(n >> 16)
		b[2] = byte(n >> 8)
		b[3] = byte(n)
	}
	return fmt.Sprintf("HO-%s-%02X%02X%02X%02X",
		time.Now().UTC().Format("20060102"), b[0], b[1], b[2], b[3])
}

// HandoffArtifact is the structured record passed between agents.
type HandoffArtifact struct {
	ID            string       `json:"id"`
	FromAgent     string       `json:"from_agent"`
	ToAgent       string       `json:"to_agent"`
	Objective     string       `json:"objective"`
	Changes       []FileChange `json:"changes,omitempty"`
	Decisions     []Decision   `json:"decisions,omitempty"`
	Evidence      []Evidence   `json:"evidence,omitempty"`
	Files         []string     `json:"files,omitempty"`
	Tests         []string     `json:"tests,omitempty"`
	Errors        []string     `json:"errors,omitempty"`
	Unresolved    []string     `json:"unresolved,omitempty"`
	KnowledgeUsed []string     `json:"knowledge_used,omitempty"`
	CreatedAt     time.Time    `json:"created_at"`
}

// FileChange describes a file modification made by an agent.
type FileChange struct {
	Path         string `json:"path"`
	Action       string `json:"action"` // created, modified, deleted
	LinesAdded   int    `json:"lines_added"`
	LinesRemoved int    `json:"lines_removed"`
	Reason       string `json:"reason"`
}

// Decision records a choice made by an agent and the reasoning behind it.
type Decision struct {
	Question     string   `json:"question"`
	Choice       string   `json:"choice"`
	Reasoning    string   `json:"reasoning"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

// NewEvidenceID generates a new evidence artifact ID: EV-YYYYMMDD-XXXXXXXX.
func NewEvidenceID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		n := uint32(time.Now().UnixNano() & 0xFFFFFFFF)
		b[0] = byte(n >> 24)
		b[1] = byte(n >> 16)
		b[2] = byte(n >> 8)
		b[3] = byte(n)
	}
	return fmt.Sprintf("EV-%s-%02X%02X%02X%02X",
		time.Now().UTC().Format("20060102"), b[0], b[1], b[2], b[3])
}

// Evidence captures supporting data for a decision.
type Evidence struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`     // test_result, build_output, lint_report, security_scan, health_check, file_diff, migration_log
	Status   string `json:"status"`   // "pass", "fail", "warning"
	Summary  string `json:"summary"`  // human-readable description
	Detail   string `json:"detail"`   // detailed output (truncated if too long)
	Source   string `json:"source"`   // which tool/agent produced this
	Artifact string `json:"artifact"` // path to stored artifact file
	Hash     string `json:"hash"`     // SHA256 of the artifact for integrity verification
}

// VerifyHash reads the artifact file, computes SHA256, and compares with Hash.
func (e *Evidence) VerifyHash() (bool, error) {
	if e.Artifact == "" {
		return false, fmt.Errorf("evidence %q has no artifact path", e.ID)
	}
	data, err := os.ReadFile(e.Artifact)
	if err != nil {
		return false, fmt.Errorf("read artifact %q: %w", e.Artifact, err)
	}
	h := sha256.Sum256(data)
	got := hex.EncodeToString(h[:])
	return got == e.Hash, nil
}

// FormatEvidenceReport formats evidence as a CLI-friendly tree.
func FormatEvidenceReport(evidence []Evidence, taskStatus string, nextAction string) string {
	var b strings.Builder
	b.WriteString("Evidence Report\n")
	b.WriteString("===============\n")
	if taskStatus != "" {
		b.WriteString(fmt.Sprintf("Task Status:  %s\n", taskStatus))
	}
	if nextAction != "" {
		b.WriteString(fmt.Sprintf("Next Action:  %s\n", nextAction))
	}
	if len(evidence) == 0 {
		b.WriteString("\n(no evidence recorded)\n")
		return b.String()
	}
	b.WriteString(fmt.Sprintf("\nEvidence (%d):\n", len(evidence)))
	for _, ev := range evidence {
		icon := "?"
		switch ev.Status {
		case "pass":
			icon = "[PASS]"
		case "fail":
			icon = "[FAIL]"
		case "warning":
			icon = "[WARN]"
		}
		b.WriteString(fmt.Sprintf("  %s %s: %s", icon, ev.Kind, ev.Summary))
		if ev.Source != "" {
			b.WriteString(fmt.Sprintf("  (source: %s)", ev.Source))
		}
		b.WriteString("\n")
		if ev.Detail != "" {
			detail := ev.Detail
			if len(detail) > 120 {
				detail = detail[:120] + "..."
			}
			b.WriteString(fmt.Sprintf("    %s\n", detail))
		}
		if ev.Artifact != "" {
			b.WriteString(fmt.Sprintf("    artifact: %s", ev.Artifact))
			if ev.Hash != "" {
				b.WriteString(fmt.Sprintf("  hash: %s", ev.Hash))
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

// HandoffStore persists artifacts for cross-agent access.
type HandoffStore struct {
	dir string // .cosca/handoffs/
	mu  sync.RWMutex
}

// NewHandoffStore creates a HandoffStore rooted at the given directory.
// The directory is created if it does not exist.
func NewHandoffStore(dir string) (*HandoffStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create handoff directory: %w", err)
	}
	return &HandoffStore{dir: dir}, nil
}

// Save persists a HandoffArtifact as a JSON file.
// The filename is derived from the artifact ID: HO-YYYYMMDD-XXXXXXXX.json.
func (s *HandoffStore) Save(artifact *HandoffArtifact) error {
	if artifact.ID == "" {
		artifact.ID = NewHandoffID()
	}
	if !handoffIDRe.MatchString(artifact.ID) {
		return fmt.Errorf("handoff: invalid artifact ID %q (expected HO-YYYYMMDD-XXXXXXXX)", artifact.ID)
	}
	if artifact.CreatedAt.IsZero() {
		artifact.CreatedAt = time.Now().UTC()
	}

	path := filepath.Join(s.dir, artifact.ID+".json")

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal handoff artifact: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write handoff artifact: %w", err)
	}

	return nil
}

// Load reads a HandoffArtifact from its JSON file by ID.
func (s *HandoffStore) Load(id string) (*HandoffArtifact, error) {
	path := filepath.Join(s.dir, id+".json")

	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read handoff artifact %q: %w", id, err)
	}

	var artifact HandoffArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return nil, fmt.Errorf("unmarshal handoff artifact %q: %w", id, err)
	}

	return &artifact, nil
}

// List returns the IDs of all stored handoff artifacts sorted by name.
func (s *HandoffStore) List() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read handoff directory: %w", err)
	}

	var ids []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) == ".json" {
			ids = append(ids, name[:len(name)-5]) // strip .json
		}
	}

	return ids, nil
}

// Dir returns the store directory path.
func (s *HandoffStore) Dir() string {
	return s.dir
}
