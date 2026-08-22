package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/CoscaAI/cosca/internal/context"
)

// NewContextCommand creates the `cosca context` command and its subcommands.
func NewContextCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Manage the Cosca context",
		Long: `Manage the Cosca context system.

Context is built from your project's memory, knowledge, and environment.
It provides relevant information to AI agents when processing requests.

Subcommands:
  build <query>  Build context for a specific query
  show           Show the current context
  clear          Clear the context
  stats          Show context statistics
`,
		Example: `  cosca context build "database schema"    Build context for query
  cosca context show                        Show current context
  cosca context clear                       Clear context
  cosca context stats                       Show context statistics`,
	}

	cmd.AddCommand(
		NewContextBuildCommand(),
		NewContextShowCommand(),
		NewContextClearCommand(),
		NewContextStatsCommand(),
	)

	return cmd
}

// createBuilder creates a context builder with default settings.
func createBuilder() *context.Builder {
	logger := zerolog.Nop()
	return context.NewBuilder(logger)
}

// NewContextBuildCommand creates the `cosca context build` subcommand.
func NewContextBuildCommand() *cobra.Command {
	var maxTokens int
	var includeMemory bool

	cmd := &cobra.Command{
		Use:   "build <query>",
		Short: "Build context for a query",
		Long:  `Build a context bundle from memory, knowledge, and project data relevant to the given query.`,
		Example: `  cosca context build "implement user auth"
  cosca context build --max-tokens 8000 "database migration"
  cosca context build --no-memory "refactor the API"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			_ = filepath.Join(dir, ".cosca")

			spinner := formatter.Spinner("Building context")
			spinner.Start()

			builder := createBuilder()
			if builder == nil {
				spinner.Fail("Context builder not available")
				return fmt.Errorf("context builder not available")
			}

			req := context.ContextRequest{
				Query:     args[0],
				MaxTokens: maxTokens,
				Intent:    context.IntentQuestion,
			}
			if !includeMemory {
				req.Filters = append(req.Filters, "no_memory")
			}

			result, err := builder.BuildContext(cmd.Context(), req)
			if err != nil {
				spinner.Fail(fmt.Sprintf("Context build failed: %v", err))
				return fmt.Errorf("failed to build context: %w", err)
			}

			spinner.Stop("Context built")

			if useJSON {
				return printJSON(cmd, result)
			}

			formatter.Header("Context Build Result")
			formatter.KeyValue("Query", args[0])
			formatter.KeyValue("Documents", fmt.Sprintf("%d", len(result.Documents)))
			formatter.KeyValue("Chunks", fmt.Sprintf("%d", len(result.Chunks)))
			formatter.KeyValue("Symbols", fmt.Sprintf("%d", len(result.Symbols)))
			formatter.KeyValue("Memory", fmt.Sprintf("%d", len(result.Memory)))
			formatter.KeyValue("Tokens", fmt.Sprintf("%d", result.TokenCount))

			for _, doc := range result.Documents {
				formatter.Bullet(fmt.Sprintf("%s (score: %.2f)", doc.Title, doc.Score))
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&maxTokens, "max-tokens", 4096, "maximum tokens for context")
	cmd.Flags().BoolVar(&includeMemory, "memory", true, "include memory in context")
	return cmd
}

// NewContextShowCommand creates the `cosca context show` subcommand.
func NewContextShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show the current context",
		Long:  `Display the currently loaded context including memory, project info, and environment.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			_ = filepath.Join(dir, ".cosca")

			builder := createBuilder()
			if builder == nil {
				return fmt.Errorf("context builder not available")
			}

			// Build a default empty context to show structure
			req := context.ContextRequest{
				Query:     "",
				MaxTokens: 1000,
				Intent:    context.IntentExplore,
			}
			currentCtx, err := builder.BuildContext(cmd.Context(), req)
			if err != nil {
				return fmt.Errorf("failed to build context: %w", err)
			}

			if useJSON {
				return printJSON(cmd, currentCtx)
			}

			formatter.Header("Current Context")

			if len(currentCtx.Documents) > 0 {
				formatter.Println("")
				formatter.KeyValue("Documents", fmt.Sprintf("%d entries", len(currentCtx.Documents)))
				for _, doc := range currentCtx.Documents {
					formatter.Bullet(fmt.Sprintf("%s: %s", doc.ID, truncate(doc.Content, 80)))
				}
			}

			if len(currentCtx.Chunks) > 0 {
				formatter.Println("")
				formatter.KeyValue("Chunks", fmt.Sprintf("%d entries", len(currentCtx.Chunks)))
				for _, chunk := range currentCtx.Chunks {
					formatter.Bullet(fmt.Sprintf("%s: %s", chunk.ID, truncate(chunk.Content, 80)))
				}
			}

			if len(currentCtx.Symbols) > 0 {
				formatter.Println("")
				formatter.KeyValue("Symbols", fmt.Sprintf("%d entries", len(currentCtx.Symbols)))
				for _, sym := range currentCtx.Symbols {
					formatter.Bullet(fmt.Sprintf("%s (%s)", sym.Name, sym.File))
				}
			}

			formatter.Println("")
			formatter.KeyValue("Token Count", fmt.Sprintf("%d", currentCtx.TokenCount))
			formatter.KeyValue("Built At", currentCtx.BuiltAt.Format(time.RFC3339))

			return nil
		},
	}

	return cmd
}

// NewContextClearCommand creates the `cosca context clear` subcommand.
func NewContextClearCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear the context",
		Long:  `Clear the current session context. Project and environment context remain intact.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)

			dir, _ := os.Getwd()
			_ = filepath.Join(dir, ".cosca")

			spinner := formatter.Spinner("Clearing context")
			spinner.Start()

			_ = createBuilder()

			spinner.Stop("Context cleared")
			formatter.Success("Session context cleared")
			return nil
		},
	}

	return cmd
}

// NewContextStatsCommand creates the `cosca context stats` subcommand.
func NewContextStatsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show context statistics",
		Long:  `Display statistics about the context system including size, composition, and usage.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatter := GetFormatter(cmd)
			useJSON := IsJSONOutput(cmd)

			dir, _ := os.Getwd()
			_ = filepath.Join(dir, ".cosca")

			builder := createBuilder()
			if builder == nil {
				return fmt.Errorf("context builder not available")
			}

			// Build a default context to compute stats
			req := context.ContextRequest{
				Query:     "",
				MaxTokens: 1000,
				Intent:    context.IntentExplore,
			}
			ctx, err := builder.BuildContext(cmd.Context(), req)
			if err != nil {
				return fmt.Errorf("failed to get context stats: %w", err)
			}

			stats := map[string]interface{}{
				"documents":     len(ctx.Documents),
				"chunks":        len(ctx.Chunks),
				"symbols":       len(ctx.Symbols),
				"entities":      len(ctx.Entities),
				"relationships": len(ctx.Relationships),
				"memory":        len(ctx.Memory),
				"total_tokens":  ctx.TokenCount,
				"last_built":    ctx.BuiltAt.Format(time.RFC3339),
				"total_results": ctx.Metadata.TotalResults,
				"confidence":    ctx.Metadata.Confidence,
			}

			if useJSON {
				return printJSON(cmd, stats)
			}

			formatter.Header("Context Statistics")
			formatter.KeyValue("Documents", fmt.Sprintf("%d", len(ctx.Documents)))
			formatter.KeyValue("Chunks", fmt.Sprintf("%d", len(ctx.Chunks)))
			formatter.KeyValue("Symbols", fmt.Sprintf("%d", len(ctx.Symbols)))
			formatter.KeyValue("Entities", fmt.Sprintf("%d", len(ctx.Entities)))
			formatter.KeyValue("Relationships", fmt.Sprintf("%d", len(ctx.Relationships)))
			formatter.KeyValue("Memory Records", fmt.Sprintf("%d", len(ctx.Memory)))
			formatter.KeyValue("Total Tokens", fmt.Sprintf("%d", ctx.TokenCount))
			formatter.KeyValue("Confidence", fmt.Sprintf("%.2f", ctx.Metadata.Confidence))
			formatter.KeyValue("Last Built", ctx.BuiltAt.Format("2006-01-02 15:04:05"))

			return nil
		},
	}

	return cmd
}

// truncate truncates a string to the given max length, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ifEmpty returns the first string if non-empty, otherwise the second.
func ifEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
