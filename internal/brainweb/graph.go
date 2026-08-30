// Package brainweb serves the Cosca "cérebro 3D" visualizer: a single-page
// WebGL/Three.js graph of the organization (agents = capos, skills =
// soldados), served by the Go REST server via go:embed.
//
// SECURITY (ordem do Don): o contrato de dados é uma PROJEÇÃO MÍNIMA
// SANITIZADA — apenas organograma. NÃO expõe: instructions/governance de
// skills, tools/capabilities/dependencies de agents, conversas inter-departamentais,
// inventário de hardware. Campos sensíveis ficam em endpoints restritos.
package brainweb

import (
	"sort"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/agents"
	"github.com/CoscaAI/cosca/internal/cost"
	"github.com/CoscaAI/cosca/internal/skills"
)

// Colleagues são os vínculos hierárquicos que o Don reconhece como raiz da
// organização, em ordem de precedência. São listados explicitamente (não é
// um dado derivado) para dar identidade visual ao núcleo (Don → Kernel → ...).
var rootTier = []string{"Don", "Kernel"}

// Graph é o payload entregue a GET /brain/graph. Todo campo é a projeção
// mínima aprovada — nada sensível.
type Graph struct {
	Meta        Meta      `json:"meta"`
	Departments []string  `json:"departments"`
	Nodes       []Node    `json:"nodes"`
	Edges       []Edge    `json:"edges"`
	Skills      []SkillVz `json:"skills"`
	GeneratedAt time.Time `json:"generated_at"`
}

// Meta carrega contagens agregadas para o painel/HUD.
type Meta struct {
	Agents      int    `json:"agents"`
	Skills      int    `json:"skills"`
	Departments int    `json:"departments"`
	Root        string `json:"root"`
	Version     string `json:"version"`
}

// Node é um agente (capo) na visão sanitizada.
type Node struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	Department  string `json:"department"`
	ReportsTo   string `json:"reports_to"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Tier        int      `json:"tier"`      // 0=Don/Kernel, 1=tenente, 2=capo
	IsRoot      bool     `json:"is_root"`   // Don ou Kernel
	IsKernel    bool     `json:"is_kernel"` // Kernel (consigliere)
	SkillCount  int      `json:"skill_count"`
	Cost        NodeCost `json:"cost"` // Token Efficiency/Energy (intensidade operacional, não nota)
}

// Edge é um vínculo "reports to" entre dois agentes.
type Edge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Kind   string `json:"kind"` // "reports_to" | "department"
}

// SkillVz é uma skill (soldado) na visão sanitizada — sem instructions/governance.
type SkillVz struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Domain      string `json:"domain"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Standard    bool   `json:"standard"`
}

// Activity é uma ação recente do cérebro (read-only). Modelada a partir do
// shape real de executions: quem (agent) fez o quê, quando, e com que estado.
// Projeção mínima — NUNCA expõe prompt/resposta/args de comando (conteúdo
// sensível do usuário). O campo Action carrega apenas o rótulo seguro da
// ação (ex.: a constante "COMMAND_EXECUTED"), nunca a entrada do usuário.
type Activity struct {
	ID         string   `json:"id"`
	Agent      string   `json:"agent"`
	Status     string   `json:"status"`
	DurationMs int64    `json:"duration_ms"`
	SkillsUsed []string `json:"skills_used,omitempty"`
	Provider   string   `json:"provider,omitempty"`
	Model      string   `json:"model,omitempty"`
	Action     string   `json:"action,omitempty"`     // rótulo seguro (constante), nunca args
	Description string  `json:"description,omitempty"` // nome do comando/ação (seguro — nunca args sensíveis)
	At         int64    `json:"at"`                    // unix (ms)
}

// Builder monta o grafo a partir dos managers. É a fonte única do shape.
type Builder struct {
	agents  *agents.Manager
	skills  *skills.Manager
	version string
	cost    *cost.Store // telemetria de custo (ADR-031); nil-safe (projeção neutra)
}

// NewBuilder cria um Builder. managers nil-safe: grafo vazio, nunca pânico.
func NewBuilder(agentsMgr *agents.Manager, skillsMgr *skills.Manager, version string) *Builder {
	return &Builder{agents: agentsMgr, skills: skillsMgr, version: version}
}

// WithCost injeta a fonte de telemetria de custo (cost.Store) para preencher a
// projeção de Token Efficiency/Energy por agente. Nil-safe: sem store (ou ileso)
// → projeção vazia/neutra, nunca pânico — mantém o padrão dos managers.
func (b *Builder) WithCost(store *cost.Store) *Builder {
	b.cost = store
	return b
}

// Build produz o grafo sanitizado.
func (b *Builder) Build() Graph {
	g := Graph{
		Departments: []string{},
		Nodes:       []Node{},
		Edges:       []Edge{},
		Skills:      []SkillVz{},
		GeneratedAt: time.Now().UTC(),
	}

	// --- Agentes (capos) e suas arestas ---
	agentByKey := make(map[string]agents.Agent)
	if b.agents != nil {
		for _, a := range b.agents.List() {
			key := strings.ToLower(strings.TrimSpace(a.Name))
			agentByKey[key] = a
		}
	}

	// skill->domain: agrupamos skills por categoria, como "halo" do domínio.
	skillByDomain := make(map[string]int)
	if b.skills != nil {
		for _, s := range b.skills.List() {
			sanitized := sanitizeSkill(s)
			g.Skills = append(g.Skills, sanitized)
			domain := strings.ToLower(strings.TrimSpace(sanitized.Domain))
			skillByDomain[domain]++
		}
	}
	sortSkills(g.Skills)

	// Nós de agentes — ordem determinística.
	costAgg := newCostAggregator(b.cost) // nil-safe: projeção neutra sem store
	if b.agents != nil {
		for _, a := range b.agents.List() {
			n := sanitizeNode(a)
			n.SkillCount = skillByDomain[strings.ToLower(strings.TrimSpace(a.Name))]
			n.Cost = costAgg.projection(a.Name)
			g.Nodes = append(g.Nodes, n)
		}
	}
	sortNodes(g.Nodes)

	// Arestas "reports to" (organograma real), só entre nós que existem.
	nodeSet := make(map[string]bool)
	for _, n := range g.Nodes {
		nodeSet[strings.ToLower(strings.TrimSpace(n.ID))] = true
	}
	for _, a := range g.Nodes {
		rt := strings.ToLower(strings.TrimSpace(a.ReportsTo))
		if rt == "" {
			continue
		}
		// Se o reports_to existe como agente, liga; senão liga ao Don (raiz).
		target := rt
		if !nodeSet[rt] {
			// Tenta casar por nome aproximado (case-insensitive).
			if found, ok := findAgentKey(rt, agentByKey); ok {
				target = found
			} else {
				target = "don"
			}
		}
		if target != "" && target != strings.ToLower(a.ID) {
			g.Edges = append(g.Edges, Edge{Source: a.ID, Target: target, Kind: "reports_to"})
		}
	}

	// Nó raiz explícito (Don) — sempre presente para dar o "núcleo".
	if !nodeSet["don"] {
		g.Nodes = append(g.Nodes, Node{
			ID: "don", Name: "Don", Role: "Patriarca da Famiglia",
			Department: "kernel", ReportsTo: "", Description: "O Don — a autoridade máxima. Toda decisão estratégica passa por ele.",
			Status: "active", Tier: 0, IsRoot: true,
			Cost: NodeCost{Energy: EnergyNeutral}, // nó sintético: sem telemetria real
		})
		nodeSet["don"] = true
	}

	// Arestas de "department" para agrupar visualmente os capos de um domínio.
	deptEdges := departmentEdges(g.Nodes)
	g.Edges = append(g.Edges, deptEdges...)
	dedupeEdges(&g.Edges)

	// Metadados.
	g.Meta = Meta{
		Agents:      len(g.Nodes),
		Skills:      len(g.Skills),
		Departments: len(g.Departments),
		Root:        "Don",
		Version:     b.version,
	}
	g.Departments = collectDepartments(g.Nodes)
	return g
}

// sanitizeNode projeta um agente para a visão mínima (retira tools/capabilities/deps).
func sanitizeNode(a agents.Agent) Node {
	n := Node{
		ID:          strings.ToLower(strings.TrimSpace(a.Name)),
		Name:        a.Name,
		Role:        a.Role,
		Department:  strings.ToLower(strings.TrimSpace(a.Department)),
		ReportsTo:   a.ReportsTo,
		Description: a.Description,
		Status:      a.Status,
	}
	n.Tier = classifyTier(n)
	n.IsRoot = n.ID == "don"
	n.IsKernel = n.ID == "kernel"
	return n
}

// sanitizeSkill projeta uma skill para a visão mínima (sem instructions/governance).
func sanitizeSkill(s skills.Skill) SkillVz {
	domain := s.Category
	if domain == "" {
		domain = inferDomain(s.Name)
	}
	return SkillVz{
		ID:          strings.ToLower(strings.TrimSpace(s.Name)),
		Name:        s.Name,
		Domain:      strings.ToLower(strings.TrimSpace(domain)),
		Description: s.Description,
		Category:    s.Category,
		Standard:    s.Standard,
	}
}

// classifyTier classifica o nó por seu papel na hierarquia.
func classifyTier(n Node) int {
	for i, r := range rootTier {
		if n.ID == strings.ToLower(r) || strings.EqualFold(n.Name, r) {
			return i
		}
	}
	upper := strings.ToUpper(n.Role)
	if strings.Contains(upper, "CEO") || strings.Contains(upper, "CTO") ||
		strings.Contains(upper, "GOVERNANCE") || strings.Contains(upper, "PRODUCT") ||
		strings.Contains(upper, "CRITIC") || strings.Contains(upper, "PARADIGM") {
		return 1
	}
	return 2
}

// collectDepartments extrai departamentos distintos, ordenados.
func collectDepartments(nodes []Node) []string {
	seen := make(map[string]bool)
	var out []string
	for _, n := range nodes {
		if n.Department == "" || seen[n.Department] {
			continue
		}
		seen[n.Department] = true
		out = append(out, n.Department)
	}
	sort.Strings(out)
	return out
}

// departmentEdges cria arestas departamento->os nós do domínio (para clustering).
func departmentEdges(nodes []Node) []Edge {
	var edges []Edge
	for _, n := range nodes {
		if n.Department == "" || n.IsRoot {
			continue
		}
		edges = append(edges, Edge{Source: n.ID, Target: n.Department, Kind: "department"})
	}
	return edges
}

// dedupeEdges remove arestas duplicadas (mesmo source/target/kind).
func dedupeEdges(edges *[]Edge) {
	seen := make(map[string]bool)
	out := (*edges)[:0]
	for _, e := range *edges {
		key := e.Source + "|" + e.Target + "|" + e.Kind
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, e)
	}
	*edges = out
}

// findAgentKey casa um reports_to com a chave de um agente (case-insensitive).
func findAgentKey(rt string, agentByKey map[string]agents.Agent) (string, bool) {
	for key, a := range agentByKey {
		if strings.EqualFold(key, rt) || strings.EqualFold(a.Role, rt) {
			return key, true
		}
	}
	return "", false
}

// inferDomain deriva um domínio do nome da skill, para skills sem categoria.
func inferDomain(name string) string {
	l := strings.ToLower(name)
	switch {
	case strings.Contains(l, "test"):
		return "testing"
	case strings.Contains(l, "secu"), strings.Contains(l, "crypt"):
		return "security"
	case strings.Contains(l, "database"), strings.Contains(l, "sql"):
		return "database"
	case strings.Contains(l, "api"), strings.Contains(l, "http"):
		return "api"
	case strings.Contains(l, "frontend"), strings.Contains(l, "ui"), strings.Contains(l, "component"):
		return "frontend"
	case strings.Contains(l, "docs"), strings.Contains(l, "adr"):
		return "documentation"
	case strings.Contains(l, "architect"):
		return "architecture"
	case strings.Contains(l, "devops"), strings.Contains(l, "ci"), strings.Contains(l, "docker"):
		return "devops"
	case strings.Contains(l, "monitor"), strings.Contains(l, "metric"):
		return "monitoring"
	case strings.Contains(l, "ml"), strings.Contains(l, "model"), strings.Contains(l, "embed"):
		return "ai"
	default:
		return "general"
	}
}

// sortNodes ordena os nós por ID.
func sortNodes(nodes []Node) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})
}

// sortSkills ordena as skills por ID.
func sortSkills(skills []SkillVz) {
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].ID < skills[j].ID
	})
}
