// ledger.go — ledger persistente de tentativas de repair.
//
// Sidecar `<db>.repair-attempts.json` (mesmo diretório do banco doente).
// Mapeia fingerprint → tentativas. Regra (adaptada de
// `hermes_state_repair.py`): recusa nova cirurgia quando o MESMO fingerprint já
// falhou MaxAttempts (3) vezes — um arquivo diferente (fingerprint diferente,
// ex.: mudou após um truncamento ou restore real) recomeça o orçamento.
//
// Qualquer repair bem-sucedido muda o fingerprint (ou remove o arquivo) — e o
// RecordSuccess remove a entrada; sem o reset, uma corrupção incurável
// re-rodaria backup forense + cirurgia em CADA boot (a lição do incidente:
// 105 tentativas / 89 GB de cópias idênticas no Hermes).
//
// A escrita do ledger é idempotente e best-effort: falha de escrita NUNCA
// derruba o repair (a proteção contra loop é um reforço, não um pré-requisito).
package staterepair

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// MaxAttempts é o orçamento de falhas no MESMO fingerprint antes de o ledger
// recusar nova cirurgia automática (ADR-043 §8 / hermes_state_repair.py).
const MaxAttempts = 3

// Attempt registra o histórico de um fingerprint no ledger.
type Attempt struct {
	// Attempts é o número de falhas consecutivas registradas para este
	// fingerprint.
	Attempts int `json:"attempts"`
	// LastTS é o timestamp (RFC3339 UTC) da última tentativa.
	LastTS string `json:"last_ts"`
	// LastStatus é o estado da última tentativa: "failed" ou "ok".
	LastStatus string `json:"last_status"`
	// Path é o caminho do banco doente (rastreabilidade forense).
	Path string `json:"path"`
}

// ledgerFile é o formato on-disk do sidecar.
type ledgerFile struct {
	// Version permite evoluir o formato sem quebrar ledgers existentes.
	Version int                `json:"version"`
	Entries map[string]Attempt `json:"entries"`
}

const (
	// statusFailed e statusOK são os valores de Attempt.LastStatus.
	statusFailed = "failed"
	statusOK     = "ok"
)

// LedgerPath devolve o caminho do sidecar de tentativas de dbPath:
// `<db>.repair-attempts.json`.
func LedgerPath(dbPath string) string {
	return dbPath + ".repair-attempts.json"
}

// loadLedger lê o sidecar. Um ledger ausente ou corrompido (JSON inválido)
// devolve um ledger vazio — "não exausto" (a regra do Hermes: um ledger quebrado
// nunca bloqueia o repair, os guards em processo ainda limitam).
func loadLedger(dbPath string) ledgerFile {
	lf := ledgerFile{Version: 1, Entries: map[string]Attempt{}}
	raw, err := os.ReadFile(LedgerPath(dbPath))
	if err != nil {
		return lf
	}
	var parsed ledgerFile
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return lf
	}
	if parsed.Entries == nil {
		parsed.Entries = map[string]Attempt{}
	}
	return parsed
}

// saveLedger grava o sidecar (0600) de forma atômica-ish (arquivo temporário +
// rename). Best-effort: erro de escrita é devolvido mas nunca deve derrubar o
// fluxo de repair — os callers registram e seguem.
func saveLedger(dbPath string, lf ledgerFile) error {
	path := LedgerPath(dbPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("mkdir ledger: %w", err)
	}
	raw, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal ledger: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("write ledger tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename ledger: %w", err)
	}
	return nil
}

// nowStamp devolve o timestamp RFC3339 UTC usado no ledger.
func nowStamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// RecordSuccess remove a entrada do fingerprint (idempotente): um repair
// bem-sucedido prova que a corrupção daquele fingerprint foi curada — manter a
// entrada faria o ledger recusar uma cirurgia futura legítima se o MESMO byte
// layout voltasse a corromper. Quando não resta nenhuma entrada, o sidecar é
// removido (estado "nunca houve tentativa").
func RecordSuccess(dbPath, fingerprint string) error {
	if fingerprint == "" {
		return nil
	}
	lf := loadLedger(dbPath)
	if _, ok := lf.Entries[fingerprint]; !ok {
		return nil // idempotente: nada a remover
	}
	delete(lf.Entries, fingerprint)
	if len(lf.Entries) == 0 {
		_ = os.Remove(LedgerPath(dbPath))
		return nil
	}
	return saveLedger(dbPath, lf)
}

// RecordFailure incrementa o contador de falhas do fingerprint (idempotente):
// mesmo fingerprint → attempts+1; fingerprint diferente → recomeça em 1 (o
// arquivo mudou — o orçamento é POR arquivo doente). Um fingerprint vazio não
// é registrado (sem identidade não existe chave segura).
func RecordFailure(dbPath, fingerprint string) error {
	if fingerprint == "" {
		return nil
	}
	lf := loadLedger(dbPath)
	entry := lf.Entries[fingerprint]
	if entry.Attempts == 0 || entry.LastStatus != statusFailed {
		entry.Attempts = 1
	} else {
		entry.Attempts++
	}
	entry.LastTS = nowStamp()
	entry.LastStatus = statusFailed
	entry.Path = dbPath
	lf.Entries[fingerprint] = entry
	return saveLedger(dbPath, lf)
}

// AttemptsExhausted devolve true quando o MESMO fingerprint já registrou
// MaxAttempts falhas no ledger (a corrupção está além das estratégias
// automáticas — provavelmente dano de página b-tree). Um fingerprint
// diferente ou um ledger ausente/corrompido devolvem false (nunca bloqueiam).
func AttemptsExhausted(dbPath, fingerprint string) bool {
	if fingerprint == "" {
		return false
	}
	lf := loadLedger(dbPath)
	entry, ok := lf.Entries[fingerprint]
	if !ok {
		return false
	}
	return entry.Attempts >= MaxAttempts
}

// ReadAttempts devolve uma cópia das entradas do ledger (para relatório/CLI).
func ReadAttempts(dbPath string) map[string]Attempt {
	lf := loadLedger(dbPath)
	out := make(map[string]Attempt, len(lf.Entries))
	for fp, a := range lf.Entries {
		out[fp] = a
	}
	return out
}
