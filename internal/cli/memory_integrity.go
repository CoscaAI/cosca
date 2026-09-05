package cli

import (
	"fmt"
	"path/filepath"

	"github.com/CoscaAI/cosca/internal/memoryintegrity"
	"github.com/spf13/cobra"
)

// NewMemoryIntegrityCommand exposes an advisory, offline integrity check. It
// never runs as part of runtime startup and never writes to the memory store.
func NewMemoryIntegrityCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "integrity",
		Short: "Verify protected memory/governance files",
		Long:  "Create or verify a reviewable SHA-256 manifest for protected memory and governance files.\n\nThis is advisory only: a mismatch is reported and returns a non-zero status, but it never blocks or changes the runtime.",
	}
	cmd.AddCommand(newMemoryIntegrityInitCommand(), newMemoryIntegrityVerifyCommand())
	return cmd
}

func newMemoryIntegrityInitCommand() *cobra.Command {
	var root, manifest string
	var force bool
	cmd := &cobra.Command{Use: "init", Short: "Create an integrity manifest", Long: "Create an integrity manifest. Existing manifests are preserved unless --force is explicitly provided.", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := integrityRoot(root)
			if err != nil {
				return err
			}
			write := memoryintegrity.Write
			if force {
				write = memoryintegrity.WriteWithForce
			}
			m, err := write(root, manifest)
			if err != nil {
				return err
			}
			if IsJSONOutput(cmd) {
				return printJSON(cmd, m)
			}
			GetFormatter(cmd).Success(fmt.Sprintf("Integrity manifest written (%d protected files)", len(m.Files)))
			GetFormatter(cmd).KeyValue("Manifest", manifestPath(root, manifest))
			return nil
		}}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: detected project root)")
	cmd.Flags().StringVar(&manifest, "manifest", "", "manifest path (default: .cosca/audit/memory-integrity-manifest.json)")
	cmd.Flags().BoolVar(&force, "force", false, "replace an existing manifest")
	return cmd
}

func newMemoryIntegrityVerifyCommand() *cobra.Command {
	var root, manifest string
	cmd := &cobra.Command{Use: "verify", Short: "Check the current files against the manifest", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := integrityRoot(root)
			if err != nil {
				return err
			}
			result, err := memoryintegrity.Verify(root, manifest)
			if err != nil {
				return err
			}
			if IsJSONOutput(cmd) {
				if err := printJSON(cmd, result); err != nil {
					return err
				}
			} else {
				f := GetFormatter(cmd)
				f.Header("Memory Integrity (advisory)")
				f.KeyValue("Manifest", manifestPath(root, manifest))
				f.KeyValue("Status", integrityStatus(result))
				for _, p := range result.Changed {
					f.Warning("Changed: " + p)
				}
				for _, p := range result.Missing {
					f.Warning("Missing: " + p)
				}
				for _, p := range result.Added {
					f.Warning("Added: " + p)
				}
			}
			if len(result.Changed)+len(result.Missing)+len(result.Added) > 0 {
				return fmt.Errorf("protected memory files differ from manifest")
			}
			return nil
		}}
	cmd.Flags().StringVar(&root, "root", "", "project root (default: detected project root)")
	cmd.Flags().StringVar(&manifest, "manifest", "", "manifest path (default: .cosca/audit/memory-integrity-manifest.json)")
	return cmd
}

func integrityRoot(root string) (string, error) {
	if root != "" && root != "." {
		return root, nil
	}
	return memoryintegrity.ProjectRoot(".")
}

func manifestPath(root, manifest string) string {
	if manifest != "" {
		if p, err := filepath.Abs(manifest); err == nil {
			return p
		}
	}
	p, _ := filepath.Abs(filepath.Join(root, memoryintegrity.DefaultManifest))
	return p
}
func integrityStatus(r memoryintegrity.Result) string {
	if len(r.Changed)+len(r.Missing)+len(r.Added) == 0 {
		return "OK"
	}
	return "MISMATCH (advisory)"
}
