package tui

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/sector-f/ssh-cli-forum/internal/service"
)

type composeState int

const (
	composeNone composeState = iota
	composeNewTitle
	composeNewBody
	composeReplyBody
)

type sessionModel struct {
	session ssh.Session
	ctx     context.Context
	users   service.UserService
	boards  service.BoardService
	chat    service.ChatService
	user    service.User

	location location
	input  textinput.Model
	output []string

	commandHistory []string
	historyIndex   int
	historyDraft   string

	compose       composeState
	draftTitle    string
	draftLines    []string
	draftThreadID int

	chatActive     bool
	chatInput      textinput.Model
	chatViewport   viewport.Model
	chatMessages   []service.ChatMessage
	chatEvents     <-chan service.ChatMessage
	chatCancel     func()
	chatGeneration uint64
	chatStatus     string
	windowWidth    int
	windowHeight   int
}

var _ tea.Model = (*sessionModel)(nil)

func NewProgram(sess ssh.Session, userService service.UserService, boardService service.BoardService, chatServices ...service.ChatService) (*tea.Program, error) {
	user, err := userService.GetUserByUsername(sess.Context(), sess.User())
	if err != nil {
		return nil, fmt.Errorf("load account: %w", err)
	}
	if user.Status == service.StatusSuspended {
		return nil, service.ErrUserSuspended
	}
	input := textinput.New()
	input.Prompt = user.Username + "@community:/> "
	input.Focus()
	input.CharLimit = 4096
	input.Width = 100

	chatInput := textinput.New()
	chatInput.Prompt = "chat> "
	chatInput.CharLimit = service.ChatMessageMaxLength
	chatInput.Width = 100
	chatViewport := viewport.New(80, 20)
	chatViewport.MouseWheelEnabled = true

	var chatService service.ChatService
	if len(chatServices) > 0 {
		chatService = chatServices[0]
	}
	appSession := service.NewSession(user.ID, userService, boardService, chatService)
	if chatService != nil {
		chatService = appSession
	}
	m := &sessionModel{
		session: sess, ctx: sess.Context(), users: appSession, boards: appSession, chat: chatService, user: user,
		location: rootLocation{}, input: input, chatInput: chatInput, chatViewport: chatViewport,
		windowWidth: 80, windowHeight: 24,
		output: []string{
			"SSH Community",
			"-------------",
			"Welcome, " + user.Username + ".",
			"Type 'help' for available commands.",
			"",
			"Use 'ls' to list this location and 'cd' to navigate.",
		},
	}
	// Deliberately omit WithAltScreen: this is a command-oriented terminal interface.
	return tea.NewProgram(m, tea.WithInput(sess), tea.WithOutput(sess)), nil
}

func Handler(userService service.UserService, boardService service.BoardService, chatServices ...service.ChatService) bubbletea.ProgramHandler {
	return func(sess ssh.Session) *tea.Program {
		program, err := NewProgram(sess, userService, boardService, chatServices...)
		if err != nil {
			_, _ = fmt.Fprintf(sess, "Unable to start forum session: %v\r\n", err)
			return nil
		}
		return program
	}
}

func (m *sessionModel) Init() tea.Cmd { return textinput.Blink }

type chatEventMsg struct {
	generation uint64
	message    service.ChatMessage
	ok         bool
}

type chatHistoryMsg struct {
	generation uint64
	messages   []service.ChatMessage
	trigger    service.ChatMessage
	err        error
}

func waitForChatMessage(generation uint64, events <-chan service.ChatMessage) tea.Cmd {
	return func() tea.Msg {
		message, ok := <-events
		return chatEventMsg{generation: generation, message: message, ok: ok}
	}
}

func teaMsgCmd(message tea.Msg) tea.Cmd {
	return func() tea.Msg { return message }
}

func (m *sessionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowWidth, m.windowHeight = msg.Width, msg.Height
		width := msg.Width - len(m.prompt()) - 1
		if width < 20 {
			width = 20
		}
		m.input.Width = width
		if m.chatActive {
			m.resizeChat(msg.Width, msg.Height)
		}
		return m, nil

	case chatEventMsg:
		if !m.chatActive || msg.generation != m.chatGeneration {
			return m, nil
		}
		if !msg.ok {
			m.chatEvents = nil
			m.chatCancel = nil
			return m, nil
		}
		return m, tea.Batch(m.appendChatMessage(msg.message), waitForChatMessage(m.chatGeneration, m.chatEvents))

	case chatHistoryMsg:
		if !m.chatActive || msg.generation != m.chatGeneration {
			return m, nil
		}
		wasAtBottom := m.chatViewport.AtBottom()
		if msg.err != nil {
			m.chatStatus = "Missed messages; history reload failed."
			m.appendChatMessageNow(msg.trigger)
			return m, nil
		}
		m.chatMessages = msg.messages
		m.chatStatus = "Recovered missed messages from history."
		m.refreshChatViewport(wasAtBottom)
		return m, nil

	case tea.MouseMsg:
		if m.chatActive {
			var cmd tea.Cmd
			m.chatViewport, cmd = m.chatViewport.Update(msg)
			return m, cmd
		}

	case tea.KeyMsg:
		if m.chatActive {
			return m, m.updateChatKey(msg)
		}
		switch msg.String() {
		case "ctrl+c":
			if m.compose != composeNone {
				m.cancelComposition()
				return m, nil
			}
			return m, tea.Quit
		case "up":
			if m.compose == composeNone {
				m.previousCommand()
				return m, nil
			}
		case "down":
			if m.compose == composeNone {
				m.nextCommand()
				return m, nil
			}
		case "tab":
			if m.compose == composeNone {
				m.completeInput()
				return m, nil
			}
		case "enter":
			line := m.input.Value()
			if m.compose == composeNone {
				m.recordCommand(line)
			}
			m.input.Reset()
			if m.compose != composeNone {
				m.handleComposition(line)
			} else {
				line = strings.TrimSpace(line)
				if line != "" {
					m.addOutput(m.prompt() + line)
					quit, cmd := m.execute(line)
					if quit {
						return m, tea.Quit
					}
					m.input.Prompt = m.currentPrompt()
					return m, cmd
				}
				m.addOutput(m.prompt())
			}
			m.input.Prompt = m.currentPrompt()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *sessionModel) recordCommand(line string) {
	line = strings.TrimSpace(line)
	if line != "" && (len(m.commandHistory) == 0 || m.commandHistory[len(m.commandHistory)-1] != line) {
		m.commandHistory = append(m.commandHistory, line)
	}
	m.historyIndex = len(m.commandHistory)
	m.historyDraft = ""
}

func (m *sessionModel) previousCommand() {
	if len(m.commandHistory) == 0 {
		return
	}
	if m.historyIndex == len(m.commandHistory) {
		m.historyDraft = m.input.Value()
	}
	if m.historyIndex > 0 {
		m.historyIndex--
		m.input.SetValue(m.commandHistory[m.historyIndex])
		m.input.CursorEnd()
	}
}

func (m *sessionModel) nextCommand() {
	if m.historyIndex >= len(m.commandHistory) {
		return
	}
	m.historyIndex++
	if m.historyIndex == len(m.commandHistory) {
		m.input.SetValue(m.historyDraft)
	} else {
		m.input.SetValue(m.commandHistory[m.historyIndex])
	}
	m.input.CursorEnd()
}

func (m *sessionModel) updateChatKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		line := strings.TrimSpace(m.chatInput.Value())
		m.chatInput.Reset()
		return m.handleChatInput(line)
	case "pgup", "ctrl+u":
		m.chatViewport.ViewUp()
		return nil
	case "pgdown", "ctrl+d":
		m.chatViewport.ViewDown()
		return nil
	case "ctrl+c":
		m.chatInput.Reset()
		m.chatStatus = "Use /leave to return to the service."
		return nil
	}
	var cmd tea.Cmd
	m.chatInput, cmd = m.chatInput.Update(msg)
	return cmd
}

func (m *sessionModel) View() string {
	if m.chatActive {
		return m.chatView()
	}
	return strings.Join(m.output, "\n") + "\n" + m.input.View()
}

func (m *sessionModel) chatView() string {
	ruleWidth := m.windowWidth
	if ruleWidth < 1 {
		ruleWidth = 1
	}
	return strings.Join([]string{
		"GLOBAL CHAT  |  /leave",
		strings.Repeat("-", ruleWidth),
		m.chatViewport.View(),
		m.chatStatus,
		m.chatInput.View(),
	}, "\n")
}

func (m *sessionModel) resizeChat(width, height int) {
	if width < 1 {
		width = 1
	}
	viewportHeight := height - 4
	if viewportHeight < 1 {
		viewportHeight = 1
	}
	m.chatViewport.Width = width
	m.chatViewport.Height = viewportHeight
	m.chatInput.Width = width - len(m.chatInput.Prompt) - 1
	if m.chatInput.Width < 20 {
		m.chatInput.Width = 20
	}
}

func (m *sessionModel) enterChat() tea.Cmd {
	if m.chat == nil {
		m.addOutput("Chat is unavailable.")
		return nil
	}

	events, cancel := m.chat.Subscribe(m.session.Context(), service.GlobalChatChannel)
	history, err := m.chat.RecentMessages(m.ctx, service.GlobalChatChannel, 100)
	if err != nil {
		cancel()
		m.fail(err)
		return nil
	}

	m.chatGeneration++
	m.chatEvents = events
	m.chatCancel = cancel
	m.chatMessages = history
	m.chatActive = true
	m.chatStatus = ""
	m.chatInput.Reset()
	m.resizeChat(m.windowWidth, m.windowHeight)
	focusCmd := m.chatInput.Focus()
	m.refreshChatViewport(true)

	return tea.Batch(
		tea.Sequence(teaMsgCmd(tea.EnterAltScreen()), teaMsgCmd(tea.EnableMouseCellMotion())),
		waitForChatMessage(m.chatGeneration, m.chatEvents),
		focusCmd,
	)
}

func (m *sessionModel) leaveChat() tea.Cmd {
	m.chatActive = false
	m.chatGeneration++
	if m.chatCancel != nil {
		m.chatCancel()
		m.chatCancel = nil
	}
	m.chatEvents = nil
	m.chatInput.Blur()
	m.chatInput.Reset()
	return tea.Sequence(teaMsgCmd(tea.DisableMouse()), teaMsgCmd(tea.ExitAltScreen()))
}

func (m *sessionModel) handleChatInput(line string) tea.Cmd {
	if line == "" {
		return nil
	}
	body := line
	if strings.HasPrefix(line, "/") {
		args, err := splitCommand(strings.TrimSpace(strings.TrimPrefix(line, "/")))
		if err != nil {
			m.chatStatus = "Command error: " + err.Error()
			return nil
		}
		if len(args) == 0 {
			return nil
		}
		switch strings.ToLower(args[0]) {
		case "leave":
			if len(args) != 1 {
				m.chatStatus = "Usage: /leave"
				return nil
			}
			return m.leaveChat()
		default:
			m.chatStatus = "Unknown chat command: /" + args[0]
			return nil
		}
	}

	if _, err := m.chat.PostMessage(m.ctx, service.GlobalChatChannel, m.user.ID, body); err != nil {
		m.chatStatus = "Error: " + err.Error()
	} else {
		m.chatStatus = ""
	}
	return nil
}

func (m *sessionModel) appendChatMessage(message service.ChatMessage) tea.Cmd {
	latestID := m.latestChatMessageID()
	if latestID > 0 && message.ID > latestID+1 && m.chat != nil {
		generation := m.chatGeneration
		return func() tea.Msg {
			messages, err := m.chat.RecentMessages(m.ctx, service.GlobalChatChannel, 500)
			return chatHistoryMsg{generation: generation, messages: messages, trigger: message, err: err}
		}
	}
	m.appendChatMessageNow(message)
	return nil
}

func (m *sessionModel) latestChatMessageID() int64 {
	var latest int64
	for _, message := range m.chatMessages {
		if message.ID > latest {
			latest = message.ID
		}
	}
	return latest
}

func (m *sessionModel) appendChatMessageNow(message service.ChatMessage) {
	if message.ID == 0 {
		return
	}
	for _, existing := range m.chatMessages {
		if existing.ID == message.ID {
			return
		}
	}
	wasAtBottom := m.chatViewport.AtBottom()
	m.chatMessages = append(m.chatMessages, message)
	sort.Slice(m.chatMessages, func(i, j int) bool { return m.chatMessages[i].ID < m.chatMessages[j].ID })
	if len(m.chatMessages) > 500 {
		m.chatMessages = append([]service.ChatMessage(nil), m.chatMessages[len(m.chatMessages)-500:]...)
	}
	m.refreshChatViewport(wasAtBottom)
}

func (m *sessionModel) refreshChatViewport(goToBottom bool) {
	lines := make([]string, 0, len(m.chatMessages))
	for _, message := range m.chatMessages {
		lines = append(lines, formatChatMessage(message))
	}
	m.chatViewport.SetContent(strings.Join(lines, "\n"))
	if goToBottom {
		m.chatViewport.GotoBottom()
	}
}

func formatChatMessage(message service.ChatMessage) string {
	username := cleanTerminalText(message.Username)
	body := cleanTerminalText(message.Text)
	timestamp := message.Timestamp.Local().Format("15:04")
	return "[" + timestamp + "] " + username + ": " + body
}

func cleanTerminalText(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
}

func (m *sessionModel) prompt() string {
	return m.user.Username + "@community:" + m.currentLocation().Path() + "> "
}

func (m *sessionModel) currentPrompt() string {
	switch m.compose {
	case composeNewTitle:
		return "Topic title: "
	case composeNewBody, composeReplyBody:
		return "| "
	default:
		return m.prompt()
	}
}

func (m *sessionModel) addOutput(value string) {
	for _, line := range strings.Split(strings.TrimRight(value, "\n"), "\n") {
		// Forum content is untrusted terminal output. Strip control characters
		// so posts cannot inject ANSI escape sequences into SSH clients.
		line = strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return -1
			}
			return r
		}, line)
		m.output = append(m.output, line)
	}
	if len(m.output) > 1200 {
		m.output = append([]string(nil), m.output[len(m.output)-1000:]...)
	}
}

func (m *sessionModel) fail(err error) {
	if err != nil {
		m.addOutput("Error: " + err.Error())
	}
}

func (m *sessionModel) execute(line string) (bool, tea.Cmd) {
	args, err := splitCommand(line)
	if err != nil {
		m.fail(err)
		return false, nil
	}
	if len(args) == 0 {
		return false, nil
	}

	cmd, ok := m.lookupContextCommand(args[0])
	if !ok {
		m.addOutput("Unknown command: " + args[0] + ". Type 'help' for commands.")
		return false, nil
	}

	cmdArgs := args[1:]
	prefix := ""
	requiredPermission := cmd.Permission
	if len(cmd.Subcommands) > 0 {
		if len(cmdArgs) == 0 {
			if !m.hasPermission(requiredPermission) {
				m.fail(service.ErrPermissionDenied)
				return false, nil
			}
			m.addOutput("Usage: " + cmd.Name + " <subcommand>. Type 'help " + cmd.Name + "' for details.")
			return false, nil
		}

		subcommand, found := lookupSubcommand(cmd, cmdArgs[0])
		if !found {
			m.addOutput("Unknown " + cmd.Name + " subcommand: " + cmdArgs[0] + ". Type 'help " + cmd.Name + "' for commands.")
			return false, nil
		}
		requiredPermission = effectivePermission(cmd, subcommand)
		prefix = cmd.Name
		cmd = subcommand
		cmdArgs = cmdArgs[1:]
	}

	if !m.hasPermission(requiredPermission) {
		m.fail(service.ErrPermissionDenied)
		return false, nil
	}
	if len(cmdArgs) < requiredArguments(cmd) {
		m.addOutput("Usage: " + commandUsage(cmd, prefix))
		return false, nil
	}
	if cmd.Handler == nil {
		m.addOutput("Command is not available.")
		return false, nil
	}
	return cmd.Handler(m, cmdArgs)
}

func (m *sessionModel) help(args []string) {
	if len(args) > 2 {
		m.addOutput("Usage: help [command [subcommand]]")
		return
	}

	available := m.availableCommands()
	if len(args) == 0 {
		for _, group := range commandGroupOrder {
			var entries []string
			for _, cmd := range available {
				if cmd.Group != group {
					continue
				}
				if len(cmd.Subcommands) > 0 {
					for _, subcommand := range cmd.Subcommands {
						if !m.hasPermission(effectivePermission(cmd, subcommand)) {
							continue
						}
						entries = append(entries, fmt.Sprintf("  %-38s %s", commandUsage(subcommand, cmd.Name), subcommand.Description))
					}
					continue
				}
				entries = append(entries, fmt.Sprintf("  %-38s %s", commandUsage(cmd, ""), cmd.Description))
			}
			if len(entries) == 0 {
				continue
			}
			m.addOutput(group + ":")
			for _, entry := range entries {
				m.addOutput(entry)
			}
			m.addOutput("")
		}
		m.addOutput("Use 'help <command>' for command details.")
		return
	}

	cmd, ok := m.lookupAvailableCommand(args[0])
	if !ok {
		m.addOutput("No help available for '" + args[0] + "'.")
		return
	}

	if len(args) == 2 {
		subcommand, found := lookupSubcommand(cmd, args[1])
		if !found {
			m.addOutput("No help available for '" + cmd.Name + " " + args[1] + "'.")
			return
		}
		if !m.hasPermission(effectivePermission(cmd, subcommand)) {
			m.fail(service.ErrPermissionDenied)
			return
		}
		subcommand.Permission = effectivePermission(cmd, subcommand)
		m.writeCommandHelp(subcommand, cmd.Name)
		return
	}

	if len(cmd.Subcommands) > 0 {
		m.addOutput(cmd.Name + " — " + cmd.Description)
		m.addOutput("Usage: " + cmd.Name + " <subcommand>")
		if cmd.Permission != permissionMember {
			m.addOutput("Permission: " + commandPermissionName(cmd.Permission))
		}
		m.addOutput("Subcommands:")
		for _, subcommand := range cmd.Subcommands {
			if !m.hasPermission(effectivePermission(cmd, subcommand)) {
				continue
			}
			m.addOutput(fmt.Sprintf("  %-36s %s", commandUsage(subcommand, cmd.Name), subcommand.Description))
		}
		return
	}

	m.writeCommandHelp(cmd, "")
}

func (m *sessionModel) writeCommandHelp(cmd command, prefix string) {
	m.addOutput("Usage: " + commandUsage(cmd, prefix))
	m.addOutput(cmd.Description)
	if len(cmd.Aliases) > 0 {
		m.addOutput("Aliases: " + strings.Join(cmd.Aliases, ", "))
	}
	if cmd.Permission != permissionMember {
		m.addOutput("Permission: " + commandPermissionName(cmd.Permission))
	}
	for _, arg := range cmd.Arguments {
		m.addOutput(fmt.Sprintf("  %s: %s", arg.Name, arg.Description))
	}
}

func (m *sessionModel) listLocation(args []string) {
	if len(args) > 1 {
		m.addOutput("Usage: ls [path]")
		return
	}

	target := m.currentLocation()
	if len(args) == 1 {
		resolved, err := m.filesystem().Resolve(m.currentLocation(), args[0])
		if err != nil {
			m.fail(err)
			return
		}
		target = resolved
	}
	m.displayLocation(target)
}

func (m *sessionModel) displayLocation(target location) {
	switch target := target.(type) {
	case rootLocation:
		m.addOutput("boards/")
	case boardsLocation:
		boards, err := m.boards.ListBoards(m.ctx)
		if err != nil {
			m.fail(err)
			return
		}
		m.printBoards(boards)
	case boardLocation:
		m.printBoardTopics(target.board.Slug, 1)
	case threadLocation:
		m.displayThread(target.id, 1)
	default:
		m.addOutput("No such location.")
	}
}

const recentTopicLimit = 100

func (m *sessionModel) listRecentTopics() {
	threads, err := m.boards.RecentThreads(m.ctx, recentTopicLimit)
	if err != nil {
		m.fail(err)
		return
	}
	if len(threads) == 0 {
		m.addOutput("No recent topics.")
		return
	}
	m.addOutput("RECENT ACTIVITY")
	m.addOutput("ID    TOPIC                                            REPLIES  LAST ACTIVITY")
	for _, thread := range threads {
		title := thread.Title
		if len(title) > 46 {
			title = title[:43] + "..."
		}
		m.addOutput(fmt.Sprintf("%-4d  %-46s %7d  %s", thread.ID, title, thread.ReplyCount, relativeTime(thread.LastActivityAt)))
	}
	m.addOutput("Use 'show <topic-id>' to open a topic.")
}

func (m *sessionModel) printBoards(boards []service.BoardListing) {
	if len(boards) == 0 {
		m.addOutput("No boards have been created.")
		return
	}
	m.addOutput("SLUG                 BOARD                         STATUS")
	for _, b := range boards {
		m.addOutput(fmt.Sprintf("%-20s %-29s %s", b.Slug, b.Name, b.Status))
		if b.Description != "" {
			m.addOutput("  " + b.Description)
		}
	}
}

func (m *sessionModel) printBoardTopics(slug string, page int) {
	b, err := m.boards.GetBoardBySlug(m.ctx, slug)
	if err != nil {
		m.fail(err)
		return
	}
	m.addOutput(fmt.Sprintf("%s — %s", b.Name, b.Description))
	m.addOutput(fmt.Sprintf("Board status: %s", b.Status))
	threads, total, err := m.boards.ListThreadsPage(m.ctx, b.ID, page, 20)
	if err != nil {
		m.fail(err)
		return
	}
	m.printThreadList(threads, page, total)
}

func (m *sessionModel) listTopics(args []string) {
	board, ok := m.currentBoardLocation()
	if !ok {
		m.addOutput("'topics' is only available inside a board.")
		return
	}
	page := pageArgument(args)
	threads, total, err := m.boards.ListThreadsPage(m.ctx, board.board.ID, page, 20)
	if err != nil {
		m.fail(err)
		return
	}
	m.printThreadList(threads, page, total)
}

func (m *sessionModel) printThreadList(threads []service.ThreadListing, page, total int) {
	if len(threads) == 0 {
		m.addOutput("No topics found.")
		return
	}
	m.addOutput("ID    TOPIC                                            REPLIES  LAST ACTIVITY")
	for _, t := range threads {
		title := t.Title
		if len(title) > 46 {
			title = title[:43] + "..."
		}
		m.addOutput(fmt.Sprintf("%-4d  %-46s %7d  %s", t.ID, title, t.ReplyCount, relativeTime(t.LastActivityAt)))
	}
	start := (page-1)*20 + 1
	end := start + len(threads) - 1
	if end > total {
		end = total
	}
	m.addOutput(fmt.Sprintf("Showing %d-%d of %d. Use 'topics --page %d' for more.", start, end, total, page+1))
}



func (m *sessionModel) showCurrent(args []string) {
	page := pageArgument(args)
	if len(args) > 0 && args[0] != "--page" {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			m.fail(fmt.Errorf("expected a topic ID"))
			return
		}
		thread, err := m.boards.GetThreadByID(m.ctx, id)
		if err != nil {
			m.fail(err)
			return
		}
		board, err := m.boards.GetBoardByID(m.ctx, thread.BoardID)
		if err != nil {
			m.fail(err)
			return
		}
		// An explicit topic ID navigates to its owning board before opening it.
		m.location = threadLocation{id: id, parent: boardLocation{board: board}}
		m.displayThread(id, page)
		return
	}

	current, ok := m.currentThreadLocation()
	if !ok {
		m.fail(fmt.Errorf("show requires a topic ID unless already inside a topic"))
		return
	}
	m.displayThread(current.id, page)
}

func (m *sessionModel) displayThread(id, page int) {
	t, err := m.boards.GetThreadByID(m.ctx, id)
	if err != nil {
		m.fail(err)
		return
	}
	comments, total, err := m.boards.ListCommentsPage(m.ctx, id, page, 10)
	if err != nil {
		m.fail(err)
		return
	}
	m.addOutput(fmt.Sprintf("[%d] %s", t.ID, t.Title))
	m.addOutput(fmt.Sprintf("Started by %s · %s · %d replies", t.Author.Username, t.CreatedAt.Format("2006-01-02 15:04"), total))
	m.addOutput(strings.Repeat("-", 72))
	m.addOutput(t.Author.Username + ":")
	m.addOutput(t.Body)
	m.addOutput(strings.Repeat("-", 72))
	for _, c := range comments {
		m.addOutput(fmt.Sprintf("%s · %s", c.Author.Username, c.PostedAt.Format("2006-01-02 15:04")))
		m.addOutput(c.Body)
		m.addOutput(strings.Repeat("-", 72))
	}
	if total > 0 {
		start := (page-1)*10 + 1
		end := start + len(comments) - 1
		if end > total {
			end = total
		}
		m.addOutput(fmt.Sprintf("Showing replies %d-%d of %d. Use 'show --page %d' for more.", start, end, total, page+1))
	}
}



func (m *sessionModel) changeDirectory(args []string) {
	if len(args) != 1 {
		m.addOutput("Usage: cd <path>")
		return
	}
	target, err := m.filesystem().Resolve(m.currentLocation(), args[0])
	if err != nil {
		m.fail(err)
		return
	}
	m.location = target
	m.displayLocation(target)
}

func (m *sessionModel) startNewTopic() {
	currentBoard, ok := m.currentBoardLocation()
	if !ok {
		m.addOutput("'new' is only available inside a board.")
		return
	}
	board, err := m.boards.GetBoardByID(m.ctx, currentBoard.board.ID)
	if err != nil {
		m.fail(err)
		return
	}
	if board.Status != service.BoardActive {
		m.fail(service.ErrReadOnly)
		return
	}
	m.compose = composeNewTitle
	m.draftLines = nil
	m.input.Prompt = m.currentPrompt()
	m.addOutput("Creating a topic. Enter /cancel at any prompt to abort.")
}

func (m *sessionModel) startReply() {
	thread, ok := m.currentThreadLocation()
	if !ok {
		m.addOutput("'reply' is only available inside a topic.")
		return
	}
	if _, err := m.boards.GetThreadByID(m.ctx, thread.id); err != nil {
		m.fail(err)
		return
	}
	m.compose = composeReplyBody
	m.draftThreadID = thread.id
	m.draftLines = nil
	m.addOutput("Reply body. Enter a single '.' on a line to post; use /cancel to abort.")
	m.input.Prompt = m.currentPrompt()
}

func (m *sessionModel) handleComposition(line string) {
	if strings.TrimSpace(line) == "/cancel" {
		m.cancelComposition()
		return
	}
	switch m.compose {
	case composeNewTitle:
		if strings.TrimSpace(line) == "" {
			m.addOutput("A topic title is required.")
			return
		}
		m.draftTitle = strings.TrimSpace(line)
		m.compose = composeNewBody
		m.addOutput("Topic body. Enter a single '.' on a line to finish; use /cancel to abort.")
	case composeNewBody, composeReplyBody:
		if line == "." {
			m.finishComposition()
			return
		}
		if line == ".." {
			line = "."
		}
		m.draftLines = append(m.draftLines, line)
		m.addOutput("| " + line)
	}
	m.input.Prompt = m.currentPrompt()
}

func (m *sessionModel) finishComposition() {
	body := strings.Join(m.draftLines, "\n")
	switch m.compose {
	case composeNewBody:
		board, ok := m.currentBoardLocation()
		if !ok {
			m.fail(fmt.Errorf("new is only available inside a board"))
			break
		}
		threadID, err := m.boards.CreateThreadWithBody(m.ctx, m.draftTitle, body, board.board.ID, m.user.ID)
		if err != nil {
			m.fail(err)
			break
		}
		if _, err := m.boards.GetThreadByID(m.ctx, threadID); err != nil {
			m.fail(err)
			break
		}
		m.addOutput(fmt.Sprintf("Topic created: #%d", threadID))
		m.location = threadLocation{id: threadID, parent: board}
		m.displayThread(threadID, 1)
	case composeReplyBody:
		err := m.boards.CreateComment(m.ctx, m.draftThreadID, m.user.ID, body)
		if err != nil {
			m.fail(err)
		} else {
			m.addOutput("Reply posted.")
			m.displayThread(m.draftThreadID, 1)
		}
	}
	m.resetComposition()
}

func (m *sessionModel) resetComposition() {
	m.compose, m.draftTitle, m.draftLines = composeNone, "", nil
	m.draftThreadID = 0
	m.input.Prompt = m.currentPrompt()
}

func (m *sessionModel) cancelComposition() {
	m.addOutput("Cancelled.")
	m.resetComposition()
}



func (m *sessionModel) deleteOwnTopic(args []string) {
	id, err := m.targetID(args)
	if err != nil {
		m.fail(err)
		return
	}
	if err := m.boards.DeleteThread(m.ctx, id); err != nil {
		m.fail(err)
		return
	}
	m.addOutput("Topic deleted.")
	if current, ok := m.currentThreadLocation(); ok && current.id == id {
		m.location = current.Parent()
	}
}

func (m *sessionModel) deleteOwnReply(args []string) {
	if len(args) != 1 {
		m.addOutput("Usage: delete-reply <reply-id>")
		return
	}
	id, err := strconv.Atoi(args[0])
	if err != nil {
		m.addOutput("Invalid reply ID.")
		return
	}
	if err := m.boards.DeleteComment(m.ctx, id); err != nil {
		m.fail(err)
		return
	}
	m.addOutput("Reply deleted.")
}

func (m *sessionModel) isAdmin() bool { return m.user.Role == service.RoleAdmin }

func (m *sessionModel) targetID(args []string) (int, error) {
	if len(args) == 1 {
		id, err := strconv.Atoi(args[0])
		if err == nil {
			return id, nil
		}
	}
	if len(args) == 0 {
		if current, ok := m.currentThreadLocation(); ok {
			return current.id, nil
		}
		return 0, fmt.Errorf("delete-topic requires a topic ID outside a topic location")
	}
	return 0, fmt.Errorf("usage: delete-topic [topic-id]")
}

func pageArgument(args []string) int {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--page" {
			n, err := strconv.Atoi(args[i+1])
			if err == nil && n > 0 {
				return n
			}
		}
	}
	return 1
}

func relativeTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}

func splitCommand(line string) ([]string, error) {
	var result []string
	var current strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if current.Len() > 0 {
			result = append(result, current.String())
			current.Reset()
		}
	}
	for _, r := range line {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && quote != 0 {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
		case ' ', '\t':
			flush()
		default:
			current.WriteRune(r)
		}
	}
	if escaped {
		current.WriteRune('\\')
	}
	if quote != 0 {
		return nil, errors.New("unterminated quoted argument")
	}
	flush()
	return result, nil
}
