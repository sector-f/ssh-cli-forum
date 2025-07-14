package table

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/evertras/bubble-table/table"
	"github.com/sector-f/ssh-cli-forum/forum"
	"go.uber.org/zap"
)

var _ tea.Model = model{}

type model struct {
	service forum.Service

	usersTable table.Model

	width  int
	height int
}

func New(logger *zap.Logger, service forum.Service) tea.Model {
	users, err := service.ListUsers()
	if err != nil {
		logger.Error("Failed to look up users", zap.Error(err))
		return model{}
	}

	var (
		columnKeyName  = "username"
		columnKeyAdmin = "isAdmin"
	)

	columns := []table.Column{
		table.NewColumn(columnKeyName, "Username", 15),
		table.NewColumn(columnKeyAdmin, "Is Admin", 8),
	}

	rows := []table.Row{}
	for _, user := range users {
		row := table.NewRow(table.RowData{
			columnKeyName:  user.Username,
			columnKeyAdmin: user.IsAdmin,
		})

		rows = append(rows, row)
	}

	t := table.New(columns).WithRows(rows)

	return model{
		service:    service,
		usersTable: t,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	m.usersTable, cmd = m.usersTable.Update(msg)

	cmds = append(cmds, cmd)
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			cmds = append(cmds, tea.Quit)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	return m.usersTable.View()
}
