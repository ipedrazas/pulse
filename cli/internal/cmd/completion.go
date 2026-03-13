package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for bash, zsh, or fish.

To load completions:

  bash:
    source <(pulse completion bash)

    # To load completions for each session, execute once:
    # Linux:
    pulse completion bash > /etc/bash_completion.d/pulse
    # macOS:
    pulse completion bash > $(brew --prefix)/etc/bash_completion.d/pulse

  zsh:
    source <(pulse completion zsh)

    # To load completions for each session, execute once:
    pulse completion zsh > "${fpath[1]}/_pulse"

  fish:
    pulse completion fish | source

    # To load completions for each session, execute once:
    pulse completion fish > ~/.config/fish/completions/pulse.fish`,
		Example: `  pulse completion bash
  pulse completion zsh
  pulse completion fish`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish"},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletionV2(os.Stdout, true)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			default:
				return fmt.Errorf("unsupported shell %q: expected bash, zsh, or fish", args[0])
			}
		},
	}
	return cmd
}
