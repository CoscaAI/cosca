# cosca-cli — Reusable Patterns

## Pattern: CLI Command Audit Pipeline

**Discovered**: 2026-07-28  
**Context**: Full audit of Cobra-based CLI with 37 top-level + 105 leaf commands

**Pipeline**:
1. **Explore structure**: Read `cmd/` + glob for `.go` files → identify all command constructors
2. **Read root registration**: `root.go` → `AddCommand()` calls → confirm all 37 top-level
3. **Build**: `go build -o binary ./cmd/X/` → verify compilation
4. **Help test**: `./binary <cmd> --help` for each top-level + subcommand → verify help text, flags, args
5. **Deep read**: Selected command files + adapter files → classify FULL vs STUB vs ERROR
6. **Run test**: `go test ./.../ -short` → verify test suite passes
7. **UX analysis**: Check aliases, flag consistency, color usage, completion, --help quality
8. **Gap analysis**: Identify missing commands, compare with peer CLIs
9. **Report**: Structured markdown with inventory, issues, recommendations

**When to use**: Any Cobra CLI audit task. Scales to any Go CLI codebase.

**Key signals**: 
- `NewXxxCommand() *cobra.Command` functions
- `cmd.AddCommand()` registration pattern
- Adapter files bridging CLI → internal packages
- `RunE: func(cmd *cobra.Command, args []string) error` for implementation depth
