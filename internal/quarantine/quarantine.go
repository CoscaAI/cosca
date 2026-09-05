// Package quarantine implements the "zona de quarentena epistemológica" for
// the Cosca.
//
// Tudo o que a IA inventa (proposals) NÃO entra diretamente em
// knowledge/laws/memory. Passa primeiro pela quarentena:
//
//	proposal → quarantine → validation → evidence → promotion (ou archival)
//
// É um garbage collector epistemológico: a Cosca arquiva o que não sobrevive
// à validação — nunca deleta, só arquiva (princípio do curador).
//
// Cada proposal vive em .cosca/quarantine/Q-XXXX.json. A promoção para
// laws/knowledge é uma etapa manual/aprovada separada — este pacote apenas
// registra o alvo da promoção (PromotedTo); NÃO escreve em laws.json.
package quarantine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ID prefix and zero-padding for quarantine proposals (Q-0003).
const (
	idPrefix = "Q-"
	idWidth  = 4
)

// Diretórios da zona de quarentena (relativos a .cosca/).
const (
	Dir        = "quarantine"         // .cosca/quarantine
	ArchiveDir = "quarantine/archive" // .cosca/quarantine/archive (arquivados)
)

// Status de uma proposal na zona de quarentena.
const (
	StatusPending    = "pending"    // aguardando validação
	StatusValidating = "validating" // em validação
	StatusPromoted   = "promoted"   // promovida (alvo registrado em PromotedTo)
	StatusDiscarded  = "discarded"  // descartada/arquivada
)

// Proposal é uma invenção da IA aguardando validação antes de entrar no
// conhecimento. O conteúdo NUNCA vai direto para knowledge/laws/memory.
type Proposal struct {
	ID         string    `json:"id"` // Q-000X auto-increment
	Title      string    `json:"title"`
	Content    string    `json:"content"` // o texto da proposta da IA
	Source     string    `json:"source"`  // provider/agente que propôs
	Status     string    `json:"status"`  // pending|validating|promoted|discarded
	CreatedAt  time.Time `json:"created_at"`
	PromotedTo string    `json:"promoted_to,omitempty"` // ex.: "K-06" ou "learning:xyz"
}

// Store é a zona de quarentena sobre .cosca/quarantine/ do projeto.
type Store struct {
	root    string // projeto raiz (contém .cosca/)
	dir     string // .cosca/quarantine
	archive string // .cosca/quarantine/archive
}

// NewStore cria a zona de quarentena para o projeto raiz dir. A operação é
// preguiçosa: os diretórios .cosca/quarantine e .cosca/quarantine/archive
// são criados na primeira escrita (Add/Discard). Permissões 0700 no diretório.
func NewStore(dir string) *Store {
	return &Store{
		root:    dir,
		dir:     filepath.Join(dir, ".cosca", Dir),
		archive: filepath.Join(dir, ".cosca", filepath.FromSlash(ArchiveDir)),
	}
}

// Path retorna o diretório ativo da quarentena (.cosca/quarantine).
func (s *Store) Path() string {
	return s.dir
}

// ArchivePath retorna o diretório de arquivamento (.cosca/quarantine/archive).
func (s *Store) ArchivePath() string {
	return s.archive
}

// Add atribui um ID Q-XXXX auto-incremental à proposal e grava
// .cosca/quarantine/Q-XXXX.json. A gravação é atômica (tmp + rename) e o ID
// é reivindicado com O_CREATE|O_EXCL, então chamadas concorrentes nunca
// colidem. Retorna o ID atribuído (ex.: "Q-0001").
func (s *Store) Add(p Proposal) (string, error) {
	if strings.TrimSpace(p.Title) == "" || strings.TrimSpace(p.Content) == "" {
		return "", fmt.Errorf("quarantine: title e content são obrigatórios")
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return "", fmt.Errorf("quarantine: criar %q: %w", s.dir, err)
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	if p.Status == "" {
		p.Status = StatusPending
	}

	for i := 0; i < 1_000_000; i++ {
		id := s.nextID()
		p.ID = id
		target := s.proposalPath(id)

		// Reivindica o ID atomicamente (O_EXCL). Se outro Add concorrente
		// tomou o mesmo ID, re-tenta com o próximo.
		f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			if os.IsExist(err) {
				continue
			}
			return "", fmt.Errorf("quarantine: reivindicar %s: %w", target, err)
		}
		_ = f.Close()

		if err := s.writeAtomic(p); err != nil {
			return "", err
		}
		return id, nil
	}
	return "", fmt.Errorf("quarantine: não foi possível atribuir um ID Q-XXXX após muitas tentativas")
}

// List retorna todas as proposals ativas (fora do archive), ordenadas por ID.
// Um diretório de quarentena ausente/vazio resulta em lista vazia, não erro.
func (s *Store) List() ([]Proposal, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("quarantine: ler %q: %w", s.dir, err)
	}

	var props []Proposal
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), idPrefix) || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		p, err := s.Get(proposalIDFromFile(e.Name()))
		if err != nil {
			return nil, err
		}
		props = append(props, *p)
	}
	sort.Slice(props, func(i, j int) bool { return props[i].ID < props[j].ID })
	return props, nil
}

// Get lê a proposal Q-XXXX. Procura primeiro no diretório ativo e, como
// fallback, no archive — uma proposal arquivada nunca é perdida (o curador
// nunca deleta, só arquiva).
func (s *Store) Get(id string) (*Proposal, error) {
	norm, err := NormalizeID(id)
	if err != nil {
		return nil, err
	}
	for _, dir := range []string{s.dir, s.archive} {
		data, err := os.ReadFile(filepath.Join(dir, norm+".json"))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("quarantine: ler %s em %q: %w", norm, dir, err)
		}
		var p Proposal
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("quarantine: parsear %s em %q: %w", norm, dir, err)
		}
		return &p, nil
	}
	return nil, fmt.Errorf("quarantine: proposal %s não encontrada", norm)
}

// SetStatus valida e aplica a transição de status, persistindo o arquivo.
// Transições permitidas:
//
//	pending    → validating
//	validating → promoted   (exige PromotedTo definido)
//	validating → discarded
//
// SetStatus apenas grava o status; arquivar (mover para archive/) é função
// de Discard. NÃO move arquivos.
func (s *Store) SetStatus(id, status string) error {
	p, err := s.Get(id)
	if err != nil {
		return err
	}
	if err := s.applyTransition(p.Status, status); err != nil {
		return err
	}
	if status == StatusPromoted && strings.TrimSpace(p.PromotedTo) == "" {
		return fmt.Errorf("quarantine: promoção exige PromotedTo definido (use Promote, não SetStatus)")
	}
	p.Status = status
	return s.writeAtomic(*p)
}

// Promote marca a proposal como promovida e registra o alvo da promoção
// (ex.: "K-06" ou "learning:xyz"). A promoção EXIGE que a proposal esteja em
// validating (validação concluída). Este método NÃO escreve em laws.json —
// a promoção ao conhecimento é uma etapa separada, manual/aprovada pelo Don.
func (s *Store) Promote(id, targetRef string) error {
	if strings.TrimSpace(targetRef) == "" {
		return fmt.Errorf("quarantine: promote exige um alvo (ex.: --to K-06)")
	}
	p, err := s.Get(id)
	if err != nil {
		return err
	}
	if p.Status != StatusValidating {
		return fmt.Errorf("quarantine: %s não pode ser promovida (status %q — valide antes)", p.ID, p.Status)
	}
	p.Status = StatusPromoted
	p.PromotedTo = targetRef
	return s.writeAtomic(*p)
}

// Discard arquiva a proposal: o status vira "discarded" e o arquivo é movido
// para .cosca/quarantine/archive/. NUNCA deleta — princípio do curador.
func (s *Store) Discard(id string) error {
	p, err := s.Get(id)
	if err != nil {
		return err
	}
	if err := s.applyTransition(p.Status, StatusDiscarded); err != nil {
		return err
	}
	if err := os.MkdirAll(s.archive, 0o700); err != nil {
		return fmt.Errorf("quarantine: criar archive %q: %w", s.archive, err)
	}
	p.Status = StatusDiscarded
	if err := s.writeAtomic(*p); err != nil {
		return err
	}
	src := s.proposalPath(p.ID)
	dst := filepath.Join(s.archive, p.ID+".json")
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("quarantine: arquivar %s: %w", src, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Internals
// ---------------------------------------------------------------------------

// applyTransition valida a máquina de estados da quarentena.
func (s *Store) applyTransition(from, to string) error {
	if from == to {
		return nil
	}
	allowed := map[string]map[string]bool{
		StatusPending:    {StatusValidating: true},
		StatusValidating: {StatusPromoted: true, StatusDiscarded: true},
	}
	if dests, ok := allowed[from]; !ok || !dests[to] {
		return fmt.Errorf(
			"quarantine: transição inválida %s → %s (permitidas: %s)",
			from, to, TransitionTable())
	}
	return nil
}

// nextID varre .cosca/quarantine e .cosca/quarantine/archive e retorna o
// próximo ID sequencial (Q-0001 quando nenhum existe). Varre ambos os
// diretórios para que IDs arquivados nunca sejam reutilizados.
func (s *Store) nextID() string {
	max := 0
	for _, dir := range []string{s.dir, s.archive} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			id := proposalIDFromFile(e.Name())
			if id == "" {
				continue
			}
			n, convErr := strconv.Atoi(strings.TrimPrefix(id, idPrefix))
			if convErr != nil {
				continue
			}
			if n > max {
				max = n
			}
		}
	}
	return fmt.Sprintf("%s%0*d", idPrefix, idWidth, max+1)
}

// proposalPath retorna o caminho do arquivo ativo da proposal.
func (s *Store) proposalPath(id string) string {
	return filepath.Join(s.dir, id+".json")
}

// writeAtomic grava a proposal de forma atômica: tmp + rename no diretório
// ativo. O diretório já deve existir.
func (s *Store) writeAtomic(p Proposal) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("quarantine: serializar %s: %w", p.ID, err)
	}
	data = append(data, '\n')

	target := s.proposalPath(p.ID)
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("quarantine: escrever tmp %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, target); err != nil {
		return fmt.Errorf("quarantine: renomear %q → %q: %w", tmp, target, err)
	}
	return nil
}

// proposalIDFromFile extrai "Q-0001" de "Q-0001.json" ("" se não casar).
func proposalIDFromFile(name string) string {
	if !strings.HasPrefix(name, idPrefix) || !strings.HasSuffix(name, ".json") {
		return ""
	}
	return strings.TrimSuffix(name, ".json")
}

// NormalizeID aceita "Q-0001", "q-0001", "Q-1" e "0001" e devolve a forma
// canônica zero-padded "Q-0001".
func NormalizeID(id string) (string, error) {
	digits := strings.TrimPrefix(strings.TrimSpace(strings.ToUpper(id)), idPrefix)
	n, err := strconv.Atoi(digits)
	if err != nil || n < 0 {
		return "", fmt.Errorf("quarantine: ID inválido %q (esperava Q-0001)", id)
	}
	return fmt.Sprintf("%s%0*d", idPrefix, idWidth, n), nil
}

// TransitionTable documenta a máquina de estados da quarentena.
func TransitionTable() string {
	return "pending → validating → promoted | discarded"
}
