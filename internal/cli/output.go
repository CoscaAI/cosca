package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/CoscaAI/cosca/internal/telemetry"
)

// OutputFormat represents the output format for commands.
type OutputFormat string

// Predefined output formats.
const (
	OutputFormatText  OutputFormat = "text"
	OutputFormatJSON  OutputFormat = "json"
	OutputFormatYAML  OutputFormat = "yaml"
	OutputFormatTable OutputFormat = "table"
)

// ColorScheme defines colors used in terminal output.
type ColorScheme struct {
	Reset  string
	Red    string
	Green  string
	Yellow string
	Blue   string
	Purple string
	Cyan   string
	White  string
	Bold   string
	Dim    string
}

// DefaultColorScheme returns the default ANSI color scheme.
func DefaultColorScheme() ColorScheme {
	return ColorScheme{
		Reset:  "\033[0m",
		Red:    "\033[31m",
		Green:  "\033[32m",
		Yellow: "\033[33m",
		Blue:   "\033[34m",
		Purple: "\033[35m",
		Cyan:   "\033[36m",
		White:  "\033[37m",
		Bold:   "\033[1m",
		Dim:    "\033[2m",
	}
}

// NoColorScheme returns a no-color scheme for --no-color mode.
func NoColorScheme() ColorScheme {
	return ColorScheme{
		Reset: "", Red: "", Green: "", Yellow: "",
		Blue: "", Purple: "", Cyan: "", White: "",
		Bold: "", Dim: "",
	}
}

// syncWriter wraps an io.Writer with a mutex so all writes are
// serialized. This prevents races when Spinner, ProgressBar, and
// OutputFormatter methods all write to the same underlying writer
// from different goroutines.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (sw *syncWriter) Write(p []byte) (n int, err error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.w.Write(p)
}

// OutputFormatter handles formatted output for CLI commands.
type OutputFormatter struct {
	writer  *syncWriter
	format  OutputFormat
	colors  ColorScheme
	verbose bool
	quiet   bool
	noColor bool
	mu      sync.Mutex
}

// NewOutputFormatter creates a new OutputFormatter.
func NewOutputFormatter(writer io.Writer, format OutputFormat, verbose, quiet, noColor bool) *OutputFormatter {
	colors := DefaultColorScheme()
	if noColor {
		colors = NoColorScheme()
	}
	if writer == nil {
		writer = os.Stdout
	}
	// Wrap the writer in a syncWriter so that Spinner and ProgressBar
	// (which may write from a background goroutine) share the same lock.
	sw := &syncWriter{w: writer}
	return &OutputFormatter{
		writer:  sw,
		format:  format,
		colors:  colors,
		verbose: verbose,
		quiet:   quiet,
		noColor: noColor,
	}
}

// SetFormat changes the output format.
func (f *OutputFormatter) SetFormat(format OutputFormat) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.format = format
}

// Print formats and writes output based on the configured format.
func (f *OutputFormatter) Print(v interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch f.format {
	case OutputFormatJSON:
		return f.printJSON(v)
	case OutputFormatYAML:
		return f.printYAML(v)
	case OutputFormatTable:
		return f.printText(v)
	default:
		return f.printText(v)
	}
}

// Println prints a message with a newline.
func (f *OutputFormatter) Println(msg string) {
	if f.quiet {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	_, _ = fmt.Fprintln(f.writer, msg)
}

// Printf prints a formatted message.
func (f *OutputFormatter) Printf(format string, args ...interface{}) {
	if f.quiet {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	_, _ = fmt.Fprintf(f.writer, format, args...)
}

// Verbose prints a message only if verbose mode is enabled.
func (f *OutputFormatter) Verbose(msg string) {
	if !f.verbose || f.quiet {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	_, _ = fmt.Fprintf(f.writer, "%s[verbose]%s %s\n", f.colors.Dim, f.colors.Reset, msg)
}

// Debug prints a debug message only if verbose mode is enabled.
func (f *OutputFormatter) Debug(msg string) {
	if !f.verbose || f.quiet {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	_, _ = fmt.Fprintf(f.writer, "%s[debug]%s %s\n", f.colors.Purple, f.colors.Reset, msg)
}

// Success prints a success message.
func (f *OutputFormatter) Success(msg string) {
	if f.quiet {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	_, _ = fmt.Fprintf(f.writer, "%s✓%s %s\n", f.colors.Green, f.colors.Reset, msg)
}

// Warning prints a warning message.
func (f *OutputFormatter) Warning(msg string) {
	if f.quiet {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	_, _ = fmt.Fprintf(f.writer, "%s⚠%s %s\n", f.colors.Yellow, f.colors.Reset, msg)
}

// Error prints an error message.
func (f *OutputFormatter) Error(msg string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, _ = fmt.Fprintf(f.writer, "%s✗%s %s\n", f.colors.Red, f.colors.Reset, msg)
}

// Errorf prints a formatted error message.
func (f *OutputFormatter) Errorf(format string, args ...interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, _ = fmt.Fprintf(f.writer, "%s✗%s ", f.colors.Red, f.colors.Reset)
	_, _ = fmt.Fprintf(f.writer, format, args...)
	_, _ = fmt.Fprintln(f.writer)
}

// Header prints a section header.
func (f *OutputFormatter) Header(msg string) {
	if f.quiet {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	_, _ = fmt.Fprintf(f.writer, "\n%s%s%s\n", f.colors.Bold, msg, f.colors.Reset)
	_, _ = fmt.Fprintf(f.writer, "%s%s%s\n", f.colors.Dim, strings.Repeat("─", len(msg)), f.colors.Reset)
}

// Table prints a formatted table.
func (f *OutputFormatter) Table(headers []string, rows [][]string) {
	if f.quiet {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(headers) == 0 || len(rows) == 0 {
		return
	}

	colWidths := make([]int, len(headers))
	for i, h := range headers {
		colWidths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(colWidths) && len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	// Print header
	for i, h := range headers {
		_, _ = fmt.Fprintf(f.writer, "%s%-*s%s  ", f.colors.Bold, colWidths[i], h, f.colors.Reset)
	}
	_, _ = fmt.Fprintln(f.writer)

	// Print separator
	for _, w := range colWidths {
		_, _ = fmt.Fprintf(f.writer, "%s%s%s  ", f.colors.Dim, strings.Repeat("─", w), f.colors.Reset)
	}
	_, _ = fmt.Fprintln(f.writer)

	// Print rows
	for _, row := range rows {
		for i, cell := range row {
			if i < len(colWidths) {
				_, _ = fmt.Fprintf(f.writer, "%-*s  ", colWidths[i], cell)
			}
		}
		_, _ = fmt.Fprintln(f.writer)
	}
}

// KeyValue prints a key-value pair.
func (f *OutputFormatter) KeyValue(key, value string) {
	if f.quiet {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	_, _ = fmt.Fprintf(f.writer, "  %s%s%s: %s\n", f.colors.Cyan, key, f.colors.Reset, value)
}

// Bullet prints a bullet point.
func (f *OutputFormatter) Bullet(msg string) {
	if f.quiet {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	_, _ = fmt.Fprintf(f.writer, "  • %s\n", msg)
}

// Tree prints a tree structure.
func (f *OutputFormatter) Tree(items []TreeItem, indent string) {
	if f.quiet {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, item := range items {
		_, _ = fmt.Fprintf(f.writer, "%s%s%s%s\n", indent, item.Prefix, f.colors.Bold, item.Label)
		if item.Detail != "" {
			_, _ = fmt.Fprintf(f.writer, "%s  %s%s\n", indent, f.colors.Dim, item.Detail)
		}
		if len(item.Children) > 0 {
			f.printTreeChildren(item.Children, indent+"  ")
		}
	}
}

func (f *OutputFormatter) printTreeChildren(items []TreeItem, indent string) {
	for _, item := range items {
		_, _ = fmt.Fprintf(f.writer, "%s%s%s%s\n", indent, item.Prefix, f.colors.Bold, item.Label)
		if item.Detail != "" {
			_, _ = fmt.Fprintf(f.writer, "%s  %s%s\n", indent, f.colors.Dim, item.Detail)
		}
		if len(item.Children) > 0 {
			f.printTreeChildren(item.Children, indent+"  ")
		}
	}
}

// ProgressBar creates a new progress bar for long operations.
func (f *OutputFormatter) ProgressBar(total int, description string) *ProgressBar {
	return NewProgressBar(f.writer, total, description, f.noColor)
}

// Spinner creates a new spinner for async operations.
// The spinner writes to the same synchronized writer as the formatter,
// so concurrent writes from the spinner goroutine and formatter methods
// are serialized to prevent data races.
func (f *OutputFormatter) Spinner(description string) *Spinner {
	return NewSpinner(f.writer, description, f.noColor)
}

// printJSON marshals and writes JSON output.
func (f *OutputFormatter) printJSON(v interface{}) error {
	encoder := json.NewEncoder(f.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}

// printYAML marshals and writes YAML output.
func (f *OutputFormatter) printYAML(v interface{}) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	_, err = f.writer.Write(data)
	return err
}

// printText writes text representation.
func (f *OutputFormatter) printText(v interface{}) error {
	switch val := v.(type) {
	case string:
		_, _ = fmt.Fprintln(f.writer, val)
	default:
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(f.writer, string(data))
	}
	return nil
}

// TreeItem represents an item in a tree structure.
type TreeItem struct {
	Label    string
	Detail   string
	Prefix   string
	Children []TreeItem
}

// ProgressBar displays a progress bar for long operations.
type ProgressBar struct {
	writer      io.Writer
	total       int
	current     int
	description string
	noColor     bool
	width       int
	mu          sync.Mutex
	started     bool
	startTime   time.Time
}

// NewProgressBar creates a new ProgressBar.
func NewProgressBar(writer io.Writer, total int, description string, noColor bool) *ProgressBar {
	return &ProgressBar{
		writer:      writer,
		total:       total,
		description: description,
		noColor:     noColor,
		width:       40,
	}
}

// Increment advances the progress bar by one step.
func (p *ProgressBar) Increment() {
	p.Add(1)
}

// Add advances the progress bar by n steps.
func (p *ProgressBar) Add(n int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		p.started = true
		p.startTime = time.Now()
	}

	p.current += n
	if p.current > p.total {
		p.current = p.total
	}
	p.render()
}

// Complete finishes the progress bar.
func (p *ProgressBar) Complete() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.current = p.total
	p.render()
	elapsed := time.Since(p.startTime)
	_, _ = fmt.Fprintf(p.writer, "\n")
	if !p.noColor {
		_, _ = fmt.Fprintf(p.writer, "\033[32m✓\033[0m %s completed in %s\n", p.description, elapsed.Round(time.Millisecond))
	} else {
		_, _ = fmt.Fprintf(p.writer, "✓ %s completed in %s\n", p.description, elapsed.Round(time.Millisecond))
	}
}

func (p *ProgressBar) render() {
	percent := float64(p.current) / float64(p.total) * 100
	filled := int(float64(p.width) * float64(p.current) / float64(p.total))

	bar := strings.Builder{}
	bar.WriteString("[")
	for i := 0; i < p.width; i++ {
		if i < filled {
			if !p.noColor {
				bar.WriteString("\033[32m█\033[0m")
			} else {
				bar.WriteString("█")
			}
		} else {
			bar.WriteString("░")
		}
	}
	bar.WriteString("]")

	_, _ = fmt.Fprintf(p.writer, "\r%s %s %d/%d (%.0f%%)",
		p.description, bar.String(), p.current, p.total, percent)
}

// Spinner displays a spinner for async operations.
type Spinner struct {
	writer      io.Writer
	description string
	noColor     bool
	done        chan struct{}
	mu          sync.Mutex
	running     bool
}

// NewSpinner creates a new Spinner.
func NewSpinner(writer io.Writer, description string, noColor bool) *Spinner {
	return &Spinner{
		writer:      writer,
		description: description,
		noColor:     noColor,
		done:        make(chan struct{}),
	}
}

// Start begins the spinner animation.
func (s *Spinner) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0

	go func() {
		for {
			select {
			case <-s.done:
				return
			default:
				s.mu.Lock()
				color := ""
				reset := ""
				if !s.noColor {
					color = "\033[36m"
					reset = "\033[0m"
				}
				_, _ = fmt.Fprintf(s.writer, "\r%s%s%s %s", color, frames[i%len(frames)], reset, s.description)
				telemetry.Emit("spinner_tick", map[string]interface{}{
					"description": s.description,
				})
				s.mu.Unlock()
				i++
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
}

// Stop stops the spinner with a final message.
func (s *Spinner) Stop(finalMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		close(s.done)
		s.running = false
		color := ""
		reset := ""
		if !s.noColor {
			color = "\033[32m"
			reset = "\033[0m"
		}
		_, _ = fmt.Fprintf(s.writer, "\r%s%s%s %s\n", color, "✓", reset, finalMsg)
	}
}

// Fail stops the spinner with a failure message.
func (s *Spinner) Fail(finalMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		close(s.done)
		s.running = false
		color := ""
		reset := ""
		if !s.noColor {
			color = "\033[31m"
			reset = "\033[0m"
		}
		_, _ = fmt.Fprintf(s.writer, "\r%s%s%s %s\n", color, "✗", reset, finalMsg)
	}
}

// FormatError formats an error for user-friendly display.
func FormatError(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%s%s%s", DefaultColorScheme().Red, err, DefaultColorScheme().Reset)
}

// FormatSuccess formats a success message.
func FormatSuccess(msg string) string {
	return fmt.Sprintf("%s✓%s %s", DefaultColorScheme().Green, DefaultColorScheme().Reset, msg)
}

// FormatWarning formats a warning message.
func FormatWarning(msg string) string {
	return fmt.Sprintf("%s⚠%s %s", DefaultColorScheme().Yellow, DefaultColorScheme().Reset, msg)
}

// FormatBold formats text in bold.
func FormatBold(msg string) string {
	return fmt.Sprintf("%s%s%s", DefaultColorScheme().Bold, msg, DefaultColorScheme().Reset)
}
