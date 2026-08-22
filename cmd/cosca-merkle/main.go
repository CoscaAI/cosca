// Command cosca-merkle regenerates the cosca-kernel memory blockchain Merkle
// metadata (merkle/epoch_*.json + merkle/index.json) from chain.dat.
//
// The Merkle construction is deliberately simple and deterministic:
//
//   - Leaves  = ordered block hashes from chain.dat (the 64-char hex string
//     of each block, in chain order).
//   - Tree    = balanced binary SHA-256 tree over the leaves: each internal
//     node is sha256(left_hex || right_hex) with the hex strings encoded as
//     UTF-8 bytes; an odd node is promoted unchanged to the next level.
//   - Root    = the single hash at the top of the tree.
//
// The block files (blocks/*.md) and chain.dat are treated as read-only
// inputs — this tool only rewrites the Merkle metadata.
//
// Usage: go run ./cmd/cosca-merkle -dir internal/embed/cosca/memory/agent/cosca-kernel
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	genesis   = "0000000000000000000000000000000000000000000000000000000000000000"
	epochSize = 32
)

// blockHash is a parsed row of chain.dat.
type blockHash struct {
	hash string
	prev string
}

func main() {
	dir := flag.String("dir", "internal/embed/cosca/memory/agent/cosca-kernel", "kernel directory holding chain.dat and merkle/")
	flag.Parse()

	chainPath := filepath.Join(*dir, "chain.dat")
	rows, err := readChain(chainPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read chain: %v\n", err)
		os.Exit(1)
	}
	if len(rows) == 0 {
		fmt.Fprintln(os.Stderr, "no blocks found in chain.dat")
		os.Exit(1)
	}
	fmt.Printf("loaded %d blocks from %s\n", len(rows), chainPath)

	// Split into epochs of epochSize blocks.
	var epochs []epochMeta
	for start := 0; start < len(rows); start += epochSize {
		end := start + epochSize - 1
		if end >= len(rows) {
			end = len(rows) - 1
		}
		leaves := make([]string, 0, end-start+1)
		for i := start; i <= end; i++ {
			leaves = append(leaves, rows[i].hash)
		}
		root := merkleRoot(leaves)
		epochs = append(epochs, epochMeta{
			Epoch:       len(epochs),
			StartBlock:  start,
			EndBlock:    end,
			BlockCount:  len(leaves),
			MerkleRoot:  root,
			FirstBlock:  rows[start].hash,
			LastBlock:   rows[end].hash,
		})
	}

	merkleDir := filepath.Join(*dir, "merkle")
	if err := os.MkdirAll(merkleDir, 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir merkle: %v\n", err)
		os.Exit(1)
	}

	for _, ep := range epochs {
		if err := writeEpoch(merkleDir, ep); err != nil {
			fmt.Fprintf(os.Stderr, "write epoch %d: %v\n", ep.Epoch, err)
			os.Exit(1)
		}
		fmt.Printf("epoch %d: blocks %d-%d (%d) root=%s\n", ep.Epoch, ep.StartBlock, ep.EndBlock, ep.BlockCount, ep.MerkleRoot)
	}

	idx := indexMeta{
		TotalBlocks: len(rows),
		EpochSize:   epochSize,
		TotalEpochs: len(epochs),
	}
	for _, ep := range epochs {
		idx.Epochs = append(idx.Epochs, indexEpoch{Epoch: ep.Epoch, Root: ep.MerkleRoot})
	}
	if err := writeIndex(merkleDir, idx); err != nil {
		fmt.Fprintf(os.Stderr, "write index: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %d epoch files + index.json in %s\n", len(epochs), merkleDir)
}

// readChain parses chain.dat, skipping comment lines. It verifies the chain
// links (each row's prev == previous row's hash, first row == genesis) and
// aborts on any break so we never rebuild metadata from a tampered ledger.
func readChain(path string) ([]blockHash, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rows []blockHash
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 2 {
			return nil, fmt.Errorf("malformed row: %q", line)
		}
		rows = append(rows, blockHash{hash: strings.TrimSpace(parts[0]), prev: strings.TrimSpace(parts[1])})
	}
	for i, r := range rows {
		if i == 0 {
			if r.prev != genesis {
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

// merkleRoot computes the SHA-256 Merkle root over the ordered leaves.
func merkleRoot(leaves []string) string {
	if len(leaves) == 1 {
		// Single-leaf epoch: the root IS the leaf (the block hash itself).
		// Returning hex(ASCII) here would produce an invalid 128-char root.
		return leaves[0]
	}
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
	return hex.EncodeToString(level[0])
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

func writeEpoch(dir string, ep epochMeta) error {
	path := filepath.Join(dir, fmt.Sprintf("epoch_%04d.json", ep.Epoch))
	return writeJSON(path, ep)
}

func writeIndex(dir string, idx indexMeta) error {
	return writeJSON(filepath.Join(dir, "index.json"), idx)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}
