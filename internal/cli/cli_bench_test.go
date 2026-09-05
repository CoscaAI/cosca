package cli

import (
	"io"
	"testing"
)

func BenchmarkRootCommandCreation(b *testing.B) {
	for i := 0; b.Loop(); i++ {
		NewRootCommand()
	}
}

func BenchmarkOutputFormatterNew(b *testing.B) {
	for i := 0; b.Loop(); i++ {
		NewOutputFormatter(io.Discard, OutputFormatText, false, false, false)
	}
}

func BenchmarkOutputFormatterPrintln(b *testing.B) {
	f := NewOutputFormatter(io.Discard, OutputFormatText, false, false, false)

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		f.Println("test output line for benchmarking")
	}
}

func BenchmarkOutputFormatterPrintf(b *testing.B) {
	f := NewOutputFormatter(io.Discard, OutputFormatText, false, false, false)

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		f.Printf("test %s %d %f", "output", 42, 3.14)
	}
}

func BenchmarkOutputFormatterJSON(b *testing.B) {
	f := NewOutputFormatter(io.Discard, OutputFormatJSON, false, false, false)
	data := map[string]interface{}{
		"name":    "test",
		"version": "1.0.0",
		"items":   []int{1, 2, 3, 4, 5},
	}

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_ = f.Print(data)
	}
}

func BenchmarkOutputFormatterYAML(b *testing.B) {
	f := NewOutputFormatter(io.Discard, OutputFormatYAML, false, false, false)
	data := map[string]interface{}{
		"name":    "test",
		"version": "1.0.0",
		"items":   []int{1, 2, 3, 4, 5},
	}

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_ = f.Print(data)
	}
}

func BenchmarkOutputFormatterVerbose(b *testing.B) {
	f := NewOutputFormatter(io.Discard, OutputFormatText, true, false, false)

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		f.Verbose("verbose message for benchmarking")
	}
}

func BenchmarkProgressBarCreate(b *testing.B) {
	for i := 0; b.Loop(); i++ {
		NewProgressBar(io.Discard, 100, "benchmark progress", false)
	}
}

func BenchmarkSpinnerCreate(b *testing.B) {
	for i := 0; b.Loop(); i++ {
		NewSpinner(io.Discard, "benchmark spinner", false)
	}
}

func BenchmarkFormatError(b *testing.B) {
	err := io.ErrUnexpectedEOF
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_ = FormatError(err)
	}
}
