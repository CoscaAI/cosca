package intelligence

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ============================================================
// PROVIDER DE MEMÓRIA — conecta o motor ao conhecimento real
// ============================================================
//
// MemoryProvider escaneia os learnings.md da memória curada
// (.cosca/memory/agent/<agent>/learnings.md) e transforma cada linha de
// índice em um Source que o Intelligence Engine consome.
//
// É determinístico: o nível/evidência/confiança são derivados de metadados
// observáveis (hash de provenance, Level N, data) — não de "achismo" de LLM.

var (
	indexLineRe = regexp.MustCompile(`^##\s+`) // linha de índice
	levelRe     = regexp.MustCompile(`(?i)\bL(?:evel)?\s*([1-5])\b`)
	hashRe      = regexp.MustCompile(`[0-9a-f]{16}`)
	dateRe      = regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})`)
)

// MemoryProvider retorna um SourceProvider que lê os learnings de memoryAgentDir.
func MemoryProvider(memoryAgentDir string) SourceProvider {
	return func(ctx context.Context) ([]Source, error) {
		var srcs []Source

		agentDirs, err := os.ReadDir(filepath.Join(memoryAgentDir, "agent"))
		if err != nil {
			return nil, err
		}

		for _, ad := range agentDirs {
			if !ad.IsDir() {
				continue
			}
			agent := ad.Name()
			lmPath := filepath.Join(memoryAgentDir, "agent", agent, "learnings.md")
			srcs = append(srcs, parseLearningsFile(agent, lmPath)...)
		}

		sort.SliceStable(srcs, func(i, j int) bool {
			return srcs[i].ID < srcs[j].ID
		})
		return srcs, nil
	}
}

// parseLearningsFile transforma as linhas de índice de um learnings.md em Sources.
func parseLearningsFile(agent, path string) []Source {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return parseLearnings(agent, string(data))
}

// parseLearnings é a versão testável do parse (sem I/O).
func parseLearnings(agent, content string) []Source {
	var srcs []Source
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if !indexLineRe.MatchString(line) {
			continue
		}
		srcs = append(srcs, lineToSource(agent, strings.TrimSpace(line)))
	}
	return srcs
}

// lineToSource converte uma linha de índice em um Source determinístico.
func lineToSource(agent, line string) Source {
	// split por "|" (formato do índice)
	parts := strings.Split(line, "|")
	content := line

	// ID estável: agent + hash de provenance (se houver)
	id := agent
	if h := hashRe.FindString(line); h != "" {
		id = agent + "/" + h
	}

	// nível (Level N) → confidence
	level := 0
	if m := levelRe.FindStringSubmatch(line); m != nil {
		var l int
		for _, c := range m[1] {
			l = l*10 + int(c-'0')
		}
		level = l
	}
	confidence := 0.5
	if level >= 1 && level <= 5 {
		confidence = float64(level) / 5.0
	}

	// evidência: presença de hash de provenance observável = forte
	evidence := 3 // documentação (nível 3)
	if hashRe.MatchString(line) {
		evidence = 5 // prova observável (código/commit/hash)
	} else if strings.Contains(strings.ToLower(line), "commit") || strings.Contains(strings.ToLower(line), "test") {
		evidence = 4
	}

	// tópico = agente (ou primeira tag, se houver).
	// Pula o 1º campo ("## data") que também começa com '#'.
	topic := agent
	for _, part := range parts[1:] {
		t := strings.TrimSpace(part)
		if strings.HasPrefix(t, "#") {
			tags := strings.TrimPrefix(t, "#")
			if strings.TrimSpace(tags) != "" {
				topic = strings.Fields(tags)[0]
			}
			break
		}
	}

	// recência: data do arquivo (fallback) — a data do índice é ambígua
	recency := time.Now()
	if m := dateRe.FindString(line); m != "" {
		if t, err := time.Parse("2006-01-02", m); err == nil {
			recency = t
		}
	}

	return Source{
		ID:         id,
		Content:    content,
		Topic:      topic,
		Evidence:   evidence,
		Confidence: confidence,
		Recency:    recency,
	}
}
