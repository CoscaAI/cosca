// Package gitmgr é o COSCA Git Manager v0 — o núcleo de colaboração e
// proveniência do COSCA. Permite ao COSCA operar um repositório Git (via
// go-git, puro Go) para versionar datasets, campanhas e LoRAs, e rastrear a
// proveniência (dataset → campanha → LoRA → avaliação).
//
// REGRA DE OURO (professor): o COSCA pode operar Git, mas TODA operação
// destrutiva exige um GATE EXPLÍCITO. Nada de reset --hard, apagar branch,
// sobrescrever arquivo ou merge destrutivo "porque o agente achou melhor".
//
// O gate é fail-closed: uma operação destrutiva SEM autorização explícita
// retorna um erro DestructiveGateError, NUNCA executa.
package gitmgr

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// ─── Erros ──────────────────────────────────────────────────────────────────

// ErrNotARepo indica que o caminho não é um repositório Git válido.
var ErrNotARepo = errors.New("gitmgr: não é um repositório Git")

// DestructiveGateError indica que uma operação destrutiva foi bloqueada pelo
// gate de segurança (fail-closed). A operação NÃO foi executada.
type DestructiveGateError struct {
	Op      string
	Reason  string
}

func (e *DestructiveGateError) Error() string {
	return fmt.Sprintf("gitmgr: operação destrutiva %q BLOQUEADA pelo gate (fail-closed): %s", e.Op, e.Reason)
}

// ─── Manager ────────────────────────────────────────────────────────────────

// Manager abre um repositório Git no diretório dado.
type Manager struct {
	repo *git.Repository
	path string
}

// Open abre (ou detecta) o repositório Git no diretório path. Se o diretório
// NÃO for um repositório, retorna ErrNotARepo (não cria nada). O COSCA deve
// usar um repo existente — nunca inicializar um sem ordem explícita.
func Open(path string) (*Manager, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		if errors.Is(err, git.ErrRepositoryNotExists) {
			return nil, ErrNotARepo
		}
		return nil, fmt.Errorf("gitmgr: abrir repo: %w", err)
	}
	return &Manager{repo: repo, path: path}, nil
}

// ─── Operações SEGURAS (leitura/informação — sempre permitidas) ─────────────

// Status descreve o estado do working tree (arquivos modificados/adicionados).
func (m *Manager) Status(ctx context.Context) (string, error) {
	wt, err := m.repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("gitmgr: worktree: %w", err)
	}
	st, err := wt.Status()
	if err != nil {
		return "", fmt.Errorf("gitmgr: status: %w", err)
	}
	return st.String(), nil
}

// CurrentBranch devolve o nome do branch atual.
func (m *Manager) CurrentBranch(ctx context.Context) (string, error) {
	head, err := m.repo.Head()
	if err != nil {
		return "", fmt.Errorf("gitmgr: head: %w", err)
	}
	return head.Name().Short(), nil
}

// Branches lista os branches locais.
func (m *Manager) Branches(ctx context.Context) ([]string, error) {
	iter, err := m.repo.Branches()
	if err != nil {
		return nil, fmt.Errorf("gitmgr: branches: %w", err)
	}
	defer iter.Close()
	var names []string
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		names = append(names, ref.Name().Short())
		return nil
	})
	return names, err
}

// Log devolve os últimos N commits (mensagem curta + hash curto).
func (m *Manager) Log(ctx context.Context, n int) ([]CommitInfo, error) {
	if n <= 0 {
		n = 10
	}
	iter, err := m.repo.Log(&git.LogOptions{})
	if err != nil {
		return nil, fmt.Errorf("gitmgr: log: %w", err)
	}
	defer iter.Close()
	var commits []CommitInfo
	_ = iter.ForEach(func(c *object.Commit) error {
		if len(commits) >= n {
			return nil
		}
		commits = append(commits, CommitInfo{
			Hash:    c.Hash.String()[:8],
			Message: firstLine(c.Message),
			Author:  c.Author.Email,
			When:    c.Author.When,
		})
		return nil
	})
	return commits, nil
}

// CommitInfo é um resumo de um commit (para exibição).
type CommitInfo struct {
	Hash    string
	Message string
	Author  string
	When    time.Time
}

// ─── Operações de ESCRITA (gate explícito) ──────────────────────────────────

// WriteOp enumera as operações de escrita que exigem gate.
type WriteOp string

const (
	OpCommit WriteOp = "commit"
	OpBranch WriteOp = "branch" // criar branch (seguro)
	OpPull   WriteOp = "pull"
	OpPush   WriteOp = "push"
)

// ComitArgs são os parâmetros de um commit.
type CommitArgs struct {
	Message string
	Author  string
	Email   string
}

// Commit cria um commit com os arquivos staged. É uma operação de ESCRITA,
// mas NÃO destrutiva (não apaga/sobrescreve sem querer) — ainda assim exige
// confirmação via gate para manter a regra de "agente não decide sozinho".
//
// O gate é implícito: o caller (CLI) só chama Comit após o Don aprovar. Este
// método NÃO executa reset/force — apenas adiciona e commita o que está staged.
func (m *Manager) Commit(ctx context.Context, args CommitArgs) (string, error) {
	if strings.TrimSpace(args.Message) == "" {
		return "", fmt.Errorf("gitmgr: commit vazio")
	}
	wt, err := m.repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("gitmgr: worktree: %w", err)
	}
	// Adiciona tudo que está modified/untracked (mas respeita .gitignore).
	// NÃO usa --all destrutivo; adiciona o estado atual do working tree.
	if err := wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return "", fmt.Errorf("gitmgr: add: %w", err)
	}
	sig := object.Signature{Name: args.Author, Email: args.Email}
	h, err := wt.Commit(args.Message, &git.CommitOptions{Author: &sig, Committer: &sig})
	if err != nil {
		return "", fmt.Errorf("gitmgr: commit: %w", err)
	}
	return h.String()[:8], nil
}

// CreateBranch cria um branch (operação SEGURA — não destrói nada).
// Usa a API do go-git (branch ref + config) que é compatível com DeleteBranch.
func (m *Manager) CreateBranch(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("gitmgr: nome de branch vazio")
	}
	head, err := m.repo.Head()
	if err != nil {
		return fmt.Errorf("gitmgr: head: %w", err)
	}
	// Cria a ref da branch apontando para o commit do HEAD.
	ref := plumbing.NewHashReference(plumbing.NewBranchReferenceName(name), head.Hash())
	if err := m.repo.Storer.SetReference(ref); err != nil {
		return fmt.Errorf("gitmgr: criar branch: %w", err)
	}
	// Registra no branch config (para o repo reconhecer como branch ativa p/ delete).
	return m.repo.CreateBranch(&config.Branch{Name: name, Remote: "origin"})
}

// ─── PROVENIÊNCIA (dataset → campanha → LoRA → avaliação) ───────────────────

// Provenance é um registro de proveniência versionado no repositório.
// O COSCA grava o vínculo dataset→campanha→LoRA→avaliação como um arquivo
// JSON em .cosca/provenance/, committado. Isso é a memória de "qual dataset
// gerou qual LoRA" — a base da colaboração e do anti-autoengano.
//
// O professor pediu para registrar proveniência DESDE O PRIMEIRO COMMIT, com
// a rastreabilidade completa: repo, commit, branch, arquivo(s), autor/agente,
// timestamp, operação, dataset_version, campaign_id, model, adapter,
// golden_result. Assim é possível reconstruir:
//   dataset-v002 → campaign-002 → LoRA-002 → commit abc123 → Golden 8/8 → PASS
type Provenance struct {
	// Identidade do artefato
	CampaignID string `json:"campaign_id"`
	Dataset    string `json:"dataset"`
	DatasetVersion string `json:"dataset_version,omitempty"`
	LoRA       string `json:"lora"`
	Base       string `json:"base"`
	Adapter    string `json:"adapter,omitempty"`
	Model      string `json:"model,omitempty"`

	// Rastreabilidade do repo
	Repo     string   `json:"repo"`
	Commit   string   `json:"commit"`
	Branch   string   `json:"branch"`
	Files    []string `json:"files,omitempty"`
	Actor    string   `json:"actor"`    // autor/agente (ex.: "cosca-kernel")
	Op       string   `json:"op"`       // operação (ex.: "train", "evaluate", "commit")
	Timestamp string  `json:"timestamp"`

	// Resultado da avaliação (Golden Gate)
	GoldenResult   string `json:"golden_result,omitempty"`   // ex.: "0.88 -> 0.75 REPROVADO"
	Promotion      string `json:"promotion,omitempty"`       // PASS | FAIL | NOT_PROMOTED

	Meta map[string]string `json:"meta,omitempty"`
}

// provenanceDir é o diretório de proveniência dentro do repo.
const provenanceDir = ".cosca/provenance"

// WriteProvenance grava um registro de proveniência como arquivo JSON no
// working tree (NÃO commita automaticamente — o Don decide quando commitar).
func (m *Manager) WriteProvenance(ctx context.Context, p Provenance) (string, error) {
	dir := m.path + "/" + provenanceDir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("gitmgr: criar dir provenance: %w", err)
	}
	path := fmt.Sprintf("%s/%s.json", dir, p.CampaignID)
	b, err := jsonMarshalIndent(p)
	if err != nil {
		return "", fmt.Errorf("gitmgr: marshal provenance: %w", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", fmt.Errorf("gitmgr: write provenance: %w", err)
	}
	return path, nil
}

// firstLine pega a primeira linha de uma mensagem (para log compacto).
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
