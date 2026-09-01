package acquisition

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Diretório dos artefatos adquiridos, relativo a .cosca. O corpo fica em
// .cosca/quarantine/artifacts/ — junto da zona de quarentena, mas FORA do
// conhecimento: artefatos externos nunca entram em knowledge/laws/memory.
const (
	// ArtifactsDir é o diretório dos artefatos (relativo a .cosca/).
	ArtifactsDir = "quarantine/artifacts"

	artifactIDPrefix = "A-"
	artifactIDWidth  = 4
)

// ArtifactStore persiste artefatos adquiridos e seus corpos em
// .cosca/quarantine/artifacts/. Cada artefato tem dois arquivos:
//
//	A-XXXX.json — metadados (AcquiredArtifact)
//	A-XXXX      — corpo bruto (nunca em knowledge)
//
// A gravação é atômica (tmp + rename) e o ID é reivindicado com
// O_CREATE|O_EXCL, como na zona de quarentena — chamadas concorrentes nunca
// colidem.
type ArtifactStore struct {
	dir string // <dataDir>/quarantine/artifacts
}

// NewArtifactStore cria a store de artefatos a partir do DATA DIR (a raiz
// .cosca do projeto — <projeto>/.cosca ou o dir global do cosca). O diretório
// dos artefatos fica em <dataDir>/quarantine/artifacts. A operação é
// preguiçosa: o diretório é criado na primeira escrita (0700).
func NewArtifactStore(dataDir string) *ArtifactStore {
	return &ArtifactStore{
		dir: filepath.Join(dataDir, filepath.FromSlash(ArtifactsDir)),
	}
}

// Path retorna o diretório dos artefatos (.cosca/quarantine/artifacts).
func (s *ArtifactStore) Path() string {
	return s.dir
}

// Add atribui um ID A-XXXX auto-incremental ao artefato, grava os metadados
// (A-XXXX.json) e o corpo bruto (A-XXXX). SHA256 ausente é derivada do corpo;
// proveniência vazia vira "UNTRUSTED". Retorna o ID atribuído.
func (s *ArtifactStore) Add(art *AcquiredArtifact, body []byte) (string, error) {
	if art == nil {
		return "", fmt.Errorf("acquisition: artefato nil")
	}
	if strings.TrimSpace(art.URL) == "" {
		return "", fmt.Errorf("acquisition: URL é obrigatória")
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return "", fmt.Errorf("acquisition: criar %q: %w", s.dir, err)
	}
	if art.RetrievedAt.IsZero() {
		art.RetrievedAt = time.Now().UTC()
	}
	if strings.TrimSpace(art.Provenance) == "" {
		art.Provenance = ProvenanceUntrusted
	}
	if strings.TrimSpace(art.SHA256) == "" {
		sum := sha256.Sum256(body)
		art.SHA256 = fmt.Sprintf("%x", sum[:])
		art.SizeBytes = int64(len(body))
	}

	for i := 0; i < 1_000_000; i++ {
		id := s.nextID()
		meta := filepath.Join(s.dir, id+".json")
		// Reivindica o ID atomicamente (O_EXCL), como na quarentena.
		f, err := os.OpenFile(meta, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			if os.IsExist(err) {
				continue
			}
			return "", fmt.Errorf("acquisition: reivindicar %s: %w", meta, err)
		}
		_ = f.Close()

		art.ID = id
		if err := s.writeMeta(art); err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(s.dir, id), body, 0o600); err != nil {
			return "", fmt.Errorf("acquisition: gravar corpo %s: %w", id, err)
		}
		return id, nil
	}
	return "", fmt.Errorf("acquisition: não foi possível atribuir um ID A-XXXX após muitas tentativas")
}

// Get lê os metadados do artefato A-XXXX.
func (s *ArtifactStore) Get(id string) (*AcquiredArtifact, error) {
	norm, err := NormalizeArtifactID(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(s.dir, norm+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("acquisition: artefato %s não encontrado", norm)
		}
		return nil, fmt.Errorf("acquisition: ler %s: %w", norm, err)
	}
	var art AcquiredArtifact
	if err := json.Unmarshal(data, &art); err != nil {
		return nil, fmt.Errorf("acquisition: parsear %s: %w", norm, err)
	}
	return &art, nil
}

// List retorna todos os artefatos adquiridos, ordenados por ID. Um diretório
// ausente/vazio resulta em lista vazia, não erro.
func (s *ArtifactStore) List() ([]AcquiredArtifact, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("acquisition: ler %q: %w", s.dir, err)
	}
	var arts []AcquiredArtifact
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), artifactIDPrefix) || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		art, err := s.Get(strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		arts = append(arts, *art)
	}
	sort.Slice(arts, func(i, j int) bool { return arts[i].ID < arts[j].ID })
	return arts, nil
}

// ReadBody lê o corpo bruto do artefato A-XXXX.
func (s *ArtifactStore) ReadBody(id string) ([]byte, error) {
	norm, err := NormalizeArtifactID(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(s.dir, norm))
	if err != nil {
		return nil, fmt.Errorf("acquisition: ler corpo %s: %w", norm, err)
	}
	return data, nil
}

// ---------------------------------------------------------------------------
// Internals
// ---------------------------------------------------------------------------

// writeMeta grava os metadados do artefato atomicamente (tmp + rename).
func (s *ArtifactStore) writeMeta(art *AcquiredArtifact) error {
	data, err := json.MarshalIndent(art, "", "  ")
	if err != nil {
		return fmt.Errorf("acquisition: serializar %s: %w", art.ID, err)
	}
	data = append(data, '\n')
	target := filepath.Join(s.dir, art.ID+".json")
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("acquisition: escrever tmp %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, target); err != nil {
		return fmt.Errorf("acquisition: renomear %q → %q: %w", tmp, target, err)
	}
	return nil
}

// nextID varre .cosca/quarantine/artifacts e retorna o próximo ID sequencial
// (A-0001 quando nenhum existe). IDs nunca são reutilizados.
func (s *ArtifactStore) nextID() string {
	max := 0
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return fmt.Sprintf("%s%0*d", artifactIDPrefix, artifactIDWidth, 1)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), artifactIDPrefix) || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(strings.TrimPrefix(e.Name(), artifactIDPrefix), ".json")
		n, convErr := strconv.Atoi(id)
		if convErr != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return fmt.Sprintf("%s%0*d", artifactIDPrefix, artifactIDWidth, max+1)
}

// NormalizeArtifactID aceita "A-0001", "a-0001", "A-1" e "0001" e devolve a
// forma canônica zero-padded "A-0001".
func NormalizeArtifactID(id string) (string, error) {
	digits := strings.TrimPrefix(strings.TrimSpace(strings.ToUpper(id)), artifactIDPrefix)
	n, err := strconv.Atoi(digits)
	if err != nil || n < 0 {
		return "", fmt.Errorf("acquisition: ID inválido %q (esperava A-0001)", id)
	}
	return fmt.Sprintf("%s%0*d", artifactIDPrefix, artifactIDWidth, n), nil
}
