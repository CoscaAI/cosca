package oracle

import (
	"os"
)

// readFileImpl reads a file from disk.
func readFileImpl(path string) ([]byte, error) {
	return os.ReadFile(path)
}
