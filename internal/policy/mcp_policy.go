package policy

import (
	"errors"
	"strings"
)

// MCPPolicy é a camada de controles default-deny para ferramentas MCP
// (ADR-011 Security, Bloco 2). Diferente do Engine, que Decide por regras, o
// MCPPolicy parte de DENY por padrão: só uma tool explicitamente allowlistada
// e com a capability necessária habilitada pode rodar. Shell/Network/FileWrite
// exigem allowlist explícita (P1) — nunca são permitidas por padrão.
type MCPPolicy struct {
	// AllowedTools é o allowlist de ferramentas permitidas. Uma tool que não
	// está aqui é DENY (default-deny). A chave é o nome da tool (case-sensitive).
	AllowedTools map[string]bool
	// Shell habilita ferramentas que executam comandos (bash/exec/...).
	Shell bool
	// Network habilita ferramentas que acessam a rede (curl/fetch/http/...).
	Network bool
	// FileWrite habilita ferramentas que escrevem/altera arquivos.
	FileWrite bool
	// MaxToolCallsPerTurn limita o número de chamadas de tool por turno
	// (default 200, ver DefaultMaxToolCalls).
	MaxToolCallsPerTurn int
	// AuditLog ativa o registro de auditoria das decisões (default true).
	AuditLog bool
}

// NewMCPPolicy cria um MCPPolicy com os defaults da casa: default-deny para
// capabilities perigosas e orçamento de 200 tool-calls por turno.
func NewMCPPolicy() *MCPPolicy {
	return &MCPPolicy{
		AllowedTools:        map[string]bool{},
		Shell:               false,
		Network:             false,
		FileWrite:           false,
		MaxToolCallsPerTurn: DefaultMaxToolCalls,
		AuditLog:            true,
	}
}

// Capability descreve a classe de risco de uma tool.
type Capability int

const (
	// CapNone — tool sem capability perigosa (leitura, computação pura).
	CapNone Capability = iota
	// CapShell — tool que executa comandos (bash/exec/...).
	CapShell
	// CapNetwork — tool que acessa a rede.
	CapNetwork
	// CapFileWrite — tool que escreve/altera arquivos.
	CapFileWrite
)

func (c Capability) String() string {
	switch c {
	case CapShell:
		return "shell"
	case CapNetwork:
		return "network"
	case CapFileWrite:
		return "file-write"
	default:
		return "none"
	}
}

// toolCapability classifica o nome de uma tool na classe de risco
// correspondente. A classificação é determinística por nome (não parser de
// shell complexo — over-engineering).
func toolCapability(tool string) Capability {
	switch strings.ToLower(strings.TrimSpace(tool)) {
	case "bash", "sh", "zsh", "exec", "run", "shell", "cmd", "powershell", "pwsh", "command":
		return CapShell
	case "http", "https", "fetch", "curl", "wget", "request", "get", "post", "put",
		"web", "network", "url", "browser", "api":
		return CapNetwork
	case "write", "edit", "delete", "remove", "mkdir", "move", "copy", "create", "rm":
		return CapFileWrite
	default:
		return CapNone
	}
}

// Evaluate aplica a política default-deny a uma chamada de tool. Retorna a
// Decision (não recria o enum — reusa Allow/Deny do pacote) e um error apenas
// para uso inválido (policy nil, tool vazia). Uma Deny é uma Decision, não um
// error — não deve ser confundida com falha técnica da política.
//
// Regras (em ordem de precedência):
//
//  1. policy nil           → error (não há como decidir).
//  2. tool vazia          → error.
//  3. tool fora de AllowedTools → Deny (default-deny).
//  4. tool de shell       sem Shell=true      → Deny.
//  5. tool de network     sem Network=true    → Deny.
//  6. tool de file-write  sem FileWrite=true  → Deny.
//  7. caso contrário      → Allow.
func (p *MCPPolicy) Evaluate(tool string, _ map[string]any) (Decision, error) {
	if p == nil {
		return Deny, errors.New("mcp-policy: nil receiver")
	}
	if strings.TrimSpace(tool) == "" {
		return Deny, errors.New("mcp-policy: empty tool name")
	}
	if !p.AllowedTools[tool] {
		return Deny, nil
	}
	switch toolCapability(tool) {
	case CapShell:
		if !p.Shell {
			return Deny, nil
		}
	case CapNetwork:
		if !p.Network {
			return Deny, nil
		}
	case CapFileWrite:
		if !p.FileWrite {
			return Deny, nil
		}
	}
	return Allow, nil
}
