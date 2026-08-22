package cli

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/CoscaAI/cosca/internal/codeindex"
	"github.com/CoscaAI/cosca/internal/config"
	"github.com/CoscaAI/cosca/internal/embeddings"
	"github.com/CoscaAI/cosca/internal/sqlite"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

const symbolsTableSchema = `
CREATE TABLE IF NOT EXISTS code_symbols (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL,
	kind       TEXT NOT NULL DEFAULT 'func',
	signature  TEXT NOT NULL DEFAULT '',
	package    TEXT NOT NULL DEFAULT '',
	file       TEXT NOT NULL DEFAULT '',
	line       INTEGER NOT NULL DEFAULT 0,
	doc        TEXT NOT NULL DEFAULT '',
	embedding  BLOB
);
CREATE INDEX IF NOT EXISTS idx_code_symbols_name ON code_symbols(name);
CREATE INDEX IF NOT EXISTS idx_code_symbols_kind ON code_symbols(kind);
`

// NewSymbolsCommand cria o comando `cosca symbols` — gatilho semântico de código.
func NewSymbolsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "symbols",
		Short: "Índice e busca semântica de símbolos de código Go (funções, métodos, tipos)",
		Long:  "Extrai símbolos do código Go via AST e indexa no knowledge.db com embedding semântico — achar a função pelo que ela FAZ, não só pelo nome.",
	}
	cmd.AddCommand(newSymbolsIndexCommand())
	cmd.AddCommand(newSymbolsSearchCommand())
	return cmd
}

// setupEmbedding configura o registry de embedding (ollama/nomic-embed-text).
func setupEmbedding() (*embeddings.ProviderRegistry, error) {
	reg := embeddings.GetRegistry()
	cfg := embeddings.DefaultProviderRegistryConfig()
	if c, err := config.Load(); err == nil {
		cfg.BaseURL = c.Embedding.BaseURL
		cfg.Model = c.Embedding.Model
		cfg.APIKey = c.Embedding.APIKey
		if c.Embedding.Dimensions > 0 {
			cfg.Dimensions = c.Embedding.Dimensions
		}
	}
	if err := reg.Select(context.Background(), cfg); err != nil {
		return nil, err
	}
	return reg, nil
}

func symbolText(s codeindex.Symbol) string {
	return strings.TrimSpace(s.Name + " " + s.Signature + " " + s.Doc)
}

func cosine(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// encodeVec encodes a float64 embedding as a binary little-endian BLOB
// (8 bytes per value). Byte-exact — decoding returns the identical float64s
// the JSON format carried, so existing scores/rankings do not change.
func encodeVec(v []float64) []byte {
	buf := make([]byte, len(v)*8)
	for i, val := range v {
		binary.LittleEndian.PutUint64(buf[i*8:], math.Float64bits(val))
	}
	return buf
}

// decodeVec decodes a code_symbols embedding: binary float64 BLOB (current) or
// JSON float64 array (legacy rows).
func decodeVec(b []byte) []float64 {
	if len(b) == 0 {
		return nil
	}
	if b[0] == '[' {
		return fastParseFloatArray(b)
	}
	if len(b)%8 != 0 {
		return nil
	}
	n := len(b) / 8
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		out[i] = math.Float64frombits(binary.LittleEndian.Uint64(b[i*8:]))
	}
	return out
}

// fastParseFloatArray parses a JSON float64 array without reflection. Returns
// nil on malformed input (same fallback as the old json.Unmarshal error path).
func fastParseFloatArray(data []byte) []float64 {
	i := 0
	for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r') {
		i++
	}
	if i >= len(data) || data[i] != '[' {
		return nil
	}
	i++
	out := make([]float64, 0, 32)
	for {
		for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r') {
			i++
		}
		if i >= len(data) {
			return nil
		}
		if data[i] == ']' {
			return out
		}
		start := i
		for i < len(data) && data[i] != ',' && data[i] != ']' &&
			!(data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r') {
			i++
		}
		if start == i {
			return nil
		}
		f, err := strconv.ParseFloat(string(data[start:i]), 64)
		if err != nil {
			return nil
		}
		out = append(out, f)
		for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r') {
			i++
		}
		if i >= len(data) {
			return nil
		}
		switch data[i] {
		case ',':
			i++
		case ']':
			return out
		default:
			return nil
		}
	}
}

func newSymbolsIndexCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index [dir]",
		Short: "Extrai símbolos Go, gera embeddings e indexa na tabela code_symbols",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := "."
			if len(args) > 0 {
				target = args[0]
			}

			symbols, err := codeindex.WalkDir(target)
			if err != nil {
				return err
			}
			if len(symbols) == 0 {
				return fmt.Errorf("nenhum símbolo Go encontrado em %q", target)
			}

			reg, err := setupEmbedding()
			if err != nil {
				return fmt.Errorf("embedding provider: %w", err)
			}

			// Gerar embeddings em batch
			texts := make([]string, len(symbols))
			for i, s := range symbols {
				texts[i] = symbolText(s)
			}
			results, err := reg.GenerateEmbeddings(context.Background(), texts)
			if err != nil {
				return fmt.Errorf("gerar embeddings: %w", err)
			}

			wd, _ := os.Getwd()
			dbPath := filepath.Join(wd, ".cosca", "knowledge.db")
			db, err := sqlite.Open(sqlite.DefaultConfig(dbPath))
			if err != nil {
				return err
			}
			defer db.Close()

			unlock := db.LockWriter()
			defer unlock()
			// Recria a tabela para garantir o schema com a coluna embedding
			// (migração de versões anteriores sem a coluna).
			if _, err := db.Exec("DROP TABLE IF EXISTS code_symbols"); err != nil {
				return fmt.Errorf("drop table: %w", err)
			}
			if _, err := db.Exec(symbolsTableSchema); err != nil {
				return fmt.Errorf("create table: %w", err)
			}

			for i, s := range symbols {
				var emb []byte
				if i < len(results) && results[i] != nil {
					emb = encodeVec(results[i].Vector)
				}
				if _, err := db.Exec(
					`INSERT INTO code_symbols (id, name, kind, signature, package, file, line, doc, embedding)
					 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					uuid.New().String(), s.Name, s.Kind, s.Signature, s.Package, s.File, s.Line, s.Doc, emb,
				); err != nil {
					return fmt.Errorf("insert %s: %w", s.Name, err)
				}
			}

			fmt.Printf("✓ %d símbolos indexados com embedding semântico\n", len(symbols))
			return nil
		},
	}
	return cmd
}

func newSymbolsSearchCommand() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Busca semântica de símbolos de código (por descrição, não só nome)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]

			wd, _ := os.Getwd()
			dbPath := filepath.Join(wd, ".cosca", "knowledge.db")
			db, err := sqlite.Open(sqlite.DefaultConfig(dbPath))
			if err != nil {
				return err
			}
			defer db.Close()

			if _, err := db.Exec(symbolsTableSchema); err != nil {
				return err
			}

			// Tenta busca semântica (embedding). Se falhar, cai para LIKE.
			reg, regErr := setupEmbedding()
			if regErr == nil {
				qres, qerr := reg.GenerateEmbedding(context.Background(), query)
				if qerr == nil && qres != nil && len(qres.Vector) > 0 {
					rows, err := db.Query(
						`SELECT name, kind, signature, package, file, line, embedding
						 FROM code_symbols WHERE embedding IS NOT NULL`,
					)
					if err == nil {
						type hit struct {
							name, kind, sig, pkg, file string
							line                       int
							score                      float64
						}
						var hits []hit
						for rows.Next() {
							var name, kind, sig, pkg, file string
							var line int
							var emb []byte
							if err := rows.Scan(&name, &kind, &sig, &pkg, &file, &line, &emb); err != nil {
								continue
							}
							if len(emb) == 0 {
								continue
							}
							score := cosine(qres.Vector, decodeVec(emb))
							hits = append(hits, hit{name, kind, sig, pkg, file, line, score})
						}
						rows.Close()
						// Ordena por score desc
						for i := 0; i < len(hits); i++ {
							for j := i + 1; j < len(hits); j++ {
								if hits[j].score > hits[i].score {
									hits[i], hits[j] = hits[j], hits[i]
								}
							}
						}
						if len(hits) > limit {
							hits = hits[:limit]
						}
						if len(hits) > 0 {
							for _, h := range hits {
								fmt.Printf("%-8s %-45s %.3f  %s:%d\n", h.kind, h.name, h.score, h.file, h.line)
								if h.sig != "" {
									fmt.Printf("         %s\n", h.sig)
								}
							}
							fmt.Printf("\n%d símbolo(s) — busca SEMÂNTICA para %q\n", len(hits), query)
							return nil
						}
					}
				}
			}

			// Fallback: busca textual (LIKE)
			rows, err := db.Query(
				`SELECT name, kind, signature, package, file, line FROM code_symbols
				 WHERE name LIKE ? OR signature LIKE ? OR doc LIKE ?
				 ORDER BY name LIMIT ?`,
				"%"+query+"%", "%"+query+"%", "%"+query+"%", limit,
			)
			if err != nil {
				return err
			}
			defer rows.Close()

			n := 0
			for rows.Next() {
				var name, kind, sig, pkg, file string
				var line int
				if err := rows.Scan(&name, &kind, &sig, &pkg, &file, &line); err != nil {
					return err
				}
				fmt.Printf("%-8s %-45s %s:%d\n", kind, name, file, line)
				n++
			}
			fmt.Printf("\n%d símbolo(s) — busca TEXTUAL para %q\n", n, query)
			return nil
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "l", 10, "número máximo de resultados")
	return cmd
}
