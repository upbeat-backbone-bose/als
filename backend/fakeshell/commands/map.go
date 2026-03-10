package commands

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

func AddExecureableAsCommand(cmd *cobra.Command, command string, argFilter func(args []string) ([]string, error)) {

	cmdDefine := &cobra.Command{
		Use: command,
		Run: func(cmd *cobra.Command, args []string) {
			if command == "" || filepath.Base(command) != command {
				cmd.Println("invalid command")
				return
			}
			args, err := argFilter(args)
			if err != nil {
				cmd.Println(err)
				return
			}
			// command name comes from predefined menu registration, not user input
			c := exec.CommandContext(cmd.Context(), command, args...) // #nosec G204
			c.Env = os.Environ()
			c.Env = append(c.Env, "TERM=xterm-256color")
			c.Stdin = cmd.InOrStdin()
			c.Stdout = cmd.OutOrStdout()
			c.Stderr = cmd.OutOrStderr()

			if err := c.Run(); err != nil {
				cmd.Println(err)
			}
		},
		DisableFlagParsing: true,
	}

	cmd.AddCommand(cmdDefine)
}
