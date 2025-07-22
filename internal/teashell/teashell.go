package teashell

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/sector-f/ssh-cli-forum/forum"
)

var _ tea.Model = model{}

var (
	titleStyle = func() lipgloss.Style {
		b := lipgloss.NormalBorder()
		return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1)
	}()

	infoStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Left = "┤"
		return titleStyle.BorderStyle(b)
	}()
)

type screen string

// TODO: Can these just be ints?
var (
	screenHome    screen = "home"
	screenChat    screen = "chatroom"
	screenBoards  screen = "boards"
	screenThreads screen = "threads"
)

type model struct {
	session      ssh.Session
	user         string
	boardService forum.BoardService

	screen  screen
	screens map[screen]tea.Model

	width int

	/*
		boards       []forum.BoardListing
		currentBoard forum.Board

		chat      forum.ChatroomService
		msgs      []forum.Message
		callback  func(forum.Message)
		connected bool

		viewport   viewport.Model
		input      textinput.Model
		buffer     string
		headerText string

		ready bool
	*/
}

func NewProgram(sess ssh.Session, boardService forum.BoardService, chat forum.ChatroomService) *tea.Program {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Focus()

	// id := sess.Context().SessionID()

	/*
		var program *tea.Program
		callback := func(msg forum.Message) {
			program.Send(msg)
		}
	*/

	m := model{
		session: sess,
		screen:  screenHome,
		user:    sess.User(),
		// chat:       chat,
		// input:      ti,
		// headerText: "SSH Chatroom",
		// callback:   callback,
	}

	program := tea.NewProgram(m, tea.WithAltScreen(), tea.WithInput(sess), tea.WithOutput(sess))

	/*
		go func() {
			program.Wait()
			chat.DisconnectUser(id)
		}()
	*/

	return program
}

func Handler(boardService forum.BoardService, chat forum.ChatroomService) bubbletea.ProgramHandler {
	return func(s ssh.Session) *tea.Program {
		return NewProgram(s, boardService, chat)
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

/*
func (m model) chatBuffer() string {
	var sb strings.Builder

	// '12:34:56 '
	prefixWidth := 9

	for _, msg := range m.msgs {
		timeStamp := msg.Time().Format(time.TimeOnly)
		wrappedMsg := wordwrap.String(msg.Body(), m.width-prefixWidth)

		sb.WriteString(timeStamp + " ")

		var lineNum int
		scanner := bufio.NewScanner(strings.NewReader(wrappedMsg))
		for scanner.Scan() {
			if lineNum > 0 {
				sb.WriteString(strings.Repeat(" ", prefixWidth))
			}
			sb.WriteString(scanner.Text() + "\n")
			lineNum++
		}
	}

	return sb.String()
}
*/

/*
func (m model) threadsBuffer() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "%s\n\n", m.currentBoard.Name)
	for i, thread := range m.currentBoard.Threads {
		numWidth := lipgloss.Width(strconv.Itoa(i))
		spaces := strings.Repeat(" ", numWidth)

		fmt.Fprintf(&sb, "%d. %s\n", i+1, thread.Title)
		fmt.Fprintf(&sb, "%s  Posted by %s\n", spaces, thread.Author.Username)

		if i < len(m.currentBoard.Threads)-1 {
			continue
		}

		fmt.Fprintf(&sb, "\n")
	}

	return sb.String()
}
*/

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd tea.Cmd
		// cmds []tea.Cmd
	)

	currentModel := m.screens[m.screen]
	_, cmd = currentModel.Update(msg)
	return m, cmd
}

/*
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case forum.Message:
		m.msgs = append(m.msgs, msg)
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			switch m.screen {
			case screenBoards:
				m.screen = screenHome
			case screenChat:
				m.screen = screenHome
			default:
				// Do nothing
			}
		case "ctrl+c":
			switch m.screen {
			case screenBoards:
				m.screen = screenHome
			case screenChat:
				m.screen = screenHome
			default:
				return m, tea.Quit
			}
		case "enter":
			command := strings.TrimSpace(m.input.Value())
			m.input.Reset()

			if command == "" {
				break
			}

			switch m.screen {
			case screenHome:
				// TODO: is there a way to automate this?
				switch command {
				case "chat":
					if !m.connected {
						msgs := m.chat.ConnectUser(m.session, m.callback)
						m.msgs = msgs
						m.connected = true
					}

					m.screen = screenChat
				case "boards":
					b, err := m.boardService.ListBoards()
					if err != nil {
						// TODO: actual error handling
						break
					}
					m.boards = b

					m.screen = screenBoards
				case "exit", "quit":
					return m, tea.Quit
				default:
					// Display error message, somehow
				}
			case screenBoards:
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

					bl := m.boards[index-1]
					id := bl.ID
					board, err := m.boardService.GetBoardByID(id)
					if err != nil {
						break
					}

					m.currentBoard = board
					m.screen = screenThreads
				}
			case screenChat:
				chatMsg := forum.ChatMsg{
					Timestamp: time.Now(),
					Username:  m.user,
					Text:      command,
				}

				cmds = append(cmds, func() tea.Msg { m.chat.SendMsg(chatMsg); return nil })
			}
		}
	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.headerView())
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

	switch m.screen {
	case screenHome:
		m.viewport.SetContent(m.homeBuffer())
	case screenChat:
		m.viewport.SetContent(m.chatBuffer())
	case screenBoards:
		m.viewport.SetContent(m.boardsBuffer())
	case screenThreads:
		m.viewport.SetContent(m.threadsBuffer())
	}

	// Handle keyboard and mouse events in the viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}
*/

func (m model) View() string {
	currentModel := m.screens[m.screen]
	return currentModel.View()
}

/*
func (m model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	return fmt.Sprintf(
		"%s\n%s\n%s\n%s",
		headerView(m.width),
		m.viewport.View(),
		strings.Repeat("─", max(0, m.viewport.Width)),
		m.input.View(),
	)
}
*/

func headerView(text string, width int) string {
	title := titleStyle.Render(text)
	line := strings.Repeat(
		"─",
		max(0, (width-lipgloss.Width(title))/2),
	)
	return lipgloss.JoinHorizontal(lipgloss.Center, line, title, line)
}
