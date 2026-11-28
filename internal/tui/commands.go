package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sector-f/ssh-cli-forum/internal/service"
)

type commandPermission uint8

const (
	permissionMember commandPermission = iota
	permissionAdmin
)

type completionSource uint8

const (
	completionNone completionSource = iota
	completionLocations
	completionBoardSlugs
	completionUsernames
	completionInviteCodes
	completionTopicIDs
)

type commandArgument struct {
	Name        string
	Description string
	Optional    bool
	Variadic    bool
	Candidates  []string
	Completion  completionSource
}

type commandHandler func(*sessionModel, []string) (bool, tea.Cmd)

type command struct {
	Name        string
	Aliases     []string
	Description string
	Group       string
	Permission  commandPermission
	Arguments   []commandArgument
	Subcommands []command
	Handler     commandHandler
}

func argument(name, description string) commandArgument {
	return commandArgument{Name: name, Description: description}
}

func optionalArgument(name, description string) commandArgument {
	return commandArgument{Name: name, Description: description, Optional: true}
}

func commandCall(fn func(*sessionModel, []string)) commandHandler {
	return func(m *sessionModel, args []string) (bool, tea.Cmd) {
		fn(m, args)
		return false, nil
	}
}

func commandCallNoArgs(fn func(*sessionModel)) commandHandler {
	return func(m *sessionModel, _ []string) (bool, tea.Cmd) {
		fn(m)
		return false, nil
	}
}

var commandGroupOrder = []string{"General", "Navigation", "Posting", "Board", "Topic", "Administration"}

func commandRegistry() []command {
	return []command{
		{
			Name: "help", Aliases: []string{"?"}, Group: "General",
			Description: "Show available commands or detailed help for a command.",
			Arguments: []commandArgument{optionalArgument("command [subcommand]", "Command to describe.")},
			Handler: commandCall(func(m *sessionModel, args []string) { m.help(args) }),
		},
		{
			Name: "pwd", Group: "Navigation", Description: "Show the current location.",
			Handler: commandCallNoArgs(func(m *sessionModel) { m.addOutput(m.currentLocation().Path()) }),
		},
		{
			Name: "ls", Group: "Navigation", Description: "List immediate entries in a directory.",
			Arguments: []commandArgument{{Name: "path", Description: "Location to list.", Optional: true, Completion: completionLocations}},
			Handler: commandCall(func(m *sessionModel, args []string) { m.listLocation(args) }),
		},
		{
			Name: "cd", Group: "Navigation", Description: "Change the current location.",
			Arguments: []commandArgument{{Name: "path", Description: "Destination; use .. for the parent location.", Completion: completionLocations}},
			Handler: commandCall(func(m *sessionModel, args []string) { m.changeDirectory(args) }),
		},
		{
			Name: "show", Group: "Navigation", Description: "Display content for the current resource or a path.",
			Arguments: []commandArgument{{Name: "path", Description: "Location to display; defaults to the current location.", Optional: true, Completion: completionLocations}},
			Handler: commandCall(func(m *sessionModel, args []string) { m.showLocation(args) }),
		},
		{
			Name: "recent", Group: "Navigation", Description: "List recently active topics.",
			Handler: commandCallNoArgs(func(m *sessionModel) { m.listRecentTopics() }),
		},
		{
			Name: "whoami", Group: "General", Description: "Show your account name, role, and status.",
			Handler: commandCallNoArgs(func(m *sessionModel) {
				m.addOutput(fmt.Sprintf("%s (role: %s, status: %s)", m.user.Username, m.user.Role, m.user.Status))
			}),
		},
		{
			Name: "chat", Group: "General", Description: "Enter the global live chat.",
			Handler: func(m *sessionModel, _ []string) (bool, tea.Cmd) { return false, m.enterChat() },
		},
		{
			Name: "delete-topic", Group: "Posting", Description: "Delete a topic you authored by ID.",
			Arguments: []commandArgument{{Name: "topic-id", Description: "Topic to delete.", Completion: completionTopicIDs}},
			Handler: commandCall(func(m *sessionModel, args []string) { m.deleteOwnTopic(args) }),
		},
		{
			Name: "board", Group: "Administration", Permission: permissionAdmin,
			Description: "Manage forum boards.",
			Subcommands: []command{
				{Name: "list", Description: "List boards.", Handler: commandCallNoArgs(func(m *sessionModel) { m.listAdminBoards() })},
				{Name: "create", Description: "Create a board.", Arguments: []commandArgument{argument("slug", "Unique board slug."), argument("name", "Display name."), optionalArgument("description", "Board description.")}, Handler: commandCall(func(m *sessionModel, args []string) { m.createAdminBoard(args) })},
				{Name: "archive", Description: "Archive a board.", Arguments: []commandArgument{{Name: "slug", Description: "Board slug.", Completion: completionBoardSlugs}}, Handler: commandCall(func(m *sessionModel, args []string) { m.setAdminBoardStatus(args, service.BoardArchived) })},
			},
		},
		{
			Name: "invite", Group: "Administration", Permission: permissionAdmin,
			Description: "Manage member invitations.",
			Subcommands: []command{
				{Name: "create", Description: "Create an invitation code.", Arguments: []commandArgument{optionalArgument("duration", "Validity period, such as 168h.")}, Handler: commandCall(func(m *sessionModel, args []string) { m.createAdminInvite(args) })},
				{Name: "list", Description: "List invitation codes and their status.", Handler: commandCallNoArgs(func(m *sessionModel) { m.listAdminInvites() })},
				{Name: "revoke", Description: "Revoke an unused invitation code.", Arguments: []commandArgument{{Name: "code", Description: "Invitation code.", Completion: completionInviteCodes}}, Handler: commandCall(func(m *sessionModel, args []string) { m.revokeAdminInvite(args) })},
			},
		},
		{
			Name: "users", Group: "Administration", Permission: permissionAdmin,
			Description: "List registered members.",
			Handler: commandCallNoArgs(func(m *sessionModel) { m.listUsers() }),
		},
		{
			Name: "user", Group: "Administration", Permission: permissionAdmin,
			Description: "Manage member account status.",
			Subcommands: []command{
				{Name: "suspend", Description: "Suspend a member account.", Arguments: []commandArgument{{Name: "username", Description: "Member to suspend.", Completion: completionUsernames}}, Handler: commandCall(func(m *sessionModel, args []string) { m.setAdminUserStatus(args, service.StatusSuspended) })},
				{Name: "activate", Description: "Activate a suspended member account.", Arguments: []commandArgument{{Name: "username", Description: "Member to activate.", Completion: completionUsernames}}, Handler: commandCall(func(m *sessionModel, args []string) { m.setAdminUserStatus(args, service.StatusActive) })},
			},
		},
		{
			Name: "exit", Aliases: []string{"quit"}, Group: "General",
			Description: "Disconnect from the SSH session.",
			Handler: func(m *sessionModel, _ []string) (bool, tea.Cmd) {
				m.addOutput("Disconnecting.")
				return true, nil
			},
		},
	}
}

func boardCommands() []command {
	return []command{
		{
			Name: "new", Group: "Board", Description: "Create a topic in the current board.",
			Handler: commandCallNoArgs(func(m *sessionModel) { m.startNewTopic() }),
		},
	}
}

func threadCommands() []command {
	return []command{
		{
			Name: "reply", Group: "Topic", Description: "Reply to the current topic.",
			Handler: commandCallNoArgs(func(m *sessionModel) { m.startReply() }),
		},
		{
			Name: "delete-topic", Group: "Topic", Description: "Delete the current topic or another topic by ID.",
			Arguments: []commandArgument{{Name: "topic-id", Description: "Topic to delete; defaults to the current topic.", Optional: true, Completion: completionTopicIDs}},
			Handler: commandCall(func(m *sessionModel, args []string) { m.deleteOwnTopic(args) }),
		},
		{
			Name: "delete-reply", Group: "Topic", Description: "Delete a reply you authored.",
			Arguments: []commandArgument{argument("reply-id", "Reply to delete.")},
			Handler: commandCall(func(m *sessionModel, args []string) { m.deleteOwnReply(args) }),
		},
	}
}

func lookupSubcommand(parent command, name string) (command, bool) {
	name = strings.ToLower(name)
	for _, candidate := range parent.Subcommands {
		if strings.EqualFold(candidate.Name, name) {
			return candidate, true
		}
	}
	return command{}, false
}

func (m *sessionModel) hasPermission(permission commandPermission) bool {
	// Keep command visibility and help in sync with administrative changes
	// made while this SSH session is connected. Authorization itself is
	// independently enforced by service.Session on every mutation.
	if m.users != nil {
		if current, err := m.users.GetUserByID(m.ctx, m.user.ID); err == nil {
			m.user = current
		}
	}
	switch permission {
	case permissionAdmin:
		return m.isAdmin()
	default:
		return true
	}
}

func effectivePermission(parent, child command) commandPermission {
	if child.Permission > parent.Permission {
		return child.Permission
	}
	return parent.Permission
}

func commandPermissionName(permission commandPermission) string {
	switch permission {
	case permissionAdmin:
		return "administrator"
	default:
		return "member"
	}
}

func (m *sessionModel) contextCommands() []command {
	commands := commandRegistry()
	local := m.currentLocation().object.Commands()

	// Location-local commands shadow same-named global commands. The topic
	// form of delete-topic defaults to the current topic; the global form
	// requires an explicit topic ID.
	for _, scoped := range local {
		replaced := false
		for i := range commands {
			if strings.EqualFold(commands[i].Name, scoped.Name) {
				commands[i] = scoped
				replaced = true
				break
			}
		}
		if !replaced {
			commands = append(commands, scoped)
		}
	}
	return commands
}

func (m *sessionModel) availableCommands() []command {
	commands := m.contextCommands()
	visible := make([]command, 0, len(commands))
	for _, candidate := range commands {
		if m.hasPermission(candidate.Permission) {
			visible = append(visible, candidate)
		}
	}
	return visible
}

func (m *sessionModel) lookupContextCommand(name string) (command, bool) {
	for _, candidate := range m.contextCommands() {
		if strings.EqualFold(candidate.Name, name) {
			return candidate, true
		}
		for _, alias := range candidate.Aliases {
			if strings.EqualFold(alias, name) {
				return candidate, true
			}
		}
	}
	return command{}, false
}

func (m *sessionModel) lookupAvailableCommand(name string) (command, bool) {
	for _, candidate := range m.availableCommands() {
		if strings.EqualFold(candidate.Name, name) {
			return candidate, true
		}
		for _, alias := range candidate.Aliases {
			if strings.EqualFold(alias, name) {
				return candidate, true
			}
		}
	}
	return command{}, false
}

func commandNamesForContext(m *sessionModel) []string {
	var names []string
	for _, candidate := range m.availableCommands() {
		names = append(names, candidate.Name)
		names = append(names, candidate.Aliases...)
	}
	return names
}

func commandUsage(cmd command, prefix string) string {
	name := cmd.Name
	if prefix != "" {
		name = prefix + " " + name
	}
	var parts []string
	for _, arg := range cmd.Arguments {
		value := arg.Name
		if arg.Variadic {
			value += "..."
		}
		if arg.Optional {
			value = "[" + value + "]"
		} else {
			value = "<" + value + ">"
		}
		parts = append(parts, value)
	}
	if len(parts) > 0 {
		name += " " + strings.Join(parts, " ")
	}
	return name
}

func requiredArguments(cmd command) int {
	required := 0
	for _, arg := range cmd.Arguments {
		if !arg.Optional && !arg.Variadic {
			required++
		}
	}
	return required
}
