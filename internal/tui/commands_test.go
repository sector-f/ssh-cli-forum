package tui

import (
	"strings"
	"testing"

	"github.com/sector-f/ssh-cli-forum/internal/service"
)

func TestCommandRegistryMetadata(t *testing.T) {
	locations := []location{
		rootLocation{},
		boardLocation{board: service.Board{ID: 1, Slug: "general"}},
		threadLocation{id: 12, parent: boardLocation{board: service.Board{ID: 1, Slug: "general"}}},
	}
	for _, loc := range locations {
		model := &sessionModel{user: service.User{Role: service.RoleAdmin}, location: loc}
		seen := make(map[string]bool)
		for _, cmd := range model.contextCommands() {
			if cmd.Name == "" || cmd.Description == "" || cmd.Group == "" {
				t.Errorf("command metadata is incomplete: %+v", cmd)
			}
			if seen[strings.ToLower(cmd.Name)] {
				t.Errorf("duplicate command name %q at %s", cmd.Name, loc.Path())
			}
			seen[strings.ToLower(cmd.Name)] = true
			if len(cmd.Subcommands) == 0 && cmd.Handler == nil {
				t.Errorf("command %q has no handler", cmd.Name)
			}
			for _, alias := range cmd.Aliases {
				if seen[strings.ToLower(alias)] {
					t.Errorf("duplicate command alias %q", alias)
				}
				seen[strings.ToLower(alias)] = true
			}
			for _, subcommand := range cmd.Subcommands {
				if subcommand.Name == "" || subcommand.Description == "" || subcommand.Handler == nil {
					t.Errorf("subcommand metadata is incomplete under %q: %+v", cmd.Name, subcommand)
				}
			}
		}
	}
}

func TestCommandAvailabilityFollowsLocation(t *testing.T) {
	member := &sessionModel{user: service.User{Role: service.RoleMember}, location: rootLocation{}}
	rootCommands := commandNamesForUser(member)
	rootNames := strings.Join(rootCommands, " ")
	for _, unavailable := range []string{"recent", "topics", "new", "reply", "delete-reply"} {
		if contains(rootCommands, unavailable) {
			t.Fatalf("root command list includes location-specific command %q: %s", unavailable, rootNames)
		}
	}
	if contains(rootCommands, "board") || contains(rootCommands, "invite") {
		t.Fatalf("member command list includes privileged commands: %s", rootNames)
	}
	if !contains(rootCommands, "show") || !contains(rootCommands, "delete-topic") {
		t.Fatalf("root command list is missing global navigation and ID-based deletion: %s", rootNames)
	}

	member.location = boardLocation{board: service.Board{ID: 1, Slug: "general"}}
	boardCommandsForUser := commandNamesForUser(member)
	boardNames := strings.Join(boardCommandsForUser, " ")
	for _, available := range []string{"topics", "new"} {
		if !contains(boardCommandsForUser, available) {
			t.Fatalf("board command list is missing %q: %s", available, boardNames)
		}
	}
	if contains(boardCommandsForUser, "reply") || contains(boardCommandsForUser, "delete-reply") {
		t.Fatalf("board command list includes topic-only commands: %s", boardNames)
	}

	member.location = threadLocation{id: 12, parent: boardLocation{board: service.Board{ID: 1, Slug: "general"}}}
	threadCommandsForUser := commandNamesForUser(member)
	threadNames := strings.Join(threadCommandsForUser, " ")
	for _, available := range []string{"reply", "delete-topic", "delete-reply"} {
		if !contains(threadCommandsForUser, available) {
			t.Fatalf("topic command list is missing %q: %s", available, threadNames)
		}
	}
	if contains(threadCommandsForUser, "new") || contains(threadCommandsForUser, "topics") {
		t.Fatalf("topic command list includes board-only commands: %s", threadNames)
	}
}

func TestContextualDeleteTopicShadowsGlobalCommand(t *testing.T) {
	member := &sessionModel{user: service.User{Role: service.RoleMember}, location: rootLocation{}}
	rootCommand, ok := member.lookupAvailableCommand("delete-topic")
	if !ok || requiredArguments(rootCommand) != 1 {
		t.Fatalf("global delete-topic should require an explicit ID: %+v, %t", rootCommand, ok)
	}

	member.location = threadLocation{id: 12, parent: boardLocation{board: service.Board{ID: 1, Slug: "general"}}}
	threadCommand, ok := member.lookupAvailableCommand("delete-topic")
	if !ok || requiredArguments(threadCommand) != 0 {
		t.Fatalf("topic-local delete-topic should default to the current topic: %+v, %t", threadCommand, ok)
	}
}

func TestHelpUsesCommandsAvailableAtCurrentLocation(t *testing.T) {
	member := &sessionModel{user: service.User{Role: service.RoleMember}, location: rootLocation{}}
	member.help(nil)
	rootHelp := strings.Join(member.output, "\n")
	if !strings.Contains(rootHelp, "cd <path>") || !strings.Contains(rootHelp, "show [topic-id]") {
		t.Fatalf("root help is missing global commands:\n%s", rootHelp)
	}
	for _, unavailable := range []string{"topics", "new", "reply", "delete-reply"} {
		if strings.Contains(rootHelp, unavailable) {
			t.Fatalf("root help exposes location-specific command %q:\n%s", unavailable, rootHelp)
		}
	}

	member.output = nil
	member.location = boardLocation{board: service.Board{ID: 1, Slug: "general"}}
	member.help(nil)
	boardHelp := strings.Join(member.output, "\n")
	if !strings.Contains(boardHelp, "new") || !strings.Contains(boardHelp, "topics") || strings.Contains(boardHelp, "reply") {
		t.Fatalf("board help has incorrect commands:\n%s", boardHelp)
	}

	member.output = nil
	member.location = threadLocation{id: 12, parent: boardLocation{board: service.Board{ID: 1, Slug: "general"}}}
	member.help(nil)
	threadHelp := strings.Join(member.output, "\n")
	if !strings.Contains(threadHelp, "reply") || !strings.Contains(threadHelp, "delete-reply") || !strings.Contains(threadHelp, "delete-topic") {
		t.Fatalf("topic help is missing topic commands:\n%s", threadHelp)
	}
	if strings.Contains(threadHelp, "new") || strings.Contains(threadHelp, "topics") {
		t.Fatalf("topic help exposes board-only commands:\n%s", threadHelp)
	}

	admin := &sessionModel{user: service.User{Role: service.RoleAdmin}, location: rootLocation{}}
	admin.help([]string{"board", "archive"})
	adminHelp := strings.Join(admin.output, "\n")
	if !strings.Contains(adminHelp, "Usage: board archive <slug>") || !strings.Contains(adminHelp, "Permission: administrator") {
		t.Fatalf("subcommand help is missing registry metadata:\n%s", adminHelp)
	}
}

func TestExecuteUsesAvailableHandlersAndPermissions(t *testing.T) {
	member := &sessionModel{user: service.User{Role: service.RoleMember}, location: boardsLocation{}}
	quit, _ := member.execute("pwd")
	if quit || len(member.output) != 1 || member.output[0] != "/boards" {
		t.Fatalf("pwd dispatch failed: quit=%t output=%v", quit, member.output)
	}

	quit, _ = member.execute("board list")
	if quit || !strings.Contains(strings.Join(member.output, "\n"), service.ErrPermissionDenied.Error()) {
		t.Fatalf("administrator command was not denied: quit=%t output=%v", quit, member.output)
	}

	quit, _ = member.execute("quit")
	if !quit {
		t.Fatal("quit alias did not invoke the exit handler")
	}
}

func TestCompletionUsesAvailableCommands(t *testing.T) {
	member := &sessionModel{user: service.User{Role: service.RoleMember}, location: rootLocation{}}
	rootCommands := member.completionCandidates(nil, "")
	if !contains(rootCommands, "pwd") || contains(rootCommands, "recent") || contains(rootCommands, "new") || contains(rootCommands, "reply") {
		t.Fatalf("root completion has incorrect commands: %v", rootCommands)
	}

	member.location = boardLocation{board: service.Board{ID: 1, Slug: "general"}}
	boardCommandsForUser := member.completionCandidates(nil, "")
	if !contains(boardCommandsForUser, "new") || !contains(boardCommandsForUser, "topics") || contains(boardCommandsForUser, "reply") {
		t.Fatalf("board completion has incorrect commands: %v", boardCommandsForUser)
	}

	member.location = threadLocation{id: 12, parent: boardLocation{board: service.Board{ID: 1, Slug: "general"}}}
	threadCommandsForUser := member.completionCandidates(nil, "")
	if !contains(threadCommandsForUser, "reply") || !contains(threadCommandsForUser, "delete-reply") || contains(threadCommandsForUser, "new") {
		t.Fatalf("topic completion has incorrect commands: %v", threadCommandsForUser)
	}

	admin := &sessionModel{user: service.User{Role: service.RoleAdmin}, location: rootLocation{}}
	adminCommands := admin.completionCandidates(nil, "")
	if !contains(adminCommands, "board") || !contains(adminCommands, "invite") {
		t.Fatalf("admin completion is missing privileged commands: %v", adminCommands)
	}
	subcommands := admin.completionCandidates([]string{"board"}, "")
	if !contains(subcommands, "create") || !contains(subcommands, "archive") || contains(subcommands, "restore") {
		t.Fatalf("admin completion has incorrect board subcommands: %v", subcommands)
	}
}

func TestTypedLocationPathsAndParents(t *testing.T) {
	root := rootLocation{}
	boards := boardsLocation{}
	board := boardLocation{board: service.Board{ID: 4, Slug: "general"}}
	topicFromBoard := threadLocation{id: 12, parent: board}
	recent := recentLocation{}
	topicFromRecent := threadLocation{id: 12, parent: recent}

	if root.Path() != "/" || boards.Path() != "/boards" || board.Path() != "/boards/general" {
		t.Fatalf("unexpected base paths: root=%q boards=%q board=%q", root.Path(), boards.Path(), board.Path())
	}
	if topicFromBoard.Path() != "/boards/general/12" || topicFromRecent.Path() != "/recent/12" {
		t.Fatalf("unexpected topic paths: board=%q recent=%q", topicFromBoard.Path(), topicFromRecent.Path())
	}
	if topicFromBoard.Parent().Path() != board.Path() || topicFromRecent.Parent().Path() != recent.Path() {
		t.Fatal("topic parent was not preserved")
	}
	if board.Parent().Path() != boards.Path() || recent.Parent().Path() != root.Path() {
		t.Fatal("collection parent was not preserved")
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
