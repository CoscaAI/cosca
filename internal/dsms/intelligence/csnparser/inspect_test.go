package csnparser

import (
	"os"
	"testing"

	"github.com/parquet-go/parquet-go"
)

// TestInspectParquetSchema inspects the real parquet schema using low-level reader.
func TestInspectParquetSchema(t *testing.T) {
	file := `E:\trainer\0000 (1).parquet`
	f, err := os.Open(file)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	// Use low-level reader to see raw values
	reader := parquet.NewReader(f)
	defer reader.Close()

	// Read first row as parquet.Row (raw values)
	rows := make([]parquet.Row, 1)
	n, err := reader.ReadRows(rows)
	if err != nil || n == 0 {
		t.Fatalf("read rows: %v (n=%d)", err, n)
	}

	t.Logf("=== PRIMEIRA LINHA (raw) ===")
	for _, value := range rows[0] {
		t.Logf("  col[%d] = %v", value.Column(), value)
	}
}