package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CoscaAI/cosca/internal/memory"
	"github.com/spf13/cobra"
)

// NewMemoryEpisodicCommand cria o subcomando `cosca memory episodic` — a consulta
// da memória episódica multimodal (FASE D). Faz o COSCa "lembrar" o que viu/ouviu
// sincronizado e responder a perguntas como "o que estava vendo quando ouvi X às
// 10:32".
//
// Flags:
//
//	--since   limite inferior da janela temporal (RFC3339 | YYYY-MM-DD | "24h" relativo)
//	--until   limite superior (RFC3339 | YYYY-MM-DD)
//	--query   texto/entidade a buscar (AND por palavra)
//	--modality vision | audio | multimodal
//	--limit   máximo de resultados (default 100)
func NewMemoryEpisodicCommand() *cobra.Command {
	var (
		sinceRaw  string
		untilRaw  string
		query     string
		modality  string
		limit     int
	)

	cmd := &cobra.Command{
		Use:   "episodic",
		Short: "Consultar memória episódica multimodal (o COSCa lembra o que viu/ouviu)",
		Long: `Consulta a memória episódica multimodal (FASE D): observações sincronizadas
de visão+áudio persistidas entre sessões. Responde "o que o COSCa estava vendo
quando ouviu determinada coisa".

Apenas a REPRESENTAÇÃO (texto/entidades/relações) é persistida — nunca frames
brutos (política de privacidade).`,
		Example: `  cosca memory episodic
  cosca memory episodic --query "carro vermelho"
  cosca memory episodic --since 24h
  cosca memory episodic --since 2026-08-31T10:30:00Z --until 2026-08-31T11:00:00Z
  cosca memory episodic --since 2026-08-30 --query "abrir o editor" --modality multimodal`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}
			coscaDir := filepath.Join(dir, ".cosca")

			mgr := NewMemoryManager(coscaDir)
			defer func() { _ = mgr.Close() }()
			if mgr == nil {
				return fmt.Errorf("memory manager not available")
			}

			q := memory.EpisodicQuery{
				Query:    query,
				Modality: modality,
				Limit:    limit,
			}
			if sinceRaw != "" {
				t, err := parseEpisodicTime(sinceRaw, true)
				if err != nil {
					return fmt.Errorf("invalid --since: %w", err)
				}
				q.Since = t
			}
			if untilRaw != "" {
				t, err := parseEpisodicTime(untilRaw, false)
				if err != nil {
					return fmt.Errorf("invalid --until: %w", err)
				}
				q.Until = t
			}

			records, err := mgr.QueryEpisodic(context.Background(), q)
			if err != nil {
				return fmt.Errorf("episodic query failed: %w", err)
			}

			if useJSON {
				return printJSON(cmd, map[string]interface{}{
					"records": records,
					"total":   len(records),
				})
			}

			if len(records) == 0 {
				formatter.Warning("Nenhum registro episódico encontrado")
				return nil
			}

			formatter.Header(fmt.Sprintf("Memória Episódica (%d registros)", len(records)))
			for i, r := range records {
				formatter.KeyValue(fmt.Sprintf("#%d", i+1), r.Timestamp.Format(time.RFC3339))
				formatter.KeyValue("  Modality", r.Modality)
				if r.AudioText != "" {
					formatter.KeyValue("  Áudio", r.AudioText)
				}
				if r.VisionSummary != "" {
					formatter.KeyValue("  Visão", truncate(r.VisionSummary, 160))
				}
				if len(r.Entities) > 0 {
					labels := make([]string, 0, len(r.Entities))
					for _, e := range r.Entities {
						labels = append(labels, e.Label)
					}
					formatter.KeyValue("  Entidades", strings.Join(labels, ", "))
				}
				if len(r.MultiRels) > 0 {
					rel := r.MultiRels[0]
					formatter.KeyValue("  Sync (áudio↔visão)", fmt.Sprintf("conf %.2f · %d janela(s) de visão", rel.Confidence, len(rel.Overlap)))
				}
				formatter.Println("")
			}
			formatter.KeyValue("Total", fmt.Sprintf("%d", len(records)))

			return nil
		},
	}

	cmd.Flags().StringVar(&sinceRaw, "since", "", "limite inferior da janela temporal (RFC3339 | YYYY-MM-DD | duração relativa ex. \"24h\")")
	cmd.Flags().StringVar(&untilRaw, "until", "", "limite superior da janela temporal (RFC3339 | YYYY-MM-DD)")
	cmd.Flags().StringVar(&query, "query", "", "texto/entidade para buscar (AND por palavra)")
	cmd.Flags().StringVar(&modality, "modality", "", "filtra por canal: vision | audio | multimodal")
	cmd.Flags().IntVarP(&limit, "limit", "l", 100, "máximo de resultados")
	return cmd
}

// parseEpisodicTime interpreta um valor de tempo do CLI. Quando `relativeOK` é
// true, aceita uma duração relativa (ex. "24h") como "agora - duração". Aceita:
//   - RFC3339 (ex. 2026-08-31T10:30:00Z)
//   - data (YYYY-MM-DD, meia-noite UTC)
//   - data+hora (YYYY-MM-DD HH:MM:SS)
//   - duração relativa (somente quando relativeOK)
func parseEpisodicTime(raw string, relativeOK bool) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	// Duração relativa (ex. "24h" → agora - 24h).
	if relativeOK && !strings.ContainsAny(raw, "T-/:") && !looksLikeDate(raw) {
		d, err := time.ParseDuration(raw)
		if err == nil {
			return time.Now().Add(-d), nil
		}
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02 15:04:05", raw); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t, nil
	}
	// Último recurso: formato curto "2006-01-02T15:04" (sem segundos).
	if t, err := time.Parse("2006-01-02T15:04", raw); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unrecognized time format %q (use RFC3339, YYYY-MM-DD, ou duração relativa)", raw)
}

func looksLikeDate(raw string) bool {
	if len(raw) < 8 {
		return false
	}
	return strings.Count(raw, "-") >= 2
}
