// Package cli provides CLI adapters that bridge the gap between what command
// files expect and what the actual internal packages provide.
//
// Each adapter wraps a real internal type and adds/renames methods to match
// the signatures used by CLI command files.
package cli

// =============================================================================
