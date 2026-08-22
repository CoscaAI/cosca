package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type EvidenceStore struct {
	dir string // .cosca/evidence/
	mu  sync.RWMutex
}

func NewEvidenceStore(dir string) (*EvidenceStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create evidence directory: %w", err)
	}
	return &EvidenceStore{dir: dir}, nil
}

func (s *EvidenceStore) Store(e *Evidence) error {
	if e.ID == "" {
		e.ID = NewEvidenceID()
	}
	if !evidenceIDRe.MatchString(e.ID) {
		return fmt.Errorf("evidence: invalid ID %q (expected EV-YYYYMMDD-XXXXXXXX)", e.ID)
	}

	if e.Artifact != "" && e.Hash == "" {
		data, err := os.ReadFile(e.Artifact)
		if err != nil {
			return fmt.Errorf("evidence: read artifact %q: %w", e.Artifact, err)
		}
		h := sha256.Sum256(data)
		e.Hash = hex.EncodeToString(h[:])
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, e.ID+".json")
	payload, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return fmt.Errorf("evidence: marshal: %w", err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("evidence: write: %w", err)
	}
	return nil
}

func (s *EvidenceStore) Load(id string) (*Evidence, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("evidence: read %q: %w", id, err)
	}
	var e Evidence
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("evidence: unmarshal %q: %w", id, err)
	}
	return &e, nil
}

func (s *EvidenceStore) List() ([]Evidence, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("evidence: read directory: %w", err)
	}

	var out []Evidence
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, name))
		if err != nil {
			continue
		}
		var e Evidence
		if err := json.Unmarshal(data, &e); err != nil {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}
