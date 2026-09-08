package learning

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// RebuildResult reports the outcome of a full vault rebuild.
type RebuildResult struct {
	Vaults        map[VaultName]int // vault → trigger count
	TotalTriggers int
	MissingBlocks int // chain rows without a block file (historical holes)
	AgentsScanned int
}

// Rebuild recreates ALL vaults from zero from the memory blockchain:
// chain.dat (ledger) + blocks/{hash}.md (content headers). Deterministic,
// idempotent, no LLM. The vaults dir is wiped first (derived artifacts).
func Rebuild(vaultDir, agentsRoot string) (*RebuildResult, error) {
	res := &RebuildResult{Vaults: make(map[VaultName]int)}

	// 1. Wipe vault dir (derived — regenerable).
	if err := os.RemoveAll(vaultDir); err != nil {
		return nil, fmt.Errorf("learning: wipe vault dir: %w", err)
	}
	if err := os.MkdirAll(vaultDir, 0o700); err != nil {
		return nil, fmt.Errorf("learning: mkdir vault dir: %w", err)
	}

	// 2. Scan every agent dir.
	agents, err := os.ReadDir(agentsRoot)
	if err != nil {
		return nil, fmt.Errorf("learning: read agents root: %w", err)
	}

	openVaults := make(map[VaultName]*sql.DB)
	defer func() {
		for _, db := range openVaults {
			_ = db.Close()
		}
	}()

	vaultDB := func(v VaultName) (*sql.DB, error) {
		if db, ok := openVaults[v]; ok {
			return db, nil
		}
		db, err := OpenVault(vaultDir, v)
		if err != nil {
			return nil, err
		}
		openVaults[v] = db
		return db, nil
	}

	for _, agent := range agents {
		if !agent.IsDir() {
			continue
		}
		agentName := agent.Name()
		agentDir := filepath.Join(agentsRoot, agentName)
		res.AgentsScanned++

		chainPath := filepath.Join(agentDir, "chain.dat")
		if _, err := os.Stat(chainPath); err != nil {
			continue // no chain — no registered learnings
		}

		rows, err := readChainRows(chainPath)
		if err != nil {
			return nil, fmt.Errorf("learning: read chain %s: %w", agentName, err)
		}

		vault := VaultForAgent(agentName)
		db, err := vaultDB(vault)
		if err != nil {
			return nil, err
		}

		for _, row := range rows {
			t := Trigger{
				ID:        row.id,
				Agent:     agentName,
				Vault:     vault,
				Date:      row.date,
				Title:     row.title,
				Hash16:    hash16(row.hash),
				BlockHash: row.hash,
				ChainPrev: row.prev,
			}

			// Enrich LEVEL/TAGS from the immutable block header.
			blockPath := filepath.Join(agentDir, "blocks", row.hash+".md")
			if content, err := os.ReadFile(blockPath); err == nil {
				t.Level = blockLevel(string(content))
				t.Tags = blockTags(string(content))
			} else {
				res.MissingBlocks++
			}

			if err := UpsertTrigger(db, t); err != nil {
				return nil, err
			}
			res.TotalTriggers++
			res.Vaults[vault]++
		}
	}

	return res, nil
}

// chainRow is one parsed row of chain.dat: hash|prev|date|id|title.
type chainRow struct {
	hash  string
	prev  string
	date  string
	id    string
	title string
}

// readChainRows parses chain.dat, skipping comment lines (#).
func readChainRows(path string) ([]chainRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var rows []chainRow
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) < 5 {
			continue
		}
		rows = append(rows, chainRow{
			hash:  strings.TrimSpace(parts[0]),
			prev:  strings.TrimSpace(parts[1]),
			date:  strings.TrimSpace(parts[2]),
			id:    strings.TrimSpace(parts[3]),
			title: strings.TrimSpace(parts[4]),
		})
	}
	return rows, sc.Err()
}

func hash16(hash string) string {
	if len(hash) > 16 {
		return hash[:16]
	}
	return hash
}

var levelRe = regexp.MustCompile(`(?m)^LEVEL:\s*(\d+)`)
var tagsRe = regexp.MustCompile(`(?m)^TAGS:\s*(.+?)\s*$`)

func blockLevel(content string) int {
	m := levelRe.FindStringSubmatch(content)
	if len(m) < 2 {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return n
}

func blockTags(content string) string {
	m := tagsRe.FindStringSubmatch(content)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}