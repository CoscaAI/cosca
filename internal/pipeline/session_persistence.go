package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type SessionStore struct {
	dir string
}

type SessionRecord struct {
	ID           string        `json:"id"`
	ProjectPath  string        `json:"project_path"`
	Goal         string        `json:"goal"`
	LastTask     string        `json:"last_task"`
	Progress     float64       `json:"progress"`
	State        string        `json:"state"`
	StartedAt    time.Time     `json:"started_at"`
	LastActiveAt time.Time     `json:"last_active_at"`
	Blockers     []string      `json:"blockers"`
	SnapshotPath string        `json:"snapshot_path"`
	Plan         *PlanSnapshot `json:"plan,omitempty"`
	Agent        string        `json:"agent,omitempty"`
	Model        string        `json:"model,omitempty"`
}

type PlanSnapshot struct {
	ID               string            `json:"id"`
	Intent           string            `json:"intent"`
	TasksTotal       int               `json:"tasks_total"`
	TasksCompleted   int               `json:"tasks_completed"`
	TasksFailed      int               `json:"tasks_failed"`
	EstimatedMinutes int               `json:"estimated_minutes"`
	TaskStatuses     map[string]string `json:"task_statuses"`
}

func NewSessionStore(dir string) (*SessionStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("session store: create directory: %w", err)
	}
	return &SessionStore{dir: dir}, nil
}

func (s *SessionStore) Save(record *SessionRecord) error {
	record.LastActiveAt = time.Now().UTC()

	path := filepath.Join(s.dir, record.ID+".session.json")
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("session store: marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("session store: write: %w", err)
	}
	return nil
}

func (s *SessionStore) Load(id string) (*SessionRecord, error) {
	path := filepath.Join(s.dir, id+".session.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("session store: read: %w", err)
	}
	var record SessionRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("session store: unmarshal: %w", err)
	}
	return &record, nil
}

func (s *SessionStore) LoadLatest(projectPath string) (*SessionRecord, error) {
	records, err := s.List(projectPath)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("no sessions found for project: %s", projectPath)
	}
	return records[0], nil
}

func (s *SessionStore) List(projectPath string) ([]*SessionRecord, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("session store: read dir: %w", err)
	}

	var records []*SessionRecord
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(s.dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var record SessionRecord
		if err := json.Unmarshal(data, &record); err != nil {
			continue
		}
		if projectPath == "" || record.ProjectPath == projectPath {
			records = append(records, &record)
		}
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].LastActiveAt.After(records[j].LastActiveAt)
	})

	return records, nil
}

func (s *SessionStore) Delete(id string) error {
	path := filepath.Join(s.dir, id+".session.json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("session store: delete: %w", err)
	}
	return nil
}

func RecordFromTerminalContext(tc *TerminalContext, goal, lastTask string) *SessionRecord {
	completed, total := 0, 0
	if tc.SessionContext != nil && tc.SessionContext.Plan != nil {
		completed, total = tc.SessionContext.Plan.Progress()
	}

	progress := float64(0)
	if total > 0 {
		progress = float64(completed) / float64(total)
	}

	record := &SessionRecord{
		ID:           tc.SessionID,
		ProjectPath:  tc.ProjectPath,
		Goal:         goal,
		LastTask:     lastTask,
		Progress:     progress,
		State:        tc.SessionState,
		StartedAt:    tc.SessionContext.Metrics.StartedAt,
		LastActiveAt: time.Now().UTC(),
		Agent:        tc.ActiveAgent,
		Model:        tc.ActiveModel,
	}

	if tc.SessionContext != nil && tc.SessionContext.Plan != nil {
		plan := tc.SessionContext.Plan
		taskStatuses := make(map[string]string)
		for _, t := range plan.Tasks {
			taskStatuses[t.ID] = string(t.Status)
		}

		record.Plan = &PlanSnapshot{
			ID:               plan.ID,
			Intent:           plan.Intent,
			TasksTotal:       total,
			TasksCompleted:   completed,
			TasksFailed:      0,
			EstimatedMinutes: plan.EstimatedMinutes,
			TaskStatuses:     taskStatuses,
		}
	}

	return record
}

func RestoreTerminalContext(tc *TerminalContext, record *SessionRecord) {
	tc.LastSessionID = record.ID
	tc.ProjectPath = record.ProjectPath
	tc.SessionState = record.State
	tc.ActiveAgent = record.Agent
	tc.ActiveModel = record.Model

	if record.Plan != nil {
		plan := &Plan{
			ID:               record.Plan.ID,
			Intent:           record.Plan.Intent,
			EstimatedMinutes: record.Plan.EstimatedMinutes,
			CreatedAt:        record.StartedAt,
		}

		for id, status := range record.Plan.TaskStatuses {
			task := &TaskNode{
				ID:     id,
				Status: TaskStatus(status),
			}
			plan.Tasks = append(plan.Tasks, task)
		}

		tc.SessionContext = NewSessionContext(plan)
	}
}
