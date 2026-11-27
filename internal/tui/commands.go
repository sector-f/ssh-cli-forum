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
	permissionModerator
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

var commandGroupOrder = []string{"General", "Navigation", "Posting", "Moderation", "Administration"}

func commandRegistry() []command {
	return []command{
		{
			Name: "help", Aliases: []string{"?"}, Group: "General",
			Description: "Show available commands or detailed help for a command.",
			Arguments:   []commandArgument{optionalArgument("command [subcommand]", "Command to describe.")},
			Handler:     commandCall(func(m *sessionModel, args []string) { m.help(args) }),
		},
		{
			Name: "pwd", Group: "Navigation", Description: "Show the current location.",
			Handler: commandCallNoArgs(func(m *sessionModel) { m.addOutput(m.cwd) }),
		},
		{
			Name: "ls", Group: "Navigation", Description: "List boards or topics at a location.",
			Arguments: []commandArgument{{Name: "path", Description: "Location to list.", Optional: true, Completion: completionLocations}},
			Handler:   commandCall(func(m *sessionModel, args []string) { m.listLocation(args) }),
		},
		{
			Name: "cd", Group: "Navigation", Description: "Change the current location.",
			Arguments: []commandArgument{{Name: "path", Description: "Destination; use .. for the parent location.", Completion: completionLocations}},
			Handler:   commandCall(func(m *sessionModel, args []string) { m.changeDirectory(args) }),
		},
		{
			Name: "recent", Group: "Navigation", Description: "Show recently active topics.",
			Handler: commandCallNoArgs(func(m *sessionModel) { m.showRecent() }),
		},
		{
			Name: "topics", Group: "Navigation", Description: "List topics in the current board.",
			Arguments: []commandArgument{{Name: "--page N", Description: "Page number.", Optional: true, Candidates: []string{"--page"}}},
			Handler:   commandCall(func(m *sessionModel, args []string) { m.listTopics(args) }),
		},
		{
			Name: "show", Group: "Navigation", Description: "Display a topic and its replies.",
			Arguments: []commandArgument{
				{Name: "topic-id", Description: "Topic to display; defaults to the current topic.", Optional: true, Completion: completionTopicIDs},
				{Name: "--page N", Description: "Reply page number.", Optional: true, Candidates: []string{"--page"}},
			},
			Handler: commandCall(func(m *sessionModel, args []string) { m.showCurrent(args) }),
		},
		{
			Name: "whoami", Group: "General", Description: "Show your account name, role, and status.",
			Handler: commandCallNoArgs(func(m *sessionModel) {
				m.addOutput(fmt.Sprintf("%s (role: %s, status: %s)", m.user.Username, m.user.Role, m.user.Status))
			}),
		},
		{
			Name: "profile", Group: "General", Description: "Display your profile or another member's profile.",
			Arguments: []commandArgument{{Name: "username", Description: "Member to display; defaults to you.", Optional: true, Completion: completionUsernames}},
			Handler:   commandCall(func(m *sessionModel, args []string) { m.showProfile(args) }),
		},
		{
			Name: "chat", Group: "General", Description: "Enter the global live chat.",
			Handler: func(m *sessionModel, _ []string) (bool, tea.Cmd) { return false, m.enterChat() },
		},
		{
			Name: "new", Group: "Posting", Description: "Create a topic in the current board.",
			Handler: commandCallNoArgs(func(m *sessionModel) { m.startNewTopic() }),
		},
		{
			Name: "reply", Group: "Posting", Description: "Reply to the current topic.",
			Handler: commandCallNoArgs(func(m *sessionModel) { m.startReply() }),
		},
		{
			Name: "edit-topic", Group: "Posting", Description: "Edit a topic you authored.",
			Arguments: []commandArgument{{Name: "topic-id", Description: "Topic to edit; defaults to the current topic.", Optional: true, Completion: completionTopicIDs}},
			Handler:   commandCall(func(m *sessionModel, args []string) { m.startEditTopic(args) }),
		},
		{
			Name: "edit-reply", Group: "Posting", Description: "Edit a reply you authored in the current topic.",
			Arguments: []commandArgument{argument("reply-id", "Reply to edit.")},
			Handler:   commandCall(func(m *sessionModel, args []string) { m.startEditReply(args) }),
		},
		{
			Name: "delete-topic", Group: "Posting", Description: "Delete a topic you authored.",
			Arguments: []commandArgument{{Name: "topic-id", Description: "Topic to delete; defaults to the current topic.", Optional: true, Completion: completionTopicIDs}},
			Handler:   commandCall(func(m *sessionModel, args []string) { m.deleteOwnTopic(args) }),
		},
		{
			Name: "delete-reply", Group: "Posting", Description: "Delete a reply you authored (moderators may delete any reply).",
			Arguments: []commandArgument{argument("reply-id", "Reply to delete.")},
			Handler:   commandCall(func(m *sessionModel, args []string) { m.deleteOwnReply(args) }),
		},
		{
			Name: "lock", Group: "Moderation", Permission: permissionModerator,
			Description: "Lock a topic to prevent new replies.",
			Arguments:   []commandArgument{argument("topic-id", "Topic to lock"), {Name: "reason", Description: "Reason recorded in the moderation log.", Optional: true, Variadic: true}},
			Handler:     moderationStatusHandler(service.ThreadLocked),
		},
		{
			Name: "unlock", Group: "Moderation", Permission: permissionModerator,
			Description: "Reopen a locked topic.",
			Arguments:   []commandArgument{argument("topic-id", "Topic to reopen"), {Name: "reason", Description: "Reason recorded in the moderation log.", Optional: true, Variadic: true}},
			Handler:     moderationStatusHandler(service.ThreadOpen),
		},
		{
			Name: "pin", Group: "Moderation", Permission: permissionModerator,
			Description: "Pin a topic.",
			Arguments:   []commandArgument{argument("topic-id", "Topic to pin"), {Name: "reason", Description: "Reason recorded in the moderation log.", Optional: true, Variadic: true}},
			Handler:     moderationPinHandler(true),
		},
		{
			Name: "unpin", Group: "Moderation", Permission: permissionModerator,
			Description: "Unpin a topic.",
			Arguments:   []commandArgument{argument("topic-id", "Topic to unpin"), {Name: "reason", Description: "Reason recorded in the moderation log.", Optional: true, Variadic: true}},
			Handler:     moderationPinHandler(false),
		},
		{
			Name: "hide-thread", Group: "Moderation", Permission: permissionModerator,
			Description: "Hide a topic from ordinary member listings.",
			Arguments:   []commandArgument{argument("topic-id", "Topic to hide"), {Name: "reason", Description: "Reason recorded in the moderation log.", Optional: true, Variadic: true}},
			Handler:     moderationStatusHandler(service.ThreadHidden),
		},
		{
			Name: "restore-thread", Group: "Moderation", Permission: permissionModerator,
			Description: "Restore a hidden topic.",
			Arguments:   []commandArgument{argument("topic-id", "Topic to restore"), {Name: "reason", Description: "Reason recorded in the moderation log.", Optional: true, Variadic: true}},
			Handler:     moderationStatusHandler(service.ThreadOpen),
		},
		{
			Name: "delete-thread", Group: "Moderation", Permission: permissionModerator,
			Description: "Delete a topic, recording the reason in the moderation log.",
			Arguments:   []commandArgument{argument("topic-id", "Topic to delete"), {Name: "reason", Description: "Reason recorded in the moderation log.", Optional: true, Variadic: true}},
			Handler:     moderationDeleteHandler(),
		},
		{
			Name: "modlog", Group: "Moderation", Permission: permissionModerator,
			Description: "View recent moderation history.",
			Handler:     commandCall(func(m *sessionModel, args []string) { m.showModlog(args) }),
		},
		{
			Name: "board", Group: "Administration", Permission: permissionAdmin,
			Description: "Manage forum boards.",
			Subcommands: []command{
				{Name: "list", Description: "List boards.", Handler: commandCallNoArgs(func(m *sessionModel) { m.listAdminBoards() })},
				{Name: "create", Description: "Create a board.", Arguments: []commandArgument{argument("slug", "Unique board slug."), argument("name", "Display name."), optionalArgument("description", "Board description.")}, Handler: commandCall(func(m *sessionModel, args []string) { m.createAdminBoard(args) })},
				{Name: "update", Description: "Update a board's name and description.", Arguments: []commandArgument{argument("slug", "Board slug."), argument("name", "New display name."), argument("description", "New description.")}, Handler: commandCall(func(m *sessionModel, args []string) { m.updateAdminBoard(args) })},
				{Name: "archive", Description: "Archive a board.", Arguments: []commandArgument{{Name: "slug", Description: "Board slug.", Completion: completionBoardSlugs}}, Handler: commandCall(func(m *sessionModel, args []string) { m.setAdminBoardStatus(args, service.BoardArchived) })},
				{Name: "restore", Description: "Restore an archived board.", Arguments: []commandArgument{{Name: "slug", Description: "Board slug.", Completion: completionBoardSlugs}}, Handler: commandCall(func(m *sessionModel, args []string) { m.setAdminBoardStatus(args, service.BoardActive) })},
			},
		},
		{
			Name: "invite", Group: "Administration", Permission: permissionAdmin,
			Description: "Manage member invitations.",
			Subcommands: []command{
				{Name: "create", Description: "Create an invitation code.", Arguments: []commandArgument{optionalArgument("duration", "Validity period, such as 168h."), {Name: "--admin", Description: "Create an administrator invitation.", Optional: true, Candidates: []string{"--admin"}}}, Handler: commandCall(func(m *sessionModel, args []string) { m.createAdminInvite(args) })},
				{Name: "list", Description: "List invitation codes and their status.", Handler: commandCallNoArgs(func(m *sessionModel) { m.listAdminInvites() })},
				{Name: "revoke", Description: "Revoke an unused invitation code.", Arguments: []commandArgument{{Name: "code", Description: "Invitation code.", Completion: completionInviteCodes}}, Handler: commandCall(func(m *sessionModel, args []string) { m.revokeAdminInvite(args) })},
			},
		},
		{
			Name: "users", Group: "Administration", Permission: permissionAdmin,
			Description: "List registered members.",
			Handler:     commandCallNoArgs(func(m *sessionModel) { m.listUsers() }),
		},
		{
			Name: "user", Group: "Administration", Permission: permissionAdmin,
			Description: "Manage member roles and account status.",
			Subcommands: []command{
				{Name: "role", Description: "Change a member's role.", Arguments: []commandArgument{{Name: "username", Description: "Member to update.", Completion: completionUsernames}, {Name: "role", Description: "New role.", Candidates: []string{"admin", "member", "moderator"}}}, Handler: commandCall(func(m *sessionModel, args []string) { m.setAdminUserRole(args) })},
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

func moderationStatusHandler(status string) commandHandler {
	return commandCall(func(m *sessionModel, args []string) {
		m.applyModeration(args, func(id int, reason string) error {
			return m.boards.SetThreadStatus(m.ctx, id, m.user.ID, status, reason)
		})
	})
}

func moderationPinHandler(pinned bool) commandHandler {
	return commandCall(func(m *sessionModel, args []string) {
		m.applyModeration(args, func(id int, reason string) error {
			return m.boards.SetThreadPinned(m.ctx, id, m.user.ID, pinned, reason)
		})
	})
}

func moderationDeleteHandler() commandHandler {
	return commandCall(func(m *sessionModel, args []string) {
		m.applyModeration(args, func(id int, reason string) error {
			return m.boards.DeleteThread(m.ctx, id, m.user.ID, reason)
		})
	})
}

func lookupCommand(name string) (command, bool) {
	name = strings.ToLower(name)
	for _, candidate := range commandRegistry() {
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
	if current, err := m.users.GetUserByID(m.ctx, m.user.ID); err == nil {
		m.user = current
	}
	switch permission {
	case permissionModerator:
		return m.isModerator()
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
	case permissionModerator:
		return "moderator"
	case permissionAdmin:
		return "administrator"
	default:
		return "member"
	}
}

func commandNamesForUser(m *sessionModel) []string {
	var names []string
	for _, candidate := range commandRegistry() {
		if !m.hasPermission(candidate.Permission) {
			continue
		}
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
