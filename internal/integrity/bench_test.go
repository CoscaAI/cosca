package integrity

import (
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkParallelFileHashes measures the per-file hash verification loop
// (1283 files) with 1 worker (sequential) vs fileHashWorkers (parallel).
func BenchmarkParallelFileHashes(b *testing.B) {
	root := b.TempDir()
	data := make([]byte, 256*1024)
	for i := 0; i < len(data); i++ {
		data[i] = byte(i)
	}

	manifest := make([]FileEntry, 0, 1283)
	for i := 0; i < 1283; i++ {
		p := filepath.Join(root, fname(i))
		if err := os.WriteFile(p, data, 0644); err != nil {
			b.Fatal(err)
		}
		manifest = append(manifest, FileEntry{Path: fname(i), Hash: "00"})
	}
	block := Block{Number: 1, HashAlgo: HashBLAKE3, Manifest: manifest}

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		verifyFileHashes(root, &block, 1)
	}
}

func BenchmarkParallelFileHashesWorkers(b *testing.B) {
	root := b.TempDir()
	data := make([]byte, 256*1024)
	for i := 0; i < len(data); i++ {
		data[i] = byte(i)
	}

	manifest := make([]FileEntry, 0, 1283)
	for i := 0; i < 1283; i++ {
		p := filepath.Join(root, fname(i))
		if err := os.WriteFile(p, data, 0644); err != nil {
			b.Fatal(err)
		}
		manifest = append(manifest, FileEntry{Path: fname(i), Hash: "00"})
	}
	block := Block{Number: 1, HashAlgo: HashBLAKE3, Manifest: manifest}

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		verifyFileHashes(root, &block, fileHashWorkers)
	}
}

func fname(i int) string {
	const digits = "0123456789"
	buf := make([]byte, 4)
	for j := 3; j >= 0; j-- {
		buf[j] = digits[i%10]
		i /= 10
	}
	return "f" + string(buf) + ".bin"
}
