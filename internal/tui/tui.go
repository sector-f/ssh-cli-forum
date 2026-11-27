package tui

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/sector-f/ssh-cli-forum/forum"
)

type composeState int

const (
	composeNone composeState = iota
	composeNewTitle
	composeNewBody
	composeReplyBody
	composeEditTitle
	composeEditBody
	composeEditReplyBody
)

type sessionModel struct {
	session ssh.Session
	users   forum.UserService
	boards  forum.BoardService
	user    forum.User

	cwd    string
	input  textinput.Model
	output []string

	compose       composeState
	draftTitle    string
	draftLines    []string
	draftThreadID int
	draftReplyID  int
}

var _ tea.Model = (*sessionModel)(nil)

func NewProgram(sess ssh.Session, userService forum.UserService, boardService forum.BoardService) (*tea.Program, error) {
	user, err := userService.GetUserByUsername(sess.User())
	if err != nil {
		return nil, fmt.Errorf("load account: %w", err)
	}
	if user.Status == forum.StatusSuspended {
		return nil, forum.ErrUserSuspended
	}
	input := textinput.New()
	input.Prompt = user.Username + "@community:/> "
	input.Focus()
	input.CharLimit = 4096
	input.Width = 100
	m := &sessionModel{
		session: sess, users: userService, boards: boardService, user: user,
		cwd: "/", input: input,
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

func Handler(userService forum.UserService, boardService forum.BoardService) bubbletea.ProgramHandler {
	return func(sess ssh.Session) *tea.Program {
		program, err := NewProgram(sess, userService, boardService)
		if err != nil {
			_, _ = fmt.Fprintf(sess, "Unable to start forum session: %v\r\n", err)
			return nil
		}
		return program
	}
}

func (m *sessionModel) Init() tea.Cmd { return textinput.Blink }

func (m *sessionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		width := msg.Width - len(m.prompt()) - 1
		if width < 20 { width = 20 }
		m.input.Width = width
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			if m.compose != composeNone {
				m.cancelComposition()
				return m, nil
			}
			return m, tea.Quit
		case "enter":
			line := m.input.Value()
			m.input.Reset()
			if m.compose != composeNone {
				m.handleComposition(line)
			} else {
				line = strings.TrimSpace(line)
				if line != "" {
					m.addOutput(m.prompt() + line)
					if m.execute(line) {
						return m, tea.Quit
					}
				}
			}
			m.input.Prompt = m.currentPrompt()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *sessionModel) View() string {
	return strings.Join(m.output, "\n") + "\n" + m.input.View()
}

func (m *sessionModel) prompt() string {
	return m.user.Username + "@community:" + m.cwd + "> "
}

func (m *sessionModel) currentPrompt() string {
	switch m.compose {
	case composeNewTitle:
		return "Topic title: "
	case composeNewBody, composeReplyBody, composeEditBody, composeEditReplyBody:
		return "| "
	case composeEditTitle:
		return "New title (empty keeps current): "
	default:
		return m.prompt()
	}
}

func (m *sessionModel) addOutput(value string) {
	for _, line := range strings.Split(strings.TrimRight(value, "\n"), "\n") {
		// Forum content is untrusted terminal output. Strip control characters
		// so posts cannot inject ANSI escape sequences into SSH clients.
		line = strings.Map(func(r rune) rune {
			if unicode.IsControl(r) { return -1 }
			return r
		}, line)
		m.output = append(m.output, line)
	}
	if len(m.output) > 1200 {
		m.output = append([]string(nil), m.output[len(m.output)-1000:]...)
	}
}

func (m *sessionModel) fail(err error) {
	if err != nil { m.addOutput("Error: " + err.Error()) }
}

func (m *sessionModel) execute(line string) bool {
	args, err := splitCommand(line)
	if err != nil { m.fail(err); return false }
	if len(args) == 0 { return false }

	switch strings.ToLower(args[0]) {
	case "help", "?":
		m.help(args[1:])
	case "pwd":
		m.addOutput(m.cwd)
	case "ls":
		m.listLocation(args[1:])
	case "cd":
		m.changeDirectory(args[1:])
	case "whoami":
		m.addOutput(fmt.Sprintf("%s (role: %s, status: %s)", m.user.Username, m.user.Role, m.user.Status))
	case "profile":
		m.showProfile(args[1:])
	case "recent":
		m.showRecent()
	case "topics":
		m.listTopics(args[1:])
	case "show":
		m.showCurrent(args[1:])
	case "new":
		m.startNewTopic()
	case "reply":
		m.startReply()
	case "edit-topic":
		m.startEditTopic(args[1:])
	case "edit-reply":
		m.startEditReply(args[1:])
	case "delete-topic":
		m.deleteOwnTopic(args[1:])
	case "delete-reply":
		m.deleteOwnReply(args[1:])
	case "lock", "unlock", "pin", "unpin", "hide-thread", "restore-thread", "delete-thread":
		m.moderateThread(args)
	case "modlog":
		m.showModlog(args[1:])
	case "board":
		m.boardAdmin(args[1:])
	case "invite":
		m.inviteAdmin(args[1:])
	case "users":
		m.listUsers()
	case "user":
		m.userAdmin(args[1:])
	case "exit", "quit":
		m.addOutput("Disconnecting.")
		return true
	default:
		m.addOutput("Unknown command: " + args[0] + ". Type 'help' for commands.")
	}
	return false
}

func (m *sessionModel) help(args []string) {
	if len(args) > 0 {
		m.addOutput("Help for '" + args[0] + "' is not available yet.")
		return
	}
	m.addOutput(`Navigation:
  ls [path]             List boards or topics
  cd <path>             Change location
  cd ..                 Go to parent location
  pwd                   Show current location
  recent                Show recently active topics
  topics [--page N]     List topics in the current board
  show [--page N]       Display the current topic
  whoami                Show your account and role
  profile [username]    Display a profile
  help                  Show this help
  exit                  Disconnect

Posting (run from a board or topic):
  new                   Create a topic in the current board
  reply                 Reply to the current topic
  edit-topic <id>       Edit a topic you authored
  edit-reply <id>       Edit a reply you authored
  delete-topic <id>     Delete your topic
  delete-reply <id>     Delete your reply

Moderator commands:
  lock <id> [reason]          Lock a topic
  unlock <id> [reason]        Reopen a topic
  pin <id> [reason]           Pin a topic
  unpin <id> [reason]         Unpin a topic
  hide-thread <id> [reason]   Hide a topic
  restore-thread <id> [reason] Restore a topic
  delete-thread <id> [reason] Delete a topic
  delete-reply <id> [reason]  Delete any reply
  modlog                      View moderation history

Administrator commands:
  board create <slug> "Name" "Description"
  board update <slug> "Name" "Description"
  board archive <slug>
  board restore <slug>
  invite create [duration] [--admin]
  invite list
  invite revoke <code>
  users
  user role <username> <member|moderator|admin>
  user suspend <username>
  user activate <username>`)
}

func (m *sessionModel) listLocation(args []string) {
	location := m.cwd
	if len(args) > 1 { m.addOutput("Usage: ls [path]"); return }
	if len(args) == 1 {
		if strings.HasPrefix(args[0], "/") { location = path.Clean(args[0]) } else { location = path.Clean(path.Join(m.cwd, args[0])) }
	}
	switch {
	case location == "/":
		m.addOutput("boards/\nrecent\nhelp")
	case location == "/boards":
		boards, err := m.boards.ListBoards()
		if err != nil { m.fail(err); return }
		m.printBoards(boards)
	case location == "/recent":
		m.showRecent()
	case strings.HasPrefix(location, "/boards/"):
		parts := strings.Split(strings.TrimPrefix(location, "/boards/"), "/")
		if len(parts) == 1 && parts[0] != "" {
			m.printBoardTopics(parts[0], 1)
		} else if len(parts) == 2 {
			id, err := strconv.Atoi(parts[1]); if err != nil { m.fail(forum.ErrThreadNotFound); return }
			m.displayThread(id, 1)
		} else { m.addOutput("No such location: " + location) }
	default:
		m.addOutput("No such location: " + location)
	}
}

func (m *sessionModel) printBoards(boards []forum.BoardListing) {
	if len(boards) == 0 { m.addOutput("No boards have been created."); return }
	m.addOutput("SLUG                 BOARD                         STATUS")
	for _, b := range boards {
		m.addOutput(fmt.Sprintf("%-20s %-29s %s", b.Slug, b.Name, b.Status))
		if b.Description != "" { m.addOutput("  " + b.Description) }
	}
}

func (m *sessionModel) printBoardTopics(slug string, page int) {
	b, err := m.boards.GetBoardBySlug(slug)
	if err != nil { m.fail(err); return }
	m.addOutput(fmt.Sprintf("%s — %s", b.Name, b.Description))
	m.addOutput(fmt.Sprintf("Board status: %s", b.Status))
	threads, total, err := m.boards.ListThreadsPage(b.ID, page, 20)
	if err != nil { m.fail(err); return }
	m.printThreadList(threads, page, total)
}

func (m *sessionModel) listTopics(args []string) {
	parts := strings.Split(strings.TrimPrefix(m.cwd, "/boards/"), "/")
	if !strings.HasPrefix(m.cwd, "/boards/") || len(parts) != 1 || parts[0] == "" {
		m.addOutput("'topics' must be run from inside a board.")
		return
	}
	page := pageArgument(args)
	board, err := m.boards.GetBoardBySlug(parts[0])
	if err != nil { m.fail(err); return }
	threads, total, err := m.boards.ListThreadsPage(board.ID, page, 20)
	if err != nil { m.fail(err); return }
	m.printThreadList(threads, page, total)
}

func (m *sessionModel) printThreadList(threads []forum.ThreadListing, page, total int) {
	if len(threads) == 0 { m.addOutput("No topics found."); return }
	m.addOutput("ID    TOPIC                                            REPLIES  LAST ACTIVITY")
	for _, t := range threads {
		title := t.Title
		if len(title) > 46 { title = title[:43] + "..." }
		marker := " "
		if t.IsPinned { marker = "*" }
		m.addOutput(fmt.Sprintf("%-4d%s %-46s %7d  %s", t.ID, marker, title, t.ReplyCount, relativeTime(t.LastActivityAt)))
	}
	start := (page-1)*20 + 1
	end := start + len(threads) - 1
	if end > total { end = total }
	m.addOutput(fmt.Sprintf("Showing %d-%d of %d. Use 'topics --page %d' for more.", start, end, total, page+1))
}

func (m *sessionModel) showRecent() {
	threads, err := m.boards.RecentThreads(20)
	if err != nil { m.fail(err); return }
	if len(threads) == 0 { m.addOutput("No recent topics."); return }
	m.addOutput("RECENT ACTIVITY")
	m.addOutput("ID    TOPIC                                            REPLIES  LAST ACTIVITY")
	for _, t := range threads {
		title := t.Title
		if len(title) > 46 { title = title[:43] + "..." }
		m.addOutput(fmt.Sprintf("%-4d  %-46s %7d  %s", t.ID, title, t.ReplyCount, relativeTime(t.LastActivityAt)))
	}
	m.addOutput("Use 'cd /boards/<board-slug>/<topic-id>' to open a topic.")
}

func (m *sessionModel) showCurrent(args []string) {
	id, err := m.currentThreadID(args)
	if err != nil { m.fail(err); return }
	// An explicit topic ID is a navigation action too: establish its board/topic
	// context so subsequent commands such as 'reply' work without another cd.
	if len(args) > 0 && args[0] != "--page" {
		thread, err := m.boards.GetThreadByID(id)
		if err != nil { m.fail(err); return }
		board, err := m.boards.GetBoardByID(thread.BoardID)
		if err != nil { m.fail(err); return }
		m.cwd = path.Join("/boards", board.Slug, strconv.Itoa(id))
	}
	m.displayThread(id, pageArgument(args))
}

func (m *sessionModel) displayThread(id, page int) {
	t, err := m.boards.GetThreadByID(id)
	if err != nil { m.fail(err); return }
	if t.Status == forum.ThreadHidden && !m.isModerator() { m.addOutput("Topic not found."); return }
	m.addOutput(fmt.Sprintf("[%d] %s", t.ID, t.Title))
	m.addOutput(fmt.Sprintf("Started by %s · %s · %d replies", t.Author.Username, t.CreatedAt.Format("2006-01-02 15:04"), len(t.Comments)))
	if t.IsPinned { m.addOutput("PINNED") }
	if t.Status == forum.ThreadLocked { m.addOutput("LOCKED") }
	m.addOutput(strings.Repeat("-", 72))
	m.addOutput(t.Author.Username + ":")
	m.addOutput(t.Body)
	m.addOutput(strings.Repeat("-", 72))
	comments, total, err := m.boards.ListCommentsPage(id, page, 10)
	if err != nil { m.fail(err); return }
	for _, c := range comments {
		m.addOutput(fmt.Sprintf("%s · %s", c.Author.Username, c.PostedAt.Format("2006-01-02 15:04")))
		m.addOutput(c.Body)
		m.addOutput(strings.Repeat("-", 72))
	}
	if total > 0 {
		start := (page-1)*10 + 1
		end := start + len(comments) - 1
		if end > total { end = total }
		m.addOutput(fmt.Sprintf("Showing replies %d-%d of %d. Use 'show --page %d' for more.", start, end, total, page+1))
	}
}

func (m *sessionModel) currentThreadID(args []string) (int, error) {
	if len(args) > 0 && args[0] != "--page" {
		id, err := strconv.Atoi(args[0]); if err != nil { return 0, errors.New("expected a topic ID") }; return id, nil
	}
	if !strings.HasPrefix(m.cwd, "/boards/") { return 0, errors.New("this command must be run from a topic") }
	parts := strings.Split(m.cwd, "/")
	if len(parts) != 4 { return 0, errors.New("this command must be run from a topic") }
	id, err := strconv.Atoi(parts[3])
	return id, err
}

func (m *sessionModel) changeDirectory(args []string) {
	if len(args) != 1 { m.addOutput("Usage: cd <path>"); return }
	target := args[0]
	if !strings.HasPrefix(target, "/") { target = path.Join(m.cwd, target) }
	target = path.Clean(target)
	switch {
	case target == "/", target == "/boards", target == "/recent":
		m.cwd = target
		if target == "/boards" {
			if b, err := m.boards.ListBoards(); err == nil { m.printBoards(b) }
		}
		if target == "/recent" { m.showRecent() }
	case strings.HasPrefix(target, "/boards/"):
		parts := strings.Split(strings.TrimPrefix(target, "/boards/"), "/")
		if len(parts) == 1 && parts[0] != "" {
			b, err := m.boards.GetBoardBySlug(parts[0]); if err != nil { m.fail(err); return }
			m.cwd = target
			m.addOutput(fmt.Sprintf("%s — %s", b.Name, b.Description))
			m.listTopics(nil)
		} else if len(parts) == 2 && parts[0] != "" {
			id, err := strconv.Atoi(parts[1]); if err != nil { m.addOutput("Invalid topic ID."); return }
			t, err := m.boards.GetThreadByID(id); if err != nil { m.fail(err); return }
			b, err := m.boards.GetBoardByID(t.BoardID); if err != nil { m.fail(err); return }
			if b.Slug != parts[0] { m.addOutput("Topic does not belong to this board."); return }
			m.cwd = target
			m.displayThread(id, 1)
		} else { m.addOutput("No such location: " + target) }
	default:
		m.addOutput("No such location: " + target)
	}
}

func (m *sessionModel) showProfile(args []string) {
	username := m.user.Username
	if len(args) > 1 { m.addOutput("Usage: profile [username]"); return }
	if len(args) == 1 { username = args[0] }
	u, err := m.users.GetUserByUsername(username)
	if err != nil { m.fail(forum.ErrUserNotFound); return }
	name := u.DisplayName
	if name == "" { name = u.Username }
	m.addOutput(fmt.Sprintf("%s (@%s)", name, u.Username))
	m.addOutput("Role: " + u.Role)
	m.addOutput("Status: " + u.Status)
	m.addOutput("Joined: " + u.CreatedAt.Format("2006-01-02"))
}

func (m *sessionModel) startNewTopic() {
	if !strings.HasPrefix(m.cwd, "/boards/") || strings.Count(m.cwd, "/") != 2 {
		m.addOutput("'new' must be run from inside a board.")
		return
	}
	board, err := m.boards.GetBoardBySlug(strings.TrimPrefix(m.cwd, "/boards/"))
	if err != nil { m.fail(err); return }
	if board.Status != forum.BoardActive { m.fail(forum.ErrReadOnly); return }
	m.compose = composeNewTitle
	m.draftLines = nil
	m.input.Prompt = m.currentPrompt()
	m.addOutput("Creating a topic. Enter /cancel at any prompt to abort.")
}

func (m *sessionModel) startReply() {
	id, err := m.currentThreadID(nil)
	if err != nil { m.fail(errors.New("'reply' must be run from inside a topic")); return }
	t, err := m.boards.GetThreadByID(id)
	if err != nil { m.fail(err); return }
	if t.Status == forum.ThreadLocked { m.fail(forum.ErrThreadLocked); return }
	m.compose = composeReplyBody
	m.draftThreadID = id
	m.draftLines = nil
	m.addOutput("Reply body. Enter a single '.' on a line to post; use /cancel to abort.")
	m.input.Prompt = m.currentPrompt()
}

func (m *sessionModel) startEditTopic(args []string) {
	id, err := m.currentThreadID(args)
	if err != nil { m.fail(errors.New("usage: edit-topic <id> (from inside a topic)")); return }
	t, err := m.boards.GetThreadByID(id)
	if err != nil { m.fail(err); return }
	if t.Author.ID != m.user.ID && !m.isModerator() { m.fail(forum.ErrPermissionDenied); return }
	m.compose, m.draftThreadID, m.draftTitle = composeEditTitle, id, t.Title
	board, boardErr := m.boards.GetBoardByID(t.BoardID)
	if boardErr == nil { m.cwd = path.Join("/boards", board.Slug, strconv.Itoa(id)) }
	m.draftLines = nil
	m.addOutput("Enter a replacement title, or leave blank to keep the current title. /cancel aborts.")
	m.input.Prompt = m.currentPrompt()
}

func (m *sessionModel) startEditReply(args []string) {
	if len(args) != 1 { m.addOutput("Usage: edit-reply <reply-id>"); return }
	id, err := strconv.Atoi(args[0]); if err != nil { m.addOutput("Invalid reply ID."); return }
	threadID, err := m.currentThreadID(nil)
	if err != nil { m.fail(errors.New("edit-reply must be run from inside a topic")); return }
	comments, err := m.boards.ListComments(threadID)
	if err != nil { m.fail(err); return }
	for _, c := range comments {
		if c.ID == id {
			if c.Author.ID != m.user.ID && !m.isModerator() { m.fail(forum.ErrPermissionDenied); return }
			m.compose, m.draftThreadID, m.draftReplyID = composeEditReplyBody, threadID, id
			m.draftLines = nil
			m.addOutput("Enter the replacement reply. Finish with '.'; /cancel aborts.")
			m.input.Prompt = m.currentPrompt()
			return
		}
	}
	m.fail(forum.ErrCommentNotFound)
}

func (m *sessionModel) handleComposition(line string) {
	if strings.TrimSpace(line) == "/cancel" { m.cancelComposition(); return }
	switch m.compose {
	case composeNewTitle:
		if strings.TrimSpace(line) == "" { m.addOutput("A topic title is required."); return }
		m.draftTitle = strings.TrimSpace(line)
		m.compose = composeNewBody
		m.addOutput("Topic body. Enter a single '.' on a line to finish; use /cancel to abort.")
	case composeEditTitle:
		if strings.TrimSpace(line) != "" { m.draftTitle = strings.TrimSpace(line) }
		m.compose = composeEditBody
		m.addOutput("Topic body. Enter a single '.' on a line to finish; use /cancel to abort.")
	case composeNewBody, composeReplyBody, composeEditBody, composeEditReplyBody:
		if line == "." {
			m.finishComposition()
			return
		}
		if line == ".." { line = "." }
		m.draftLines = append(m.draftLines, line)
		m.addOutput("| " + line)
	}
	m.input.Prompt = m.currentPrompt()
}

func (m *sessionModel) finishComposition() {
	body := strings.Join(m.draftLines, "\n")
	switch m.compose {
	case composeNewBody:
		id, err := m.currentBoardID()
		if err == nil {
			var threadID int
			threadID, err = m.boards.CreateThreadWithBody(m.draftTitle, body, id, m.user.ID)
			if err == nil {
				m.addOutput(fmt.Sprintf("Topic created: #%d", threadID))
				m.cwd = path.Join(m.cwd, strconv.Itoa(threadID))
				m.displayThread(threadID, 1)
			}
		}
		m.fail(err)
	case composeReplyBody:
		err := m.boards.CreateComment(m.draftThreadID, m.user.ID, body)
		if err != nil { m.fail(err) } else { m.addOutput("Reply posted."); m.displayThread(m.draftThreadID, 1) }
	case composeEditBody:
		err := m.boards.UpdateThread(m.draftThreadID, m.user.ID, m.draftTitle, body, m.isModerator())
		if err != nil { m.fail(err) } else { m.addOutput("Topic updated."); m.displayThread(m.draftThreadID, 1) }
	case composeEditReplyBody:
		err := m.boards.UpdateComment(m.draftReplyID, m.user.ID, body, m.isModerator())
		if err != nil { m.fail(err) } else { m.addOutput("Reply updated."); m.displayThread(m.draftThreadID, 1) }
	}
	m.resetComposition()
}

func (m *sessionModel) resetComposition() {
	m.compose, m.draftTitle, m.draftLines = composeNone, "", nil
	m.draftThreadID, m.draftReplyID = 0, 0
	m.input.Prompt = m.currentPrompt()
}

func (m *sessionModel) cancelComposition() {
	m.addOutput("Cancelled.")
	m.resetComposition()
}

func (m *sessionModel) currentBoardID() (int, error) {
	if !strings.HasPrefix(m.cwd, "/boards/") { return 0, errors.New("not currently in a board") }
	slug := strings.TrimPrefix(m.cwd, "/boards/")
	if strings.Contains(slug, "/") { return 0, errors.New("not currently in a board") }
	board, err := m.boards.GetBoardBySlug(slug)
	return board.ID, err
}

func (m *sessionModel) deleteOwnTopic(args []string) {
	id, err := m.targetID(args)
	if err != nil { m.fail(err); return }
	if err := m.boards.DeleteThread(id, m.user.ID, false, ""); err != nil { m.fail(err); return }
	m.addOutput("Topic deleted.")
	if strings.HasSuffix(m.cwd, "/"+strconv.Itoa(id)) { m.cwd = path.Dir(m.cwd) }
}

func (m *sessionModel) deleteOwnReply(args []string) {
	if len(args) != 1 { m.addOutput("Usage: delete-reply <reply-id>"); return }
	id, err := strconv.Atoi(args[0]); if err != nil { m.addOutput("Invalid reply ID."); return }
	if err := m.boards.DeleteComment(id, m.user.ID, m.isModerator(), ""); err != nil { m.fail(err); return }
	m.addOutput("Reply deleted.")
}

func (m *sessionModel) moderateThread(args []string) {
	if !m.isModerator() { m.fail(forum.ErrPermissionDenied); return }
	if len(args) < 2 { m.addOutput("Usage: " + args[0] + " <topic-id> [reason]"); return }
	id, err := strconv.Atoi(args[1]); if err != nil { m.addOutput("Invalid topic ID."); return }
	reason := strings.TrimSpace(strings.Join(args[2:], " "))
	switch args[0] {
	case "lock": err = m.boards.SetThreadStatus(id, m.user.ID, forum.ThreadLocked, reason)
	case "unlock", "restore-thread": err = m.boards.SetThreadStatus(id, m.user.ID, forum.ThreadOpen, reason)
	case "hide-thread": err = m.boards.SetThreadStatus(id, m.user.ID, forum.ThreadHidden, reason)
	case "pin": err = m.boards.SetThreadPinned(id, m.user.ID, true, reason)
	case "unpin": err = m.boards.SetThreadPinned(id, m.user.ID, false, reason)
	case "delete-thread": err = m.boards.DeleteThread(id, m.user.ID, true, reason)
	}
	if err != nil { m.fail(err) } else { m.addOutput("Moderation action completed.") }
}

func (m *sessionModel) showModlog(args []string) {
	if !m.isModerator() { m.fail(forum.ErrPermissionDenied); return }
	entries, err := m.boards.ListModerationLog(100)
	if err != nil { m.fail(err); return }
	if len(entries) == 0 { m.addOutput("No moderation actions recorded."); return }
	for _, e := range entries {
		m.addOutput(fmt.Sprintf("%s  %s  %s #%d  by %s  %s", e.CreatedAt.Format("2006-01-02 15:04"), e.Action, e.TargetType, e.TargetID, e.Actor.Username, e.Reason))
	}
}

func (m *sessionModel) boardAdmin(args []string) {
	if !m.isAdmin() { m.fail(forum.ErrPermissionDenied); return }
	if len(args) == 0 { m.addOutput("Usage: board <create|update|archive|restore|list> ..."); return }
	switch args[0] {
	case "list":
		boards, err := m.boards.ListBoards(); if err != nil { m.fail(err); return }; m.printBoards(boards)
	case "create":
		if len(args) < 3 { m.addOutput(`Usage: board create <slug> "Name" ["Description"]`); return }
		description := ""; if len(args) > 3 { description = strings.Join(args[3:], " ") }
		if err := m.boards.CreateBoardWithDetails(args[1], args[2], description, 0, m.user.ID); err != nil { m.fail(err) } else { m.addOutput("Board created: " + args[1]) }
	case "update":
		if len(args) < 4 { m.addOutput(`Usage: board update <slug> "Name" "Description"`); return }
		if err := m.boards.UpdateBoard(args[1], args[2], strings.Join(args[3:], " ")); err != nil { m.fail(err) } else { m.addOutput("Board updated.") }
	case "archive", "restore":
		if len(args) != 2 { m.addOutput("Usage: board " + args[0] + " <slug>"); return }
		status := forum.BoardArchived; if args[0] == "restore" { status = forum.BoardActive }
		if err := m.boards.SetBoardStatus(args[1], status); err != nil { m.fail(err) } else { m.addOutput("Board status updated.") }
	default:
		m.addOutput("Unknown board subcommand.")
	}
}

func (m *sessionModel) inviteAdmin(args []string) {
	if !m.isAdmin() { m.fail(forum.ErrPermissionDenied); return }
	if len(args) == 0 { m.addOutput("Usage: invite <create|list|revoke>"); return }
	switch args[0] {
	case "list":
		invites, err := m.users.ListInvites(); if err != nil { m.fail(err); return }
		for _, i := range invites {
			status := "available"
			if i.RedeemedBy != "" { status = "redeemed by " + i.RedeemedBy }
			if !i.RevokedAt.IsZero() { status = "revoked" } else if time.Now().After(i.ExpiresAt) { status = "expired" }
			m.addOutput(fmt.Sprintf("%s  %-20s expires %s  admin=%t", i.Code, status, i.ExpiresAt.Format("2006-01-02 15:04"), i.IsAdmin))
		}
	case "create":
		duration := 7 * 24 * time.Hour
		admin := false
		for _, arg := range args[1:] {
			if arg == "--admin" { admin = true; continue }
			if d, err := time.ParseDuration(arg); err == nil { duration = d } else { m.addOutput("Invalid duration: " + arg); return }
		}
		if duration <= 0 { m.addOutput("Invite duration must be positive."); return }
		code, err := newInviteCode()
		if err == nil { err = m.users.CreateInvite(code, time.Now().Add(duration), admin) }
		if err != nil { m.fail(err) } else { m.addOutput("Invitation created: " + code); m.addOutput("Expires: " + time.Now().Add(duration).Format(time.RFC3339)) }
	case "revoke":
		if len(args) != 2 { m.addOutput("Usage: invite revoke <code>"); return }
		if err := m.users.RevokeInvite(args[1]); err != nil { m.fail(err) } else { m.addOutput("Invitation revoked.") }
	default:
		m.addOutput("Unknown invite subcommand.")
	}
}

func (m *sessionModel) listUsers() {
	if !m.isAdmin() { m.fail(forum.ErrPermissionDenied); return }
	users, err := m.users.ListUsers()
	if err != nil { m.fail(err); return }
	m.addOutput("USERNAME              ROLE       STATUS")
	for _, u := range users { m.addOutput(fmt.Sprintf("%-21s %-10s %s", u.Username, u.Role, u.Status)) }
}

func (m *sessionModel) userAdmin(args []string) {
	if !m.isAdmin() { m.fail(forum.ErrPermissionDenied); return }
	if len(args) != 2 && len(args) != 3 { m.addOutput("Usage: user role <username> <role> | user suspend|activate <username>"); return }
	var err error
	switch args[0] {
	case "role":
		if len(args) != 3 { m.addOutput("Usage: user role <username> <member|moderator|admin>"); return }
		err = m.users.SetUserRole(args[1], args[2])
	case "suspend":
		if len(args) != 2 { m.addOutput("Usage: user suspend <username>"); return }
		err = m.users.SetUserStatus(args[1], forum.StatusSuspended)
	case "activate":
		if len(args) != 2 { m.addOutput("Usage: user activate <username>"); return }
		err = m.users.SetUserStatus(args[1], forum.StatusActive)
	default:
		m.addOutput("Unknown user subcommand."); return
	}
	if err != nil { m.fail(err) } else { m.addOutput("User updated.") }
}

func (m *sessionModel) isModerator() bool {
	return m.user.Role == forum.RoleModerator || m.user.Role == forum.RoleAdmin || m.user.IsAdmin
}
func (m *sessionModel) isAdmin() bool { return m.user.Role == forum.RoleAdmin || m.user.IsAdmin }

func (m *sessionModel) targetID(args []string) (int, error) {
	if len(args) == 1 { id, err := strconv.Atoi(args[0]); if err == nil { return id, nil } }
	if len(args) == 0 { return m.currentThreadID(nil) }
	return 0, errors.New("usage: delete-topic [topic-id]")
}

func pageArgument(args []string) int {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--page" { n, err := strconv.Atoi(args[i+1]); if err == nil && n > 0 { return n } }
	}
	return 1
}

func relativeTime(t time.Time) string {
	if t.IsZero() { return "unknown" }
	d := time.Since(t)
	switch {
	case d < time.Minute: return "just now"
	case d < time.Hour: return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour: return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour: return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default: return t.Format("2006-01-02")
	}
}

func newInviteCode() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil { return "", err }
	return hex.EncodeToString(buf), nil
}

func splitCommand(line string) ([]string, error) {
	var result []string
	var current strings.Builder
	var quote rune
	escaped := false
	flush := func() { if current.Len() > 0 { result = append(result, current.String()); current.Reset() } }
	for _, r := range line {
		if escaped { current.WriteRune(r); escaped = false; continue }
		if r == '\\' && quote != 0 { escaped = true; continue }
		if quote != 0 {
			if r == quote { quote = 0 } else { current.WriteRune(r) }
			continue
		}
		switch r {
		case '\'', '"': quote = r
		case ' ', '\t': flush()
		default: current.WriteRune(r)
		}
	}
	if escaped { current.WriteRune('\\') }
	if quote != 0 { return nil, errors.New("unterminated quoted argument") }
	flush()
	return result, nil
}

