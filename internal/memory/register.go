package memory

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/memoryguard"
)

const (
	genesisHash     = "0000000000000000000000000000000000000000000000000000000000000000"
	merkleEpochSize = 32
)

// LearningInput carries the fields that build a learning block. The
// mechanical parts — PREV, ID, TIME, hash, chain.dat, merkle — are derived
// automatically. See MEMORY_ACCESS_PROTOCOL.md §3.
type LearningInput struct {
	Agent      string
	Title      string
	Level      int
	Tags       []string
	Task       string
	Technique  string
	Outcome    string
	Confidence float64
	Learned    string
	Next       string
	Related    string
}

// LearningResult reports what RegisterLearning produced.
type LearningResult struct {
	ID        string
	Hash      string
	Hash16    string
	Prev      string
	BlockPath string
	// Guard é o veredito da 4ª Muralha (memoryguard). Preenchido sempre que a
	// validação roda (RegisterLearning e PreviewLearning). Approved=false = o
	// aprendizado violou a régua; por padrão o write é bloqueado (fail-closed).
	Guard *memoryguard.Verdict `json:"guard,omitempty"`
}

// registerOptions configura o comportamento de RegisterLearning.
type registerOptions struct {
	force bool
}

// RegisterOption altera o comportamento de RegisterLearning.
type RegisterOption func(*registerOptions)

// WithForce grava mesmo se a 4ª Muralha reprovar — privilégio do Don (o portão
// de 3 fatores já validou a presença no terminal). O Verdict continua no
// resultado e a decisão é registrada em log; o bloqueio vira aviso.
func WithForce() RegisterOption {
	return func(o *registerOptions) { o.force = true }
}

// RegisterLearning performs the full 8-step memory registration flow
// (MEMORY_ACCESS_PROTOCOL.md §3): build block → 4ª Muralha (memoryguard) →
// sha256(conteúdo completo) → rename → trigger line → chain.dat row → merkle
// regen. It does NOT commit or sign — those remain explicit git/integrity
// steps (ORDEM SAGRADA L199).
//
// A 4ª Muralha roda ANTES de qualquer write: se o Verdict reprovar, o block
// NÃO é gravado (fail-closed) a menos que WithForce() seja passado (Don).
func RegisterLearning(agentDir string, in LearningInput, opts ...RegisterOption) (*LearningResult, error) {
	var ro registerOptions
	for _, opt := range opts {
		opt(&ro)
	}

	res, content, err := prepareLearning(agentDir, in)
	if err != nil {
		return nil, err
	}

	// 4ª Muralha — fail-closed por padrão. Sem override explícito do Don,
	// aprendizado reprovado NÃO toca o disco.
	if res.Guard != nil && !res.Guard.Approved && !ro.force {
		return nil, fmt.Errorf("memoryguard: aprendizado bloqueado pela 4ª Muralha: %s",
			strings.Join(res.Guard.Reasons, "; "))
	}
	if res.Guard != nil && !res.Guard.Approved {
		log.Printf("memoryguard: FORÇADO pelo Don (override) — categorias: %s",
			strings.Join(res.Guard.Reasons, "; "))
	}

	blockPath := filepath.Join(agentDir, "blocks", res.Hash+".md")
	if err := os.MkdirAll(filepath.Dir(blockPath), 0o755); err != nil {
		return nil, fmt.Errorf("create blocks dir: %w", err)
	}
	if err := os.WriteFile(blockPath, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("write block: %w", err)
	}

	date := time.Now().UTC().Format("2006-01-02")
	tags := strings.Join(in.Tags, " ")
	if err := appendTrigger(filepath.Join(agentDir, "learnings.md"), res.ID, date, in.Title, in.Level, tags, res.Hash16); err != nil {
		return nil, err
	}
	if err := appendChainRow(filepath.Join(agentDir, "chain.dat"), res.Hash, res.Prev, date, res.ID, in.Title); err != nil {
		return nil, err
	}
	if err := RegenerateMerkle(agentDir); err != nil {
		return nil, err
	}

	res.BlockPath = blockPath
	return res, nil
}

// PreviewLearning computes what RegisterLearning would do — next ID, PREV,
// block hash and full block content — WITHOUT writing anything. Used by the
// CLI --dry-run flag.
func PreviewLearning(agentDir string, in LearningInput) (*LearningResult, string, error) {
	return prepareLearning(agentDir, in)
}

// prepareLearning validates input, derives ID/PREV, builds the block and
// computes its content hash. Shared by RegisterLearning and PreviewLearning.
func prepareLearning(agentDir string, in LearningInput) (*LearningResult, string, error) {
	if in.Agent == "" {
		in.Agent = "cosca-kernel"
	}
	if in.Title == "" {
		return nil, "", fmt.Errorf("title is required")
	}
	if in.Level < 1 || in.Level > 5 {
		return nil, "", fmt.Errorf("level must be 1-5, got %d", in.Level)
	}
	if len(in.Tags) == 0 {
		return nil, "", fmt.Errorf("at least one tag is required")
	}

	nextID, err := nextLearningID(filepath.Join(agentDir, "learnings.md"))
	if err != nil {
		return nil, "", err
	}
	prev, err := lastChainHash(filepath.Join(agentDir, "chain.dat"))
	if err != nil {
		return nil, "", err
	}

	date := time.Now().UTC().Format("2006-01-02")
	tags := strings.Join(in.Tags, " ")
	content := buildBlock(prev, nextID, date, in, tags)

	// 4ª Muralha: o oráculo valida o conteúdo completo do block ANTES do
	// write. O Verdict vai no resultado — PreviewLearning expõe sem bloquear,
	// RegisterLearning bloqueia quando reprovado (fail-closed).
	guard := validateGuard(in.Level, content)

	sum := sha256.Sum256([]byte(content))
	hash := fmt.Sprintf("%x", sum)

	return &LearningResult{
		ID:     nextID,
		Hash:   hash,
		Hash16: hash[:16],
		Prev:   prev,
		Guard:  &guard,
	}, content, nil
}

// validateGuard roda os dois oráculos da 4ª Muralha sobre o conteúdo completo
// do block: ValidateLearning (vaidade, auto-engrandecimento, confiança
// inflada, ofuscação, falsa identidade, injeção) + ValidateFullContent
// (FALSE SUCCESS — alegação de sucesso sem evidência observável). Qualquer
// reprovação derruba o Approved.
func validateGuard(level int, content string) memoryguard.Verdict {
	v := memoryguard.ValidateLearning(level, content)
	full := memoryguard.ValidateFullContent(content)
	if !full.Approved {
		v.Approved = false
		v.Reasons = append(v.Reasons, full.Reasons...)
	}
	return v
}

// buildBlock renders the full block file (header + --- + título + tabela).
func buildBlock(prev, id, date string, in LearningInput, tags string) string {
	confidence := in.Confidence
	if confidence == 0 {
		confidence = 0.85
	}
	var b strings.Builder
	fmt.Fprintf(&b, "PREV: %s\n", prev)
	fmt.Fprintf(&b, "ID: %s\n", id)
	fmt.Fprintf(&b, "TIME: %s\n", date)
	fmt.Fprintf(&b, "LEVEL: %d\n", in.Level)
	fmt.Fprintf(&b, "TAGS: %s\n", tags)
	b.WriteString("---\n")
	fmt.Fprintf(&b, "## %s — %s — %s | Level %d\n", id, date, in.Title, in.Level)
	b.WriteString("\n")
	b.WriteString("| Field | Value |\n")
	b.WriteString("|-------|-------|\n")
	fmt.Fprintf(&b, "| **Agent** | %s |\n", in.Agent)
	fmt.Fprintf(&b, "| **Task** | %s |\n", in.Task)
	fmt.Fprintf(&b, "| **Technique** | %s |\n", in.Technique)
	fmt.Fprintf(&b, "| **Level** | %d |\n", in.Level)
	fmt.Fprintf(&b, "| **Outcome** | %s |\n", in.Outcome)
	fmt.Fprintf(&b, "| **Confidence** | %.2f |\n", confidence)
	fmt.Fprintf(&b, "| **Tags** | %s |\n", tags)
	fmt.Fprintf(&b, "| **Related** | %s |\n", in.Related)
	fmt.Fprintf(&b, "| **Learned** | %s |\n", in.Learned)
	fmt.Fprintf(&b, "| **Next** | %s |\n", in.Next)
	return b.String()
}

var learningIDRe = regexp.MustCompile(`^## L(\d+) \|`)

// nextLearningID returns the next L-number by scanning learnings.md.
func nextLearningID(learningsPath string) (string, error) {
	data, err := os.ReadFile(learningsPath)
	if err != nil {
		return "", fmt.Errorf("read learnings.md: %w", err)
	}
	max := 0
	for _, line := range strings.Split(string(data), "\n") {
		m := learningIDRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	if max == 0 {
		return "", fmt.Errorf("no existing learnings found in %s", learningsPath)
	}
	return fmt.Sprintf("L%d", max+1), nil
}

// lastChainHash returns the hash of the last block in chain.dat.
func lastChainHash(chainPath string) (string, error) {
	data, err := os.ReadFile(chainPath)
	if err != nil {
		return "", fmt.Errorf("read chain.dat: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 1 || parts[0] == "" {
			continue
		}
		return strings.TrimSpace(parts[0]), nil
	}
	return genesisHash, nil
}

// appendTrigger appends the 1-line gatilho to learnings.md.
func appendTrigger(learningsPath, id, date, title string, level int, tags, hash16 string) error {
	line := fmt.Sprintf("## %s | %s | %s | L%d | %s | %s\n", id, date, title, level, tags, hash16)
	return appendLine(learningsPath, line)
}

// appendChainRow appends the 1-line ledger row to chain.dat.
func appendChainRow(chainPath, hash, prev, date, id, title string) error {
	line := fmt.Sprintf("%s|%s|%s|%s|%s\n", hash, prev, date, id, title)
	return appendLine(chainPath, line)
}

func appendLine(path, line string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	if _, err := f.WriteString(line); err != nil {
		return fmt.Errorf("append to %s: %w", path, err)
	}
	return nil
}

// ── Merkle ──────────────────────────────────────────────────────────────────

// merkleBlockHash is a parsed row of chain.dat.
type merkleBlockHash struct {
	hash string
	prev string
}

// RegenerateMerkle rebuilds merkle/epoch_*.json + merkle/index.json from
// chain.dat. It verifies chain links (each prev == previous hash, first ==
// genesis) so metadata is never rebuilt from a tampered ledger.
func RegenerateMerkle(dir string) error {
	chainPath := filepath.Join(dir, "chain.dat")
	rows, err := readChainRows(chainPath)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return fmt.Errorf("no blocks found in chain.dat")
	}

	type epochMeta struct {
		Epoch      int    `json:"epoch"`
		StartBlock int    `json:"start_block"`
		EndBlock   int    `json:"end_block"`
		BlockCount int    `json:"block_count"`
		MerkleRoot string `json:"merkle_root"`
		FirstBlock string `json:"first_block"`
		LastBlock  string `json:"last_block"`
	}
	type indexEpoch struct {
		Epoch int    `json:"epoch"`
		Root  string `json:"root"`
	}
	type indexMeta struct {
		TotalBlocks int          `json:"total_blocks"`
		EpochSize   int          `json:"epoch_size"`
		TotalEpochs int          `json:"total_epochs"`
		Epochs      []indexEpoch `json:"epochs"`
	}

	var epochs []epochMeta
	for start := 0; start < len(rows); start += merkleEpochSize {
		end := start + merkleEpochSize - 1
		if end >= len(rows) {
			end = len(rows) - 1
		}
		leaves := make([]string, 0, end-start+1)
		for i := start; i <= end; i++ {
			leaves = append(leaves, rows[i].hash)
		}
		epochs = append(epochs, epochMeta{
			Epoch:      len(epochs),
			StartBlock: start,
			EndBlock:   end,
			BlockCount: len(leaves),
			MerkleRoot: merkleRoot(leaves),
			FirstBlock: rows[start].hash,
			LastBlock:  rows[end].hash,
		})
	}

	merkleDir := filepath.Join(dir, "merkle")
	if err := os.MkdirAll(merkleDir, 0o700); err != nil {
		return fmt.Errorf("mkdir merkle: %w", err)
	}
	for _, ep := range epochs {
		path := filepath.Join(merkleDir, fmt.Sprintf("epoch_%04d.json", ep.Epoch))
		if err := writeJSONFile(path, ep); err != nil {
			return err
		}
	}
	idx := indexMeta{
		TotalBlocks: len(rows),
		EpochSize:   merkleEpochSize,
		TotalEpochs: len(epochs),
	}
	for _, ep := range epochs {
		idx.Epochs = append(idx.Epochs, indexEpoch{Epoch: ep.Epoch, Root: ep.MerkleRoot})
	}
	return writeJSONFile(filepath.Join(merkleDir, "index.json"), idx)
}

func readChainRows(path string) ([]merkleBlockHash, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rows []merkleBlockHash
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 2 {
			return nil, fmt.Errorf("malformed row: %q", line)
		}
		rows = append(rows, merkleBlockHash{hash: strings.TrimSpace(parts[0]), prev: strings.TrimSpace(parts[1])})
	}
	for i, r := range rows {
		if i == 0 {
			if r.prev != genesisHash {
				return nil, fmt.Errorf("row 0 prev %q != genesis", r.prev)
			}
			continue
		}
		if r.prev != rows[i-1].hash {
			return nil, fmt.Errorf("chain break at row %d: prev %q != previous hash %q", i, r.prev, rows[i-1].hash)
		}
	}
	return rows, nil
}

func merkleRoot(leaves []string) string {
	level := make([][]byte, len(leaves))
	for i, l := range leaves {
		level[i] = []byte(l)
	}
	for len(level) > 1 {
		next := make([][]byte, 0, (len(level)+1)/2)
		for i := 0; i < len(level); i += 2 {
			if i+1 < len(level) {
				h := sha256.Sum256(append(append([]byte{}, level[i]...), level[i+1]...))
				next = append(next, h[:])
			} else {
				next = append(next, level[i])
			}
		}
		level = next
	}
	return fmt.Sprintf("%x", level[0])
}

func writeJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}
