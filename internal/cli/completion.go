package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// NewCompletionCommand creates the `cosca completion` command for shell completion.
func NewCompletionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for various shells.

This command generates auto-completion scripts for bash, zsh, fish, and PowerShell shells.
To use the completions, you need to source the output in your shell profile.

Examples:
  # Bash
  $ cosca completion bash > /etc/bash_completion.d/cosca
  # or
  $ source <(cosca completion bash)

  # Zsh
  $ cosca completion zsh > "${fpath[1]}/_cosca"
  # or
  $ source <(cosca completion zsh)

  # Fish
  $ cosca completion fish > ~/.config/fish/completions/cosca.fish

  # PowerShell
  $ cosca completion powershell > cosca.ps1
  $ . .\cosca.ps1
`,
		Example: `  cosca completion bash > /usr/local/etc/bash_completion.d/cosca
  cosca completion zsh > /usr/local/share/zsh/site-functions/_cosca
  cosca completion fish > ~/.config/fish/completions/cosca.fish
  cosca completion powershell > cosca.ps1`,
		Args: cobra.ExactArgs(1),
		ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return []string{"bash", "zsh", "fish", "powershell"}, cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			shell := args[0]
			var err error

			switch shell {
			case "bash":
				err = cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				err = cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				err = cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				err = cmd.Root().GenPowerShellCompletion(os.Stdout)
			default:
				return fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish, powershell)", shell)
			}

			if err != nil {
				return fmt.Errorf("failed to generate completion for %s: %w", shell, err)
			}

			return nil
		},
	}

	return cmd
}
