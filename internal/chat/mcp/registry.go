// registry.go — MCP Registry/Router com NAMESPACING (gema #6, ADR-022;
// mineração federated-mcp — o gap que ele apontou mas não entregou).
//
// Agrega N servidores MCP numa superfície única de skills dos capos, resolve
// conflito de nomes via namespacing (`server.tool`), e — CRÍTICO (I8) — usa
// DEFAULT-DENY: uma tool só é chamável se o contexto explicito `server.tool`
// resolve para um servidor registrado. Externo NUNCA ganha autoridade; só o
// routing é do registry, a execução é do cliente MCP (que já passa por policy).
//
// A superfície é a "fronteira de capacidade externa" do Cosca:
//
//	KERNEL → decide/authorizes → MCP Adapter (Registry) → tool externa → result
//	→ provenance → ledger.  (ADR-017 §4)
//
// Determinístico (I1), fail-closed (I2).
package mcp

import (
	"fmt"
	"sort"
	"strings"
)

// Registry agrega servidores MCP + suas tools, com namespacing.
type Registry struct {
	servers map[string][]ToolInfo
}

// NewRegistry cria um registry vazio.
func NewRegistry() *Registry {
	return &Registry{servers: map[string][]ToolInfo{}}
}

// Register adiciona (ou substitui) um servidor e seu inventário de tools.
// Fail-closed (I2): nome vazio → erro.
func (r *Registry) Register(name string, tools []ToolInfo) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("mcp registry: server name required (fail-closed)")
	}
	r.servers[name] = append([]ToolInfo(nil), tools...)
	return nil
}

// Remove remove um servidor do registry.
func (r *Registry) Remove(name string) {
	delete(r.servers, name)
}

// Servers devolve os namespaces registrados (ordenado, determinístico).
func (r *Registry) Servers() []string {
	out := make([]string, 0, len(r.servers))
	for name := range r.servers {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// ToolNames devolve todas as tools como "server.tool" (ordenado).
func (r *Registry) ToolNames() []string {
	var out []string
	for name, tools := range r.servers {
		for _, t := range tools {
			out = append(out, name+"."+t.Name)
		}
	}
	sort.Strings(out)
	return out
}

// ResolveCall resolve `server.tool` (namespacing) para (server, tool).
// DEFAULT-DENY (I8): formato inválido, servidor ou tool inexistente → erro
// fail-closed (nunca "chama por nome pelado" nem delega autoridade).
func (r *Registry) ResolveCall(qualified string) (server, tool string, err error) {
	parts := strings.Split(qualified, ".")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("mcp registry: %q not qualified as server.tool (default-deny)", qualified)
	}
	server, tool = parts[0], strings.Join(parts[1:], ".")
	if _, ok := r.servers[server]; !ok {
		return "", "", fmt.Errorf("mcp registry: server %q not registered (default-deny)", server)
	}
	for _, t := range r.servers[server] {
		if t.Name == tool {
			return server, tool, nil
		}
	}
	return "", "", fmt.Errorf("mcp registry: tool %q not found in %q (default-deny)", tool, server)
}

// ResolveTool devolve o descriptor da tool por nome qualificado. Fail-closed.
func (r *Registry) ResolveTool(qualified string) (ToolInfo, error) {
	server, tool, err := r.ResolveCall(qualified)
	if err != nil {
		return ToolInfo{}, err
	}
	for _, t := range r.servers[server] {
		if t.Name == tool {
			return t, nil
		}
	}
	return ToolInfo{}, fmt.Errorf("mcp registry: tool not found")
}
