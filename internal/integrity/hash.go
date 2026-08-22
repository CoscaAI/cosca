package integrity

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	"github.com/zeebo/blake3"
)

// HashAlgo identifies the content-addressing hash algorithm used by a chain
// block. New blocks are BLAKE3; historical blocks default to SHA-256 (the
// legacy format) for backward compatibility.
type HashAlgo string

const (
	// HashSHA256 is the legacy content hash (Go crypto/sha256). Blocks
	// without a HASH_ALGO field are treated as sha256.
	HashSHA256 HashAlgo = "sha256"
	// HashBLAKE3 is the modern content hash (github.com/zeebo/blake3).
	HashBLAKE3 HashAlgo = "blake3"
)

// Valid reports whether the algorithm is one of the supported hash algorithms.
func (a HashAlgo) Valid() bool {
	return a == HashSHA256 || a == HashBLAKE3
}

// HashBytes returns the hex digest of data using the given algorithm.
func HashBytes(algo HashAlgo, data []byte) string {
	switch algo {
	case HashBLAKE3:
		return fmt.Sprintf("%x", blake3.Sum256(data))
	default:
		return fmt.Sprintf("%x", sha256.Sum256(data))
	}
}

// HashFile reads the file at path and returns its hex digest using the given
// algorithm.
func HashFile(algo HashAlgo, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return HashReader(algo, f)
}

// HashReader reads all of r and returns its hex digest using the given
// algorithm.
func HashReader(algo HashAlgo, r io.Reader) (string, error) {
	switch algo {
	case HashBLAKE3:
		h := blake3.New()
		if _, err := io.Copy(h, r); err != nil {
			return "", err
		}
		return fmt.Sprintf("%x", h.Sum(nil)), nil
	default:
		h := sha256.New()
		if _, err := io.Copy(h, r); err != nil {
			return "", err
		}
		return fmt.Sprintf("%x", h.Sum(nil)), nil
	}
}

// sha256File computes SHA-256 of a file. Kept for the legacy code paths and
// routed through the hash abstraction.
func sha256File(path string) (string, error) {
	return HashFile(HashSHA256, path)
}
