package shell

import (
	shell "github.com/DomBlack/bubble-shell"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/sector-f/ssh-cli-forum/forum"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var _ tea.Model = model{}

type model struct {
	shell tea.Model
}

func New(logger *zap.Logger, s ssh.Session, service forum.Service) model {
	rootCmd := &cobra.Command{}

	// Create a command to add to the root command
	myCmd := &cobra.Command{
		Use: "hi",
		RunE: func(cmd *cobra.Command, args []string) error {
			s.Write([]byte("Hello World\n"))
			return nil
		},
	}
	rootCmd.AddCommand(myCmd)

	shellModel := shell.New(
		rootCmd,
		shell.WithNoHistory(),
		shell.WithInlineShell(),
	)

	m := model{
		shell: shellModel,
	}

	return m
}

func (m model) Init() tea.Cmd {
	return m.shell.Init()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m.shell.Update(msg)
}

func (m model) View() string {
	return m.shell.View()
}
