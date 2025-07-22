package teashell

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sector-f/ssh-cli-forum/forum"
)

var _ tea.Model = boardListModel{}

type boardListModel struct {
	boardService forum.BoardService
	boards       []forum.BoardListing

	viewport viewport.Model
	input    textinput.Model
	buffer   string
	width    int
	ready    bool
	header   string
}

func newBoardList(service forum.BoardService) boardListModel {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Focus()

	b, err := service.ListBoards()
	if err != nil {
		// TODO: actual error handling
		return boardListModel{}
	}

	return boardListModel{
		input:  ti,
		boards: b,
		header: "Boards",
	}
}

func (m boardListModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m boardListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			// Return to home
		case "enter":
			command := strings.TrimSpace(m.input.Value())
			m.input.Reset()

			if command == "" {
				break
			}

			args := strings.Split(command, " ")
			if len(args) != 2 {
				// TODO: error message?
				break
			}

			// Use new tea.Cmd type for changing to new board/thread?
			// Can probably just get list of threads once at program start.
			switch args[0] {
			case "show":
				index, err := strconv.Atoi(args[1])
				if err != nil {
					break
				}

				if index <= 0 || index > len(m.boards) {
					break
				}

				/*

					bl := m.boards[index-1]
					id := bl.ID
					board, err := m.boardService.GetBoardByID(id)
					if err != nil {
						break
					}

					m.currentBoard = board
					m.screen = screenThreads
				*/
			}
		}
	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(headerView(m.header, msg.Width))
		verticalMarginHeight := headerHeight + lipgloss.Height(m.input.View()) + 1
		m.width = msg.Width
		m.input.Width = msg.Width - lipgloss.Width(m.input.Prompt) - 1

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight

			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}
	}

	// Handle keyboard and mouse events in the viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m boardListModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	return fmt.Sprintf(
		"%s\n%s\n%s\n%s",
		headerView(m.header, m.width),
		m.viewport.View(),
		strings.Repeat("─", max(0, m.viewport.Width)),
		m.input.View(),
	)
}

func (m boardListModel) boardsBuffer() string {
	var sb strings.Builder

	for i, board := range m.boards {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, board.Name)
	}

	return sb.String()
}
