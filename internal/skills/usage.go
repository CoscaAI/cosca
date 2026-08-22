// Telemetria de uso das skills + curador.
//
// Um sidecar .cosca/skills/.usage.json registra use_count, last_activity_at,
// created_by e o estado (active | stale | archived) de cada skill. O curador
// usa esses dados para ARQUIVAR skills stale — NUNCA deleta nada (princípio
// do curador). Skills embutidas (sem fonte local) são apenas marcadas como
// archived, nunca movidas.
//
// Regra do Don: o curador é EXPLÍCITO — não roda sozinho e nunca apaga.
package skills

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Estados de uma skill na telemetria.
const (
	StateActive   = "active"   // em uso / pronta para uso
	StateStale    = "stale"    // sem uso há N dias (candidata ao curador)
	StateArchived = "archived" // arquivada (movida/marcada pelo curador)
)

// usageFileName é o sidecar de telemetria de uso das skills.
const usageFileName = ".usage.json"

// SkillUsage registra a telemetria de uso de uma skill.
type SkillUsage struct {
	SkillName      string `json:"skill_name"`
	UseCount       int    `json:"use_count"`
	LastActivityAt string `json:"last_activity_at"` // RFC3339 ou vazio (nunca usada)
	CreatedBy      string `json:"created_by,omitempty"`
	State          string `json:"state"` // active | stale | archived
	Notes          string `json:"notes,omitempty"`
}

// UsageStore é o sidecar de telemetria de uso das skills em
// <coscaDir>/skills/.usage.json. O arquivo é gravado de forma atômica
// (tmp + rename) com 0600.
type UsageStore struct {
	mu         sync.Mutex
	coscaDir   string
	skillsDir  string
	archiveDir string
	path       string
}

// NewUsageStore cria o sidecar de telemetria em <coscaDir>/skills/.usage.json,
// criando o diretório pai se necessário.
func NewUsageStore(coscaDir string) (*UsageStore, error) {
	if coscaDir == "" {
		coscaDir = filepath.Join(".", ".cosca")
	}
	skillsDir := filepath.Join(coscaDir, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return nil, fmt.Errorf("usage store: criar %q: %w", skillsDir, err)
	}
	return &UsageStore{
		coscaDir:   coscaDir,
		skillsDir:  skillsDir,
		archiveDir: filepath.Join(skillsDir, ".archive"),
		path:       filepath.Join(skillsDir, usageFileName),
	}, nil
}

// Path retorna o caminho do arquivo .usage.json.
func (s *UsageStore) Path() string { return s.path }

// SkillsDir retorna o diretório de skills locais (.cosca/skills).
func (s *UsageStore) SkillsDir() string { return s.skillsDir }

// ArchiveDir retorna o diretório de arquivamento (.cosca/skills/.archive).
func (s *UsageStore) ArchiveDir() string { return s.archiveDir }

// Get lê o registro de uso de uma skill.
func (s *UsageStore) Get(name string) (*SkillUsage, error) {
	snap, err := s.Snapshot()
	if err != nil {
		return nil, err
	}
	u, ok := snap[name]
	if !ok {
		return nil, fmt.Errorf("usage store: skill %q sem registro de uso", name)
	}
	return &u, nil
}

// Snapshot retorna o mapa atual de uso das skills (vazio se o arquivo ainda
// não existir).
func (s *UsageStore) Snapshot() (map[string]SkillUsage, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]SkillUsage{}, nil
		}
		return nil, fmt.Errorf("usage store: ler %q: %w", s.path, err)
	}
	snap := map[string]SkillUsage{}
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("usage store: parsear %q: %w", s.path, err)
	}
	if snap == nil {
		snap = map[string]SkillUsage{}
	}
	return snap, nil
}

// Save grava o snapshot de forma atômica (tmp + rename) com 0600.
func (s *UsageStore) Save(snapshot map[string]SkillUsage) error {
	if snapshot == nil {
		snapshot = map[string]SkillUsage{}
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("usage store: serializar: %w", err)
	}
	data = append(data, '\n')

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("usage store: escrever tmp %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("usage store: renomear %q → %q: %w", tmp, s.path, err)
	}
	return nil
}

// RecordUse incrementa UseCount, atualiza LastActivityAt para agora e garante
// o estado active. Gravação atômica (tmp + rename).
func (s *UsageStore) RecordUse(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	snap, err := s.Snapshot()
	if err != nil {
		return err
	}
	u, ok := snap[name]
	if !ok {
		u = SkillUsage{SkillName: name}
	}
	u.SkillName = name
	u.UseCount++
	u.LastActivityAt = time.Now().UTC().Format(time.RFC3339)
	u.State = StateActive
	snap[name] = u
	return s.Save(snap)
}

// MarkStale marca a skill como stale (sem mover nada). Útil para marcar
// manualmente antes de o curador arquivar.
func (s *UsageStore) MarkStale(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	snap, err := s.Snapshot()
	if err != nil {
		return err
	}
	u, ok := snap[name]
	if !ok {
		u = SkillUsage{SkillName: name}
	}
	u.SkillName = name
	u.State = StateStale
	snap[name] = u
	return s.Save(snap)
}

// StaleSince devolve as skills active cuja última atividade é mais antiga que
// N dias. Skills nunca usadas (state active sem timestamp) são tratadas como
// stale depois de N dias desde a criação do arquivo de telemetria. Retorna em
// ordem alfabética. Não altera nada.
func (s *UsageStore) StaleSince(days int) ([]SkillUsage, error) {
	return s.staleSinceAt(time.Now(), days)
}

func (s *UsageStore) staleSinceAt(now time.Time, days int) ([]SkillUsage, error) {
	if days < 0 {
		days = 0
	}
	snap, err := s.Snapshot()
	if err != nil {
		return nil, err
	}
	// Referência para skills nunca usadas: a criação do arquivo de telemetria.
	fileAge := now
	if info, err := os.Stat(s.path); err == nil {
		fileAge = info.ModTime()
	}
	threshold := now.AddDate(0, 0, -days)

	var stale []SkillUsage
	for name, u := range snap {
		if u.State != StateActive {
			continue
		}
		if u.LastActivityAt == "" {
			// Nunca usada: stale depois de N dias desde a criação da telemetria.
			if fileAge.AddDate(0, 0, days).Before(now) {
				u.SkillName = name
				stale = append(stale, u)
			}
			continue
		}
		ts, err := time.Parse(time.RFC3339, u.LastActivityAt)
		if err != nil {
			continue
		}
		if ts.Before(threshold) {
			u.SkillName = name
			stale = append(stale, u)
		}
	}
	sort.Slice(stale, func(i, j int) bool { return stale[i].SkillName < stale[j].SkillName })
	return stale, nil
}

// Archive arquiva a skill: se houver fonte local, move o diretório/arquivo
// para .cosca/skills/.archive/<name>/ e marca state=archived. Se a skill for
// embutida (sem fonte local), apenas marca state=archived com a nota
// "embedded — não removível, apenas marcado". NUNCA deleta nada.
func (s *UsageStore) Archive(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	snap, err := s.Snapshot()
	if err != nil {
		return err
	}
	u, ok := snap[name]
	if !ok {
		u = SkillUsage{SkillName: name}
	}
	u.SkillName = name

	src, err := s.findLocalSource(name)
	if err != nil {
		return err
	}
	if src == "" {
		// Sem fonte local → skill embutida (ou override inexistente):
		// apenas marca; nunca move.
		u.State = StateArchived
		u.Notes = "embedded — não removível, apenas marcado"
		snap[name] = u
		return s.Save(snap)
	}

	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("usage store: stat %q: %w", src, err)
	}
	if err := os.MkdirAll(s.archiveDir, 0o700); err != nil {
		return fmt.Errorf("usage store: criar archive %q: %w", s.archiveDir, err)
	}
	dst := filepath.Join(s.archiveDir, name)
	if !info.IsDir() {
		// Fonte é um arquivo .md: movido para .archive/<name>/<name>.md.
		if err := os.MkdirAll(dst, 0o700); err != nil {
			return fmt.Errorf("usage store: criar %q: %w", dst, err)
		}
		dst = filepath.Join(dst, info.Name())
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("usage store: arquivar %s → %s: %w", src, dst, err)
	}

	u.State = StateArchived
	u.Notes = ""
	snap[name] = u
	return s.Save(snap)
}

// Restore move a skill arquivada de volta para .cosca/skills/ e marca
// state=active (archived → active). Skills embutidas (apenas marcadas) são
// reativadas sem movimento de arquivos.
func (s *UsageStore) Restore(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	snap, err := s.Snapshot()
	if err != nil {
		return err
	}
	u, ok := snap[name]
	if !ok {
		return fmt.Errorf("usage store: skill %q sem registro de uso", name)
	}

	archived := filepath.Join(s.archiveDir, name)
	if info, err := os.Stat(archived); err == nil {
		var target string
		if info.IsDir() {
			target = filepath.Join(s.skillsDir, name)
		} else {
			target = filepath.Join(s.skillsDir, info.Name())
		}
		if err := os.Rename(archived, target); err != nil {
			return fmt.Errorf("usage store: restaurar %s: %w", name, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("usage store: stat %q: %w", archived, err)
	}

	u.State = StateActive
	u.Notes = ""
	snap[name] = u
	return s.Save(snap)
}

// findLocalSource procura a fonte local da skill em .cosca/skills (arquivo
// <name>.md ou diretório <name>, inclusive em subdiretórios de categoria).
// Retorna "" quando não há fonte local (skill embutida).
func (s *UsageStore) findLocalSource(name string) (string, error) {
	for _, c := range []string{
		filepath.Join(s.skillsDir, name+".md"),
		filepath.Join(s.skillsDir, name),
	} {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("usage store: stat %q: %w", c, err)
		}
	}

	found := ""
	_ = filepath.Walk(s.skillsDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if path == s.skillsDir {
			return nil
		}
		base := filepath.Base(path)
		if info.IsDir() {
			if base == ".archive" {
				return filepath.SkipDir
			}
			if base == name {
				found = path
				return filepath.SkipAll
			}
			return nil
		}
		if base == name+".md" && path != s.path {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	return found, nil
}
