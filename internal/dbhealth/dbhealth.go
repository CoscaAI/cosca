// Package dbhealth implementa o gate de tamanho por banco definido na
// Decisão 1 do ADR-013 (limite de 100 MB por banco) — §2.2.1.
//
// O gate é 100% READ-ONLY: ele apenas abre cada banco SQLite em modo somente
// leitura (`mode=ro`), calcula o tamanho on-disk real via `PRAGMA page_count *
// PRAGMA page_size` (a métrica determinística prescrita pelo ADR) e classifica
// o status de cada banco contra os limites (ok | warn | fail). NUNCA escreve,
// NUNCA migra, NUNCA apaga.
//
// Escopo: mede os módulos alvo (knowledge.db, memory/index.db, core.db,
// events.db e os derivados projetos projects/graph/vector/fts — mesmo quando
// ainda não existem, reportando-os como "não encontrado" sem quebrar) e,
// por padrão, também varre todos os `*.db` encontrados dentro do `.cosca`
// (runtime dbs como audit.db, session.db, gate.db etc.). Assim o relatório é
// completo E extensível: um novo módulo físico que aparecer no `.cosca` é
// capturado automaticamente.
package dbhealth

import (
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite" // pure-Go SQLite driver — driver do gate
)

// ─── Constantes de referência (ADR-013 §2.2.1, Decisão 1) ───────────────────

const (
	// DefaultLimitBytes é o teto de 100 MB por banco (fonte da verdade + cada
	// módulo). 100 MB = 100 * 1024 * 1024 bytes, a unidade binária usada pelo
	// ADR (104857600 bytes).
	DefaultLimitBytes int64 = 100 * 1024 * 1024

	// ReferenceCeiling é a referência fixa de 104857600 bytes usada em
	// `percent_of_100mb` — o "100 MB" canônico do ADR, independente do limite
	// configurado via --limit-mb.
	ReferenceCeiling int64 = 100 * 1024 * 1024

	// DefaultWarnRatio é o limiar de alerta: 80% do teto (~80 MB).
	DefaultWarnRatio = 0.80

	// DefaultFailRatio é o limiar de bloqueio: 100% do teto.
	DefaultFailRatio = 1.00
)

// mb é a constante de conversão MB (binário) → bytes. 1 MB = 1024*1024.
const mb = 1024 * 1024

// Status é o estado de um banco medido pelo gate.
type Status string

// Estados possíveis de um banco no relatório.
const (
	// StatusOK: o banco está dentro dos limites (abaixo do alerta).
	StatusOK Status = "ok"
	// StatusWarn: o banco cruzou o limiar de alerta (ex.: 80% do teto).
	StatusWarn Status = "warn"
	// StatusFail: o banco cruzou o teto (excedeu 100%) — gate bloqueia.
	StatusFail Status = "fail"
	// StatusNotFound: o módulo alvo não existe no disco. Não contribui para
	// warn/fail — apenas informa.
	StatusNotFound Status = "not_found"
)

// NormalizeStatus normaliza strings de status; é usada na CLI para exibir.
func NormalizeStatus(s Status) string {
	if s == "" {
		return "desconhecido"
	}
	return string(s)
}

// Target é um módulo de banco monitorado pelo gate. Path é relativo ao
// diretório `.cosca`.
type Target struct {
	// Name é o rótulo/identificador do módulo (ex.: "knowledge").
	Name string
	// Path é o caminho relativo ao `.cosca` (ex.: "knowledge.db" ou
	// "memory/index.db").
	Path string
}

// DefaultTargets é a lista de módulos monitorados pelo gate (ADR-013 §2.1/§3).
// Inclui o `knowledge.db` atual (a fonte da verdade de hoje) e os módulos
// projetados pelo ADR (core, events e os derivados projects/graph/vector/fts).
// Módulos que ainda não existem no disco são reportados como "não encontrado",
// sem quebrar o gate — o gate é extensível a eles por construção.
var DefaultTargets = []Target{
	{Name: "knowledge", Path: "knowledge.db"},
	{Name: "memory", Path: "memory/index.db"},
	{Name: "core", Path: "core.db"},
	{Name: "events", Path: "events.db"},
	{Name: "projects", Path: "projects.db"},
	{Name: "graph", Path: "graph.db"},
	{Name: "vector", Path: "vector.db"},
	{Name: "fts", Path: "fts.db"},
}

// Limits configura o teto e os limiares do gate.
type Limits struct {
	// LimitBytes é o teto por banco (default: 100 MB).
	LimitBytes int64
	// WarnBytes é o limiar de alerta (default: 80% do teto).
	WarnBytes int64
	// FailBytes é o limiar de bloqueio (default: 100% do teto).
	FailBytes int64
}

// DefaultLimits devolve os limites padrão do gate: 100 MB / 80% / 100%.
func DefaultLimits() Limits {
	return Limits{
		LimitBytes: DefaultLimitBytes,
		WarnBytes:  int64(float64(DefaultLimitBytes) * DefaultWarnRatio),
		FailBytes:  int64(float64(DefaultLimitBytes) * DefaultFailRatio),
	}
}

// FromConfigMB constrói Limits a partir de um teto em MB e um alerta opcional
// em MB (mebibytes, 1 MB = 1024*1024). warnMB <= 0 significa "automático =
// 80% do teto". fail é sempre igual ao teto (exceder 100% = bloqueio), conforme
// a Decisão 1.
func FromConfigMB(limitMB, warnMB float64) Limits {
	l := Limits{LimitBytes: int64(limitMB * mb)}
	if l.LimitBytes <= 0 {
		l.LimitBytes = DefaultLimitBytes
	}
	l.FailBytes = l.LimitBytes
	if warnMB > 0 {
		l.WarnBytes = int64(warnMB * mb)
	} else {
		l.WarnBytes = int64(float64(l.LimitBytes) * DefaultWarnRatio)
	}
	if l.WarnBytes <= 0 {
		l.WarnBytes = int64(float64(l.LimitBytes) * DefaultWarnRatio)
	}
	return l
}

// Report descreve um banco medido pelo gate.
type Report struct {
	// Name é o rótulo do módulo (ex.: "knowledge").
	Name string `json:"name"`
	// Path é o caminho absoluto do arquivo .db.
	Path string `json:"path"`
	// RelPath é o caminho relativo ao diretório `.cosca` (ex.: "knowledge.db").
	RelPath string `json:"rel_path"`
	// Found indica se o arquivo existe no disco.
	Found bool `json:"found"`
	// PageCount é o valor de PRAGMA page_count.
	PageCount int64 `json:"page_count"`
	// PageSize é o valor de PRAGMA page_size.
	PageSize int64 `json:"page_size"`
	// DBSizeBytes é o tamanho on-disk real = page_count * page_size.
	DBSizeBytes int64 `json:"db_size_bytes"`
	// PercentOf100MB é a fração do tamanho sobre o teto canônico de 100 MB
	// (db_size / 104857600), conforme prescrito no ADR (para 257 MB seria
	// ≈ 2.57). Não é um percentual ×100 — é a razão exata.
	PercentOf100MB float64 `json:"percent_of_100mb"`
	// PercentOfLimit é a fração do tamanho sobre o teto configurado
	// (db_size / limit_bytes). Com --limit-mb 100 é idêntico a
	// PercentOf100MB; com outro limite, reflete o % do teto configurado.
	PercentOfLimit float64 `json:"percent_of_limit"`
	// Status é o estado do banco (ok | warn | fail | not_found).
	Status Status `json:"status"`
	// Error traz um problema de leitura/medição, quando houver.
	Error string `json:"error,omitempty"`
}

// Result é o relatório agregado do gate: um report por banco + o veredito.
type Result struct {
	// CoscaDir é o diretório `.cosca` inspecionado.
	CoscaDir string `json:"cosca_dir"`
	// LimitBytes é o teto aplicado (100 MB por default).
	LimitBytes int64 `json:"limit_bytes"`
	// WarnBytes é o limiar de alerta aplicado.
	WarnBytes int64 `json:"warn_bytes"`
	// FailBytes é o limiar de bloqueio aplicado.
	FailBytes int64 `json:"fail_bytes"`
	// Databases é a lista de relatórios por banco.
	Databases []Report `json:"databases"`
	// AnyWarn é true quando algum banco existente cruzou o alerta.
	AnyWarn bool `json:"any_warn"`
	// AnyFail é true quando algum banco existente cruzou o teto.
	AnyFail bool `json:"any_fail"`
	// Passed é true quando nenhum banco cruzou o teto (gate aprovado).
	Passed bool `json:"passed"`
	// Gate indica se a medição foi rodada em modo gate (--gate). Preenchido
	// pela CLI para o output JSON.
	Gate bool `json:"gate"`
}

// Options configura a medição do gate.
type Options struct {
	// CoscaDir é o diretório `.cosca` a inspecionar (obrigatório).
	CoscaDir string
	// Targets é a lista de módulos alvo; nil → DefaultTargets.
	Targets []Target
	// Limits são os limites do gate; zero → DefaultLimits.
	Limits Limits
	// IncludeAll indica se, além dos módulos alvo, o gate deve varrer todos
	// os `*.db` encontrados recursivamente no `.cosca`. Default: true.
	IncludeAll bool
}

// Check inspeciona o `.cosca`, mede cada banco (módulos alvo +, opcionalmente,
// todos os `*.db` no disco) e devolve o relatório agregado com o veredito do
// gate. É READ-ONLY: nunca escreve, nunca cria, nunca migra.
func Check(opts Options) (*Result, error) {
	if opts.CoscaDir == "" {
		return nil, fmt.Errorf("dbhealth: cosca dir é obrigatório")
	}
	if opts.Targets == nil {
		opts.Targets = DefaultTargets
	}
	l := opts.Limits
	if l.LimitBytes <= 0 {
		l = DefaultLimits()
	}
	if l.WarnBytes <= 0 {
		l.WarnBytes = int64(float64(l.LimitBytes) * DefaultWarnRatio)
	}
	if l.FailBytes <= 0 {
		l.FailBytes = l.LimitBytes
	}

	res := &Result{
		CoscaDir:   opts.CoscaDir,
		LimitBytes: l.LimitBytes,
		WarnBytes:  l.WarnBytes,
		FailBytes:  l.FailBytes,
		Databases:  make([]Report, 0, len(opts.Targets)),
		Passed:     true,
	}

	seen := make(map[string]bool, len(opts.Targets))

	// 1. Módulos alvo — sempre reportados (mesmo quando não existem).
	for _, t := range opts.Targets {
		abs := filepath.Join(opts.CoscaDir, filepath.FromSlash(t.Path))
		abs = filepath.Clean(abs)
		if seen[abs] {
			continue
		}
		seen[abs] = true
		rep := measure(abs, t.Name, opts.CoscaDir, l)
		res.Databases = append(res.Databases, rep)
	}

	// 2. Todos os *.db no disco (deduplicados com os alvos).
	if opts.IncludeAll {
		absList := findDBs(opts.CoscaDir)
		// findDBs pode incluir o conhecimento alvo; os alvos já foram medidos.
		for _, abs := range absList {
			abs = filepath.Clean(abs)
			if seen[abs] {
				continue
			}
			seen[abs] = true
			rep := measure(abs, deriveName(opts.CoscaDir, abs), opts.CoscaDir, l)
			res.Databases = append(res.Databases, rep)
		}
	}

	// Ordena o relatório por caminho relativo (determinístico e legível).
	sort.SliceStable(res.Databases, func(i, j int) bool {
		return res.Databases[i].RelPath < res.Databases[j].RelPath
	})

	// Veredito.
	for i := range res.Databases {
		rep := &res.Databases[i]
		if rep.Status == StatusWarn {
			res.AnyWarn = true
		}
		if rep.Status == StatusFail {
			res.AnyFail = true
			res.Passed = false
		}
	}

	return res, nil
}

// Classify devolve o estado de um banco dado o tamanho e os limiares. O
// bloqueio (fail) tem precedência: se o banco cruzou o teto, é fail mesmo que
// o limiar de alerta esteja acima do teto.
func Classify(size, warnBytes, failBytes int64) Status {
	if size > failBytes {
		return StatusFail
	}
	if size >= warnBytes {
		return StatusWarn
	}
	return StatusOK
}

// measure abre um único banco SQLite em modo somente-leitura, calcula o
// tamanho on-disk via page_count*page_size e classifica o status. Nunca
// escreve no arquivo. Um arquivo ausente é reportado como not_found (sem
// quebrar o gate); um arquivo ilegível/corrompido é reportado como warn com o
// erro preenchido (não bloqueia o gate, apenas alerta).
func measure(absPath, name, coscaDir string, l Limits) Report {
	rep := Report{
		Name:    name,
		Path:    absPath,
		RelPath: relativeTo(coscaDir, absPath),
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			rep.Found = false
			rep.Status = StatusNotFound
			return rep
		}
		rep.Found = true
		rep.Status = StatusWarn
		rep.Error = "stat: " + err.Error()
		return rep
	}
	rep.Found = true
	if info.IsDir() {
		rep.Status = StatusWarn
		rep.Error = "caminho é um diretório, não um banco SQLite"
		return rep
	}

	db, err := openReadOnly(absPath)
	if err != nil {
		rep.Status = StatusWarn
		rep.Error = "open: " + err.Error()
		return rep
	}
	defer db.Close()

	if err := db.QueryRow("PRAGMA page_count").Scan(&rep.PageCount); err != nil {
		rep.Status = StatusWarn
		rep.Error = "page_count: " + err.Error()
		return rep
	}
	if err := db.QueryRow("PRAGMA page_size").Scan(&rep.PageSize); err != nil {
		rep.Status = StatusWarn
		rep.Error = "page_size: " + err.Error()
		return rep
	}

	rep.DBSizeBytes = rep.PageCount * rep.PageSize
	rep.PercentOf100MB = float64(rep.DBSizeBytes) / float64(ReferenceCeiling)
	rep.PercentOfLimit = float64(rep.DBSizeBytes) / float64(l.LimitBytes)
	rep.Status = Classify(rep.DBSizeBytes, l.WarnBytes, l.FailBytes)
	return rep
}

// openReadOnly abre um banco SQLite em modo somente leitura (mode=ro) de forma
// cross-platform (Windows usa path com barras invertidas → `file:` URI com
// forward slashes). Valida com Ping para detectar arquivo que não é SQLite
// válido.
func openReadOnly(absPath string) (*sql.DB, error) {
	dsn := "file:" + filepath.ToSlash(absPath) + "?mode=ro"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// findDBs varre recursivamente o `.cosca` e devolve os caminhos absolutos de
// todos os arquivos com extensão `.db`. Ignora diretórios e arquivos auxiliares
// (-wal/-shm, que não terminam em .db). Devolve lista vazia sem erro se o
// `.cosca` não existir.
func findDBs(coscaDir string) []string {
	var out []string
	_ = filepath.WalkDir(coscaDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return nil // best-effort: não derruba o gate por um subcaminho ilegível
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".db") {
			if abs, err := filepath.Abs(path); err == nil {
				out = append(out, abs)
			}
		}
		return nil
	})

	if len(out) == 0 {
		return out
	}
	sort.Strings(out)
	return out
}

// deriveName gera um rótulo legível a partir do caminho relativo ao `.cosca`
// (remove a extensão .db e usa "/" como separador). Exemplos:
// "knowledge.db" → "knowledge"; "memory/index.db" → "memory/index".
func deriveName(coscaDir, absPath string) string {
	rel := relativeTo(coscaDir, absPath)
	rel = strings.TrimSuffix(rel, filepath.Ext(rel))
	rel = strings.TrimSuffix(rel, ".db")
	rel = filepath.ToSlash(rel)
	if rel == "" {
		return filepath.Base(absPath)
	}
	return rel
}

// relativeTo devolve o caminho de absPath relativo a coscaDir. Se absPath não
// for descendente de coscaDir, devolve o path absoluto (fallback seguro).
func relativeTo(coscaDir, absPath string) string {
	rel, err := filepath.Rel(coscaDir, absPath)
	if err != nil || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(absPath)
	}
	if rel == "." {
		return filepath.Base(absPath)
	}
	return filepath.ToSlash(rel)
}
