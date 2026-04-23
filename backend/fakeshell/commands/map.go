package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var dangerousPatterns = []string{
	";", "|", "&", "`", "$(", "${",
	">", "<", ">>", "<<",
	"\n", "\r",
}

func isDangerousArg(arg string) bool {
	for _, pattern := range dangerousPatterns {
		if strings.Contains(arg, pattern) {
			return true
		}
	}
	return false
}

func AddExecutableAsCommand(cmd *cobra.Command, command string, argFilter func(args []string) ([]string, error)) {

	cmdDefine := &cobra.Command{
		Use: command,
		Run: func(cmd *cobra.Command, args []string) {
			if command == "" || filepath.Base(command) != command {
				cmd.Println("invalid command")
				return
			}

			for _, arg := range args {
				if isDangerousArg(arg) {
					cmd.Println("invalid argument detected")
					return
				}
			}

			args, err := argFilter(args)
			if err != nil {
				cmd.Println(err)
				return
			}

			c := exec.CommandContext(cmd.Context(), command, args...)
			c.Env = os.Environ()
			c.Env = append(c.Env, "TERM=xterm-256color")
			c.Stdin = cmd.InOrStdin()
			c.Stdout = cmd.OutOrStdout()
			c.Stderr = cmd.ErrOrStderr()

			if err := c.Run(); err != nil {
				cmd.Println(err)
			}
		},
	}

	cmd.AddCommand(cmdDefine)
}
