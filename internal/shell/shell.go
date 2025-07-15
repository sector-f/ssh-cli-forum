package shell

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/chzyer/readline"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

type shell struct{}

func Middleware(logger *zap.Logger) wish.Middleware {
	return func(next ssh.Handler) ssh.Handler {
		return func(sess ssh.Session) {
			rlConf := &readline.Config{
				Prompt:      "> ",
				HistoryFile: "",

				Stdin:  sess,
				Stdout: sess,
				Stderr: sess.Stderr(),
			}

			rl, err := readline.NewEx(rlConf)
			if err != nil {
				logger.Error("Could not create readline", zap.Error(err))
				sess.Exit(1)
				return
			}

			for {
				line, err := rl.Readline()
				if err != nil {
					fmt.Fprintf(sess, "Error getting line: %v\n", err)
					sess.Exit(1)
					return
				}

				line = strings.TrimSpace(line)
				args := strings.Split(line, " ")

				if len(args) == 0 {
					continue
				}

				cmd := rootCommand()
				// cmd.SetIn(sess)
				cmd.SetOut(sess)
				cmd.SetErr(sess)
				cmd.SetArgs(args)

				switch args[0] {
				case "exit", "quit":
					sess.Exit(0)
					return
				default:
					cmd.Execute()
				}
			}
		}
	}
}

func rootCommand() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.AddCommand(echoCommand())
	return cmd
}

func echoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "echo",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prefix, err := cmd.Flags().GetString("prefix")
			if err != nil {
				return err
			}

			line := args[0]

			fmt.Fprintf(cmd.OutOrStdout(), "%s%s\n", prefix, line)

			return nil
		},
	}
	cmd.PersistentFlags().String("prefix", "Got line: ", "Set prefix")

	return cmd
}
