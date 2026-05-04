// Package completion provides `ovh completion <shell>`. Mirrors `gh completion`.
package completion

import "github.com/spf13/cobra"

// NewCmd returns the completion subcommand. Generates shell-specific completion
// for bash, zsh, fish, or powershell on stdout. See PRD-01 §Command taxonomy.
func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:                   "completion <bash|zsh|fish|powershell>",
		Short:                 "Generate shell completion script",
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			out := cmd.OutOrStdout()
			switch args[0] {
			case "bash":
				return root.GenBashCompletionV2(out, true)
			case "zsh":
				return root.GenZshCompletion(out)
			case "fish":
				return root.GenFishCompletion(out, true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(out)
			}
			return nil // unreachable; OnlyValidArgs guards entry
		},
	}
}
