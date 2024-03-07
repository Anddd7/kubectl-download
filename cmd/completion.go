package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate completion script",
	Long: `To load completions:

Bash:
	$ source <(kubectl-download completion bash)

	# To load completions for each session, execute once:
	Linux:
	$ kubectl-download completion bash > /etc/bash_completion.d/kubectl-download
	MacOS:
	$ kubectl-download completion bash > /usr/local/etc/bash_completion.d/kubectl-download

Zsh:
	# If shell completion is not already enabled in your environment you will need
	# to enable it.  You can execute the following once:

	$ echo "autoload -U compinit; compinit" >> ~/.zshrc

	# To load completions for each session, execute once:
	$ kubectl-download completion zsh > "${fpath[1]}/_kubectl-download"

	# You will need to start a new shell for this setup to take effect.

Fish:
	$ kubectl-download completion fish | source

	# To load completions for each session, execute once:
	$ kubectl-download completion fish > ~/.config/fish/completions/kubectl-download.fish

Powershell:
	PS> kubectl-download completion powershell | Out-String | Invoke-Expression

	# To load completions for every new session, run:
	PS> kubectl-download completion powershell > kubectl-download.ps1
	
	# and source this file from your powershell profile...
`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
			os.Exit(1)
		}

		switch args[0] {
		case "bash":
			cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			cmd.Root().GenPowerShellCompletion(os.Stdout)
		}
	},
}
