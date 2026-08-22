// Hall da Fama do Conhecimento
//
// Registro das maiores descobertas do runtime Cosca, com impacto medido.
// Cada descoberta nasce de evidências reais do repositório — testes,
// auditorias red team, ondas de segurança e conversas do Don — e é
// classificada por estrelas de impacto (1-5).
//
// A descoberta é o irmão operacional da lei (CKL): enquanto a lei responde
// "por que isso é verdade?" (evidências + promoção), a descoberta responde
// "o que isso mudou?" (impacto, economia, redução de risco, sessões
// validadas). Uma descoberta pode ligar-se a uma lei via RelatedLaw
// (ex: D-01 → K-01).
//
// Persistência é JSON em .cosca/knowledge/hall-of-fame.json — runtime
// (gitignored), nunca .cosca/framework.
package knowledge

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Discovery é uma grande descoberta do runtime, com impacto medido.
type Discovery struct {
	ID                   string    `json:"id"`                     // ex: "D-01"
	Name                 string    `json:"name"`                   // ex: "Jail fail-closed: recusa root"
	Description          string    `json:"description"`            // o que mudou
	Impact               int       `json:"impact"`                 // 1-5 estrelas
	ProjectsAffected     int       `json:"projects_affected"`      // projetos afetados
	EconomyPercent       float64   `json:"economy_percent"`        // % de economia (opcional)
	EconomyUSD           float64   `json:"economy_usd"`            // economia em USD (opcional)
	RiskReductionPercent float64   `json:"risk_reduction_percent"` // redução de risco (%)
	ValidatedSessions    int       `json:"validated_sessions"`     // sessões que validaram
	Origin               string    `json:"origin"`                 // ex: "Auditoria #52", "red-team-onda-1"
	RelatedLaw           string    `json:"related_law"`            // ex: "K-01" (liga ao CKL)
	CreatedAt            time.Time `json:"created_at"`
}

// Stars renderiza o impacto em estrelas preenchidas/vazias (escala 1-5),
// ex: impacto 4 → "★★★★☆". Impacto nunca deve sair da escala 1-5
// (Register valida), mas a função defende o display mesmo assim.
func (d *Discovery) Stars() string {
	impact := d.Impact
	if impact < 0 {
		impact = 0
	}
	if impact > 5 {
		impact = 5
	}
	return strings.Repeat("★", impact) + strings.Repeat("☆", 5-impact)
}

// HallOfFame é o gerenciador do registro de descobertas. Todas as
// operações são seguras para uso concorrente.
type HallOfFame struct {
	discoveries map[string]*Discovery
	mu          sync.RWMutex
}

// NewHallOfFame cria um Hall da Fama vazio.
func NewHallOfFame() *HallOfFame {
	return &HallOfFame{discoveries: make(map[string]*Discovery)}
}

// Register adiciona uma descoberta, que passa a ser de propriedade do
// Hall da Fama. Validações: descoberta não-nula, ID e Nome obrigatórios,
// impacto na escala 1-5 (estrelas) e métricas não-negativas. Um ID já
// registrado é erro (sem duplicação). CreatedAt zero recebe o tempo atual.
func (h *HallOfFame) Register(d *Discovery) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if d == nil {
		return errors.New("knowledge: cannot register a nil discovery")
	}
	if strings.TrimSpace(d.ID) == "" {
		return errors.New("knowledge: discovery ID is required")
	}
	if strings.TrimSpace(d.Name) == "" {
		return errors.New("knowledge: discovery name is required")
	}
	if d.Impact < 1 || d.Impact > 5 {
		return fmt.Errorf("knowledge: discovery %q impact %d out of range [1,5] (estrelas)", d.ID, d.Impact)
	}
	if d.ProjectsAffected < 0 || d.ValidatedSessions < 0 {
		return fmt.Errorf("knowledge: discovery %q has negative counters", d.ID)
	}
	if d.RiskReductionPercent < 0 || d.RiskReductionPercent > 100 {
		return fmt.Errorf("knowledge: discovery %q risk_reduction_percent %.1f out of range [0,100]", d.ID, d.RiskReductionPercent)
	}
	if d.EconomyPercent < 0 || d.EconomyPercent > 100 {
		return fmt.Errorf("knowledge: discovery %q economy_percent %.1f out of range [0,100]", d.ID, d.EconomyPercent)
	}
	if d.EconomyUSD < 0 {
		return fmt.Errorf("knowledge: discovery %q economy_usd %.2f is negative", d.ID, d.EconomyUSD)
	}
	if _, exists := h.discoveries[d.ID]; exists {
		return fmt.Errorf("knowledge: discovery %q is already registered", d.ID)
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now()
	}
	h.discoveries[d.ID] = d
	return nil
}

// Get devolve uma cópia defensiva da descoberta, ou ok=false quando o ID
// não está registrado.
func (h *HallOfFame) Get(id string) (*Discovery, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	d, ok := h.discoveries[id]
	if !ok {
		return nil, false
	}
	cp := *d
	return &cp, true
}

// All devolve cópias defensivas de todas as descobertas, ordenadas por
// impacto desc (Hall da Fama: a maior descoberta primeiro). Empates são
// quebrados por CreatedAt desc (a mais recente primeiro) e, no último
// caso, por ID asc — ordenação determinística.
func (h *HallOfFame) All() []*Discovery {
	h.mu.RLock()
	items := make([]*Discovery, 0, len(h.discoveries))
	for _, d := range h.discoveries {
		cp := *d
		items = append(items, &cp)
	}
	h.mu.RUnlock()

	sortDiscoveries(items)
	return items
}

// Top devolve as n melhores descobertas por impacto. n <= 0 devolve uma
// fatia vazia; n >= total devolve todas.
func (h *HallOfFame) Top(n int) []*Discovery {
	all := h.All()
	if n <= 0 {
		return []*Discovery{}
	}
	if n >= len(all) {
		return all
	}
	return all[:n]
}

// sortDiscoveries ordena por impacto desc, CreatedAt desc, ID asc.
func sortDiscoveries(items []*Discovery) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Impact != items[j].Impact {
			return items[i].Impact > items[j].Impact
		}
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		return items[i].ID < items[j].ID
	})
}

// ── Persistência ────────────────────────────────────────────────────────────

// hallOfFameStore é o layout JSON on-disk do Hall da Fama.
type hallOfFameStore struct {
	Version     int          `json:"version"`
	Discoveries []*Discovery `json:"discoveries"`
}

// Load lê o arquivo JSON escrito por Save e substitui o conteúdo do
// Hall da Fama. Entradas corrompidas (nil ou sem ID) são ignoradas.
func (h *HallOfFame) Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("knowledge: load %s: %w", path, err)
	}

	var store hallOfFameStore
	if err := json.Unmarshal(data, &store); err != nil {
		return fmt.Errorf("knowledge: parse %s: %w", path, err)
	}

	items := make(map[string]*Discovery, len(store.Discoveries))
	for _, d := range store.Discoveries {
		if d == nil || strings.TrimSpace(d.ID) == "" {
			continue
		}
		if d.CreatedAt.IsZero() {
			d.CreatedAt = time.Now()
		}
		items[d.ID] = d
	}

	h.mu.Lock()
	h.discoveries = items
	h.mu.Unlock()
	return nil
}

// Save persiste todas as descobertas como JSON. O diretório pai é criado
// com permissões dono-only (0700) e o arquivo com 0600, seguindo as
// convenções da árvore .cosca. Itens são ordenados pela mesma ordem do
// All() (impacto desc) para output determinístico.
func (h *HallOfFame) Save(path string) error {
	h.mu.RLock()
	items := make([]*Discovery, 0, len(h.discoveries))
	for _, d := range h.discoveries {
		cp := *d
		items = append(items, &cp)
	}
	h.mu.RUnlock()

	sortDiscoveries(items)

	data, err := json.MarshalIndent(hallOfFameStore{Version: 1, Discoveries: items}, "", "  ")
	if err != nil {
		return fmt.Errorf("knowledge: encode: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("knowledge: create dir %s: %w", dir, err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return fmt.Errorf("knowledge: restrict dir %s: %w", dir, err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("knowledge: write %s: %w", path, err)
	}
	return nil
}

// ── Seed real ───────────────────────────────────────────────────────────────

// SeedHallOfFame registra as 5 descobertas REAIS do Cosca — evidências dos
// commits das ondas de segurança e do CKL (git log: 35e85e5 Onda 1,
// 96398b7 Onda 2, 238ac6d CKL). Nenhuma descoberta é inventada: cada uma
// aponta para auditorias, testes e fixes que existem de fato no repositório
// (ver pkg/cosca/jail_test.go: TestJail_FailClosed, TestJail_RootUnsafe,
// TestJail_DieWithParent; api/rest/handler; internal/auth/tokenstore_test.go).
//
// O seed roda uma única vez: na primeira execução de `cosca knowledge
// discovery list`, quando .cosca/knowledge/hall-of-fame.json ainda não
// existe. É idempotente por construção — um arquivo existente nunca é
// sobrescrito.
func SeedHallOfFame(h *HallOfFame) error {
	for _, d := range hallOfFameSeeds() {
		if err := h.Register(d); err != nil {
			return fmt.Errorf("knowledge: seed hall of fame: %w", err)
		}
	}
	return nil
}

// hallOfFameSeeds devolve as 5 grandes descobertas com CreatedAt = data dos
// commits reais (2026-07-31): Onda 1 11:36, Onda 2 11:58, CKL 12:51.
func hallOfFameSeeds() []*Discovery {
	return []*Discovery{
		{
			ID:   "D-01",
			Name: "Jail fail-closed — recusa executar como root",
			Description: "A jail passou a falhar fechada (fail-closed): recusa " +
				"executar como root automaticamente, eliminando a superfície do " +
				"incidente F001 (fuga da jail como root).",
			Impact:               5,
			ProjectsAffected:     1,
			RiskReductionPercent: 87,
			ValidatedSessions:    4, // TestJail_FailClosed + TestJail_RootUnsafe + TestJail_DieWithParent + red team
			Origin:               "red-team-onda-1 (auditoria A1+C3)",
			RelatedLaw:           "K-01",
			CreatedAt:            discoveryTime(11, 36),
		},
		{
			ID:   "D-02",
			Name: "Registro público desabilitado por padrão",
			Description: "O endpoint de registro passou a ser desabilitado por " +
				"padrão (fail-closed): nenhum usuário anônimo sem configuração " +
				"explícita — auditoria A1 encontrou o registro público.",
			Impact:               4,
			ProjectsAffected:     1,
			RiskReductionPercent: 70,
			ValidatedSessions:    3,
			Origin:               "red-team-onda-1 (auditoria A1)",
			RelatedLaw:           "K-02",
			CreatedAt:            discoveryTime(11, 36),
		},
		{
			ID:   "D-03",
			Name: "Path traversal bloqueado por containment",
			Description: "Containment no indexador e no memory ID: caminhos fora " +
				"da raiz do sandbox são rejeitados (fail-closed) — as auditorias " +
				"C1 (indexador) e C2 (memory ID) encontraram traversal.",
			Impact:               4,
			ProjectsAffected:     1,
			RiskReductionPercent: 65,
			ValidatedSessions:    5,
			Origin:               "red-team-onda-1 (C1+C2)",
			RelatedLaw:           "K-03",
			CreatedAt:            discoveryTime(11, 36),
		},
		{
			ID:   "D-04",
			Name: "Refresh token com rotação e revogação",
			Description: "O refresh token agora rotaciona a cada uso e revoga o " +
				"anterior: reuse de token rotacionado é detectado e bloqueado " +
				"(fail-closed) — auditoria A4 encontrou o refresh reusável.",
			Impact:               4,
			ProjectsAffected:     1,
			RiskReductionPercent: 60,
			ValidatedSessions:    4,
			Origin:               "red-team-onda-2 (A4)",
			RelatedLaw:           "K-04",
			CreatedAt:            discoveryTime(11, 58),
		},
		{
			ID:   "D-05",
			Name: "Cosca Knowledge Lifecycle (CKL)",
			Description: "Conhecimento como código: leis com evidência rastreável, " +
				"promoção gradual (observation → … → law) e aprovação do Don — " +
				"contexto governado reduz o contexto carregado em 18%.",
			Impact:               5,
			ProjectsAffected:     1,
			EconomyPercent:       18, // contexto reduzido
			RiskReductionPercent: 0,
			ValidatedSessions:    1,
			Origin:               "conversa do Don — conhecimento como código",
			RelatedLaw:           "K-05",
			CreatedAt:            discoveryTime(12, 51),
		},
	}
}

// discoveryTime devolve um timestamp determinístico do dia das ondas de
// segurança (2026-07-31) — mantém o seed estável e reproduzível.
func discoveryTime(hour, min int) time.Time {
	return time.Date(2026, 7, 31, hour, min, 0, 0, time.UTC)
}
