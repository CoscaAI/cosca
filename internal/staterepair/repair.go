// repair.go — o Manager e o fluxo de Repair assistido (ADR-043 §8).
//
// Fluxo por arquivo doente (apply): ledger guard (recusa após MaxAttempts
// falhas no MESMO fingerprint) → ForensicBackup → quarentena
// (.cosca/quarantine/staterepair/) → recipe de rebuild (re-execução do
// próprio binário) → Health pós → sucesso: RecordSuccess + limpa quarentena /
// falha: RecordFailure + RESTAURA o original da quarentena + instruções
// manuais explícitas (nunca meio-repair — o original corrompido volta ao
// lugar e nada de "reparado" é reportado sem Health pós limpo).
//
// dry-run NUNCA escreve: nenhum diretório criado, nenhum ledger tocado,
// nenhum arquivo movido — apenas reporta o plano.
package staterepair

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// sqliteSidecarSuffixes são os arquivos companheiros do SQLite movidos junto
// com o banco doente para a quarentena (e devolvidos na restauração). Sem
// eles, um -wal órfão com o nome original contaminaria o banco recém-criado
// pela recipe.
var sqliteSidecarSuffixes = []string{"-wal", "-shm", "-journal"}

// Status classifica o resultado do repair de um arquivo.
type Status string

// Estados possíveis de um PathReport.
const (
	// StatusOK: banco saudável — nada a reparar.
	StatusOK Status = "ok"
	// StatusSick: banco doente detectado (relato de check sem cirurgia).
	StatusSick Status = "sick"
	// StatusMissing: arquivo não existe — nada a reparar (não é corrupção).
	StatusMissing Status = "missing"
	// StatusDryRun: banco doente — plano reportado, nada escrito.
	StatusDryRun Status = "dry_run"
	// StatusRepaired: recipe rodou e o Health pós está limpo.
	StatusRepaired Status = "repaired"
	// StatusAttemptsExhausted: ledger recusou nova cirurgia (3 falhas no
	// mesmo fingerprint) — recuperação manual exigida.
	StatusAttemptsExhausted Status = "attempts_exhausted"
	// StatusRecipeFailed: recipe não regenerou um banco saudável — original
	// devolvido da quarentena + instruções manuais.
	StatusRecipeFailed Status = "recipe_failed"
	// StatusError: falha de infraestrutura (sem cirurgia).
	StatusError Status = "error"
)

// PathReport é o resultado do repair de UM arquivo de banco.
type PathReport struct {
	Kind        DBKind `json:"kind"`
	Path        string `json:"path"`
	Status      Status `json:"status"`
	Detail      string `json:"detail,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	BackupPath  string `json:"backup_path,omitempty"`
	// QuarantinePath é o caminho do original doente na quarentena (quando o
	// repair falhou e o original foi restaurado, o caminho aponta para onde
	// ele ESTEVE — rastreabilidade).
	QuarantinePath     string   `json:"quarantine_path,omitempty"`
	RecipeLabel        string   `json:"recipe_label,omitempty"`
	Recipe             []string `json:"recipe,omitempty"`
	RecipeOutput       string   `json:"recipe_output,omitempty"`
	ManualInstructions string   `json:"manual_instructions,omitempty"`
	Error              string   `json:"error,omitempty"`
}

// Options configura o Repair.
type Options struct {
	// Apply executa a cirurgia (ledger guard, backup, quarentena, recipe).
	Apply bool
	// DryRun apenas reporta o plano — NUNCA escreve nada.
	DryRun bool
}

// recipeRunner assinatura do executor de recipe (injetável nos testes).
type recipeRunner func(ctx context.Context, projectRoot string, r Recipe) (string, error)

// Manager é o ponto de entrada do repair assistido de estado.
//
// Manager pattern (mesma forma de internal/agents, internal/memory): o Manager
// possui a lógica de negócio + acesso a dados (o filesystem do .cosca). A CLI
// injeta o Manager; nada de serve/boot/engine passa por aqui.
type Manager struct {
	// root é o diretório `.cosca` alvo.
	root string
	// runRecipe executa a recipe de rebuild. Default: re-execução do próprio
	// binário (runRecipeBinary). Injetável nos testes.
	runRecipe recipeRunner
}

// NewManager cria o Manager do repair sobre o diretório `.cosca` informado.
func NewManager(root string) *Manager {
	if root == "" {
		root = ".cosca"
	}
	abs, err := filepath.Abs(root)
	if err == nil {
		root = abs
	}
	return &Manager{
		root:      filepath.Clean(root),
		runRecipe: runRecipeBinary,
	}
}

// Root devolve o diretório `.cosca` alvo.
func (m *Manager) Root() string { return m.root }

// DBPath devolve o caminho do banco de uma classe única (não expande glob).
func (m *Manager) DBPath(kind DBKind) (string, error) {
	switch kind {
	case KindSession:
		return filepath.Join(m.root, "session.db"), nil
	case KindIndex:
		return filepath.Join(m.root, "memory", "index.db"), nil
	case KindKnowledge:
		return filepath.Join(m.root, "knowledge.db"), nil
	case KindVector:
		return "", fmt.Errorf("vector é um glob (vector-*.db) — use DBPaths")
	default:
		return "", fmt.Errorf("classe de banco inválida %q", kind)
	}
}

// DBPaths devolve os caminhos de banco da classe (o glob vector-*.db é
// expandido e ordenado). Um glob sem matches devolve lista vazia.
func (m *Manager) DBPaths(kind DBKind) ([]string, error) {
	if kind == KindVector {
		matches, err := filepath.Glob(filepath.Join(m.root, "vector-*.db"))
		if err != nil {
			return nil, fmt.Errorf("expandir vector-*.db: %w", err)
		}
		sort.Strings(matches)
		return matches, nil
	}
	p, err := m.DBPath(kind)
	if err != nil {
		return nil, err
	}
	return []string{p}, nil
}

// Repair roda o fluxo de repair para a classe (ou KindAll). Devolve um
// PathReport por arquivo avaliado. Erros de infraestrutura (classe inválida,
// modo ambíguo) são devolvidos; falhas por arquivo ficam no PathReport.
func (m *Manager) Repair(kind DBKind, o Options) ([]PathReport, error) {
	if m == nil {
		return nil, fmt.Errorf("staterepair: Manager nil")
	}
	if o.Apply && o.DryRun {
		return nil, fmt.Errorf("staterepair: --apply e --dry-run são mutuamente exclusivos")
	}
	kinds, err := expandKind(kind)
	if err != nil {
		return nil, err
	}
	var reports []PathReport
	for _, k := range kinds {
		paths, err := m.DBPaths(k)
		if err != nil {
			return nil, err
		}
		for _, p := range paths {
			reports = append(reports, m.repairPath(k, p, o))
		}
	}
	return reports, nil
}

// expandKind normaliza o seletor da CLI para a lista de classes concretas.
func expandKind(kind DBKind) ([]DBKind, error) {
	switch kind {
	case KindAll:
		return SupportedKinds(), nil
	case KindSession, KindIndex, KindKnowledge, KindVector:
		return []DBKind{kind}, nil
	default:
		return nil, fmt.Errorf("classe de banco inválida %q (use session|index|knowledge|vector|all)", kind)
	}
}

// repairPath roda o fluxo de repair para um único arquivo.
func (m *Manager) repairPath(kind DBKind, dbPath string, o Options) PathReport {
	r := PathReport{Kind: kind, Path: dbPath}

	// 1. Arquivo ausente não é corrupção — nada a reparar.
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		r.Status = StatusMissing
		r.Detail = "arquivo não existe — nada a reparar (não é corrupção)"
		return r
	}

	// 2. Health.
	ok, detail, hErr := Health(dbPath)
	if hErr != nil {
		r.Status = StatusError
		r.Error = fmt.Sprintf("health: %v", hErr)
		return r
	}
	if ok {
		r.Status = StatusOK
		r.Detail = "quick_check ok — banco saudável"
		return r
	}
	r.Status = StatusSick
	r.Detail = detail

	// Fingerprint (best-effort; identidade para o ledger).
	fp, fpErr := Fingerprint(dbPath)
	if fpErr == nil {
		r.Fingerprint = fp
	}

	recipe, recipeErr := recipeFor(kind)
	if recipeErr != nil {
		r.Status = StatusError
		r.Error = recipeErr.Error()
		return r
	}
	r.RecipeLabel = recipe.Label
	for _, s := range recipe.Steps {
		r.Recipe = append(r.Recipe, s.commandString())
	}

	// Plano (quarentena + backup) calculado de forma determinística, sem criar
	// nada — usado no dry-run e no relatório de falha.
	plannedQuarantine := quarantinePlannedPath(dbPath)

	// 3. Dry-run: apenas reporta o plano. NUNCA escreve.
	if o.DryRun {
		r.Status = StatusDryRun
		r.QuarantinePath = plannedQuarantine
		r.Detail = "plano (dry-run): backup forense + quarentena + " + recipe.Label + " + health pós"
		r.ManualInstructions = recipe.ManualInstructions
		return r
	}

	// 4. Modo check (sem --apply): relata o doente, não toca em nada.
	if !o.Apply {
		r.Detail = detail + " — rode com --dry-run para o plano ou --apply para reparar"
		return r
	}

	// 5. Apply — ledger guard: recusa nova cirurgia no MESMO fingerprint
	//    exausto (3 falhas). O original NÃO é tocado.
	if fp != "" && AttemptsExhausted(dbPath, fp) {
		r.Status = StatusAttemptsExhausted
		r.ManualInstructions = exhaustedInstructions(dbPath, r.Detail)
		return r
	}

	// 6. Backup forense (dedupe por conteúdo + retenção 3). Falha = HARD STOP:
	//    a cópia forense é o caminho de recuperação quando toda recipe falha.
	backupPath, bErr := ForensicBackup(dbPath)
	if bErr != nil {
		r.Status = StatusError
		r.Error = fmt.Sprintf("backup forense recusado (hard stop): %v", bErr)
		return r
	}
	r.BackupPath = backupPath

	// 7. Quarentena: move o original doente (+ sidecars SQLite) para
	//    .cosca/quarantine/staterepair/.
	moved, qErr := quarantineMove(dbPath)
	if qErr != nil {
		r.Status = StatusError
		r.Error = fmt.Sprintf("quarentena: %v", qErr)
		return r
	}
	r.QuarantinePath = plannedQuarantine

	// 8. Recipe de rebuild (re-execução do próprio binário).
	ctx := context.Background()
	out, runErr := m.runRecipe(ctx, filepath.Dir(m.root), recipe)
	if runErr != nil {
		r.Status = StatusRecipeFailed
		r.Error = fmt.Sprintf("recipe falhou: %v", runErr)
		r.RecipeOutput = tailOutput(out)
		_ = quarantineRestore(dbPath, moved)
		if fp != "" {
			_ = RecordFailure(dbPath, fp)
		}
		r.ManualInstructions = recipeManual(dbPath, recipe, r.Detail)
		return r
	}
	r.RecipeOutput = tailOutput(out)

	// 9. Health pós — o veredito é do banco regenerado, não da recipe.
	ok, detail, hErr = Health(dbPath)
	if hErr == nil && ok {
		r.Status = StatusRepaired
		r.Detail = "recipe executada e health pós ok (" + recipe.Label + ")"
		if fp != "" {
			_ = RecordSuccess(dbPath, fp)
		}
		_ = quarantineCleanup(moved)
		return r
	}
	if hErr != nil {
		r.Detail = detail + " (health pós: " + hErr.Error() + ")"
	} else {
		r.Detail = "health pós ainda doente: " + detail
	}
	r.Status = StatusRecipeFailed
	_ = quarantineRestore(dbPath, moved)
	if fp != "" {
		_ = RecordFailure(dbPath, fp)
	}
	r.ManualInstructions = recipeManual(dbPath, recipe, r.Detail)
	return r
}

// quarantinePlannedPath devolve o destino determinístico da quarentena
// (sem criar nada).
func quarantinePlannedPath(dbPath string) string {
	return filepath.Join(quarantineDir(dbPath), "staterepair-"+stemOf(dbPath)+"-<ts>.db")
}

// quarantineDir devolve `.cosca/quarantine/staterepair/`.
func quarantineDir(dbPath string) string {
	return filepath.Join(coscaRootOf(dbPath), "quarantine", "staterepair")
}

// uniqueDest resolve um destino sem colisão no diretório.
func uniqueDest(dir, base, suffix string) string {
	dst := filepath.Join(dir, base+suffix)
	for i := 2; ; i++ {
		if _, err := os.Stat(dst); os.IsNotExist(err) {
			return dst
		}
		dst = filepath.Join(dir, fmt.Sprintf("%s_%d%s", base, i, suffix))
	}
}

// quarantineMove move o banco doente (+ sidecars) para a quarentena e devolve
// o mapa origem → destino (usado pela restauração/limpeza).
func quarantineMove(dbPath string) (map[string]string, error) {
	dir := quarantineDir(dbPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("criar quarentena %s: %w", dir, err)
	}
	base := "staterepair-" + stemOf(dbPath) + "-" + time.Now().UTC().Format(backupTimeLayout)
	moved := map[string]string{}
	for _, suffix := range append([]string{""}, sqliteSidecarSuffixes...) {
		src := dbPath + suffix
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}
		dst := uniqueDest(dir, base, suffix)
		if err := os.Rename(src, dst); err != nil {
			return moved, fmt.Errorf("mover %s para quarentena: %w", src, err)
		}
		moved[src] = dst
	}
	return moved, nil
}

// quarantineRestore devolve o original doente da quarentena ao lugar. Remove
// primeiro o que a recipe tiver deixado no caminho original (nunca misturar um
// banco recém-criado com o original corrompido preservado).
func quarantineRestore(dbPath string, moved map[string]string) error {
	for _, suffix := range append([]string{""}, sqliteSidecarSuffixes...) {
		_ = os.Remove(dbPath + suffix)
	}
	for src, dst := range moved {
		if err := os.Rename(dst, src); err != nil {
			return fmt.Errorf("restaurar %s: %w", src, err)
		}
	}
	return nil
}

// quarantineCleanup remove as cópias da quarentena após um repair bem-sucedido
// (o backup forense já preserva o original doente).
func quarantineCleanup(moved map[string]string) error {
	for _, dst := range moved {
		if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// runRecipeBinary executa os passos da recipe re-executando o PRÓPRIO binário
// (os.Executable + args), no diretório do projeto (pai do .cosca). Mesmo
// padrão de internal/cli/despertar.go checkChain e start.go findCoscaBin.
func runRecipeBinary(ctx context.Context, projectRoot string, r Recipe) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolver o próprio executável: %w", err)
	}
	var chunks []string
	for _, step := range r.Steps {
		cmd := exec.CommandContext(ctx, exe, step.Args...)
		cmd.Dir = projectRoot
		cmd.Env = os.Environ()
		b, err := cmd.CombinedOutput()
		chunks = append(chunks, fmt.Sprintf("$ cosca %s\n%s", strings.Join(step.Args, " "), strings.TrimSpace(string(b))))
		if err != nil {
			return strings.Join(chunks, "\n"), fmt.Errorf("passo %q falhou: %w", step.commandString(), err)
		}
	}
	return strings.Join(chunks, "\n"), nil
}

// tailOutput limita o output da recipe no relatório (diagnóstico sem ruído).
func tailOutput(out string) string {
	const max = 4000
	if len(out) <= max {
		return out
	}
	return "…" + out[len(out)-max:]
}

// exhaustedInstructions é a orientação estável quando o ledger recusa cirurgia.
func exhaustedInstructions(dbPath, detail string) string {
	return fmt.Sprintf(
		"o ledger registrou %d falhas no MESMO fingerprint deste arquivo — a corrupção provavelmente está além do rebuild automático (dano de página b-tree). Recuperação manual exigida: restaure um backup forense de .cosca/backups/staterepair/ (o mais recente), ou rode as recipes manuais listadas abaixo. Detalhe do health: %s. Apague %s para forçar uma nova tentativa automática.",
		MaxAttempts, detail, LedgerPath(dbPath),
	)
}

// recipeManual monta as instruções manuais de uma falha de recipe.
func recipeManual(dbPath string, recipe Recipe, healthDetail string) string {
	var b strings.Builder
	b.WriteString("a recipe automática não regenerou um banco saudável — o original doente foi RESTAURADO da quarentena (nada foi perdido, nada foi meio-reparado).\n")
	b.WriteString("health: " + healthDetail + "\n")
	b.WriteString("backup forense do arquivo doente: .cosca/backups/staterepair/\n")
	if recipe.ManualInstructions != "" {
		b.WriteString("instruções manuais: " + recipe.ManualInstructions + "\n")
	} else {
		b.WriteString("instruções manuais: rode manualmente, em ordem:\n")
		for _, s := range recipe.Steps {
			b.WriteString("  cosca " + strings.Join(s.Args, " ") + "\n")
		}
	}
	return strings.TrimSpace(b.String())
}
