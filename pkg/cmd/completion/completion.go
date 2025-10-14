package completion

import (
	"os"

	"github.com/spf13/cobra"
)

func NewCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for Mirako CLI.

The completion script for each shell will be generated to stdout.
You can source it in your shell's configuration file.

### Bash

To load completions in your current shell session:

	source <(mirako completion bash)

To load completions for every new session, execute once:

Linux:

	mirako completion bash > /etc/bash_completion.d/mirako

macOS (via Homebrew):

	mirako completion bash > $(brew --prefix)/etc/bash_completion.d/mirako

Note: If installed via Homebrew, completions are automatically installed.
See https://docs.brew.sh/Shell-Completion for more information.

### Zsh

If shell completion is not already enabled in your environment, enable it by executing once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions for every new session, execute once:

	mirako completion zsh > "${fpath[1]}/_mirako"

You will need to start a new shell for this setup to take effect.

Note: If installed via Homebrew, completions are automatically installed.
See https://docs.brew.sh/Shell-Completion for more information.

### Fish

To load completions in your current shell session:

	mirako completion fish | source

To load completions for every new session, execute once:

	mirako completion fish > ~/.config/fish/completions/mirako.fish

### PowerShell

To load completions in your current shell session:

	mirako completion powershell | Out-String | Invoke-Expression

To load completions for every new session, run:

	mirako completion powershell >> $PROFILE

For more information, visit: https://mirako.co
`,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletionV2(os.Stdout, true)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			}
			return nil
		},
	}

	return cmd
}
