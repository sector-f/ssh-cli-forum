package tui

import (
	"strconv"
	"strings"
	"testing"

	"github.com/sector-f/ssh-cli-forum/internal/service"
)

func TestCommandRegistryMetadata(t *testing.T) {
	locations := []*pathNode{
		rootNode(),
		boardNode(service.Board{ID: 1, Slug: "general"}),
		topicNode(service.Board{ID: 1, Slug: "general"}, 12),
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
	member := &sessionModel{user: service.User{Role: service.RoleMember}, location: rootNode()}
	rootCommands := commandNamesForContext(member)
	rootNames := strings.Join(rootCommands, " ")
	for _, unavailable := range []string{"topics", "new", "reply", "delete-reply"} {
		if contains(rootCommands, unavailable) {
			t.Fatalf("root command list includes location-specific command %q: %s", unavailable, rootNames)
		}
	}
	if contains(rootCommands, "board") || contains(rootCommands, "invite") {
		t.Fatalf("member command list includes privileged commands: %s", rootNames)
	}
	if !contains(rootCommands, "show") || !contains(rootCommands, "delete-topic") || !contains(rootCommands, "recent") {
		t.Fatalf("root command list is missing global navigation, recent activity, or ID-based deletion: %s", rootNames)
	}

	member.location = boardNode(service.Board{ID: 1, Slug: "general"})
	boardCommandsForUser := commandNamesForContext(member)
	boardNames := strings.Join(boardCommandsForUser, " ")
	for _, available := range []string{"new"} {
		if !contains(boardCommandsForUser, available) {
			t.Fatalf("board command list is missing %q: %s", available, boardNames)
		}
	}
	if contains(boardCommandsForUser, "topics") || contains(boardCommandsForUser, "reply") || contains(boardCommandsForUser, "delete-reply") {
		t.Fatalf("board command list includes topic-only commands: %s", boardNames)
	}

	member.location = topicNode(service.Board{ID: 1, Slug: "general"}, 12)
	threadCommandsForUser := commandNamesForContext(member)
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
	member := &sessionModel{user: service.User{Role: service.RoleMember}, location: rootNode()}
	rootCommand, ok := member.lookupAvailableCommand("delete-topic")
	if !ok || requiredArguments(rootCommand) != 1 {
		t.Fatalf("global delete-topic should require an explicit ID: %+v, %t", rootCommand, ok)
	}

	member.location = topicNode(service.Board{ID: 1, Slug: "general"}, 12)
	threadCommand, ok := member.lookupAvailableCommand("delete-topic")
	if !ok || requiredArguments(threadCommand) != 0 {
		t.Fatalf("topic-local delete-topic should default to the current topic: %+v, %t", threadCommand, ok)
	}
	if threadCommand.Arguments[0].Completion != completionTopicIDs {
		t.Fatal("topic-local delete-topic should offer topic ID completion")
	}
}

func TestHelpUsesCommandsAvailableAtCurrentLocation(t *testing.T) {
	member := &sessionModel{user: service.User{Role: service.RoleMember}, location: rootNode()}
	member.help(nil)
	rootHelp := strings.Join(member.output, "\n")
	if !strings.Contains(rootHelp, "cd <path>") || !helpHasCommand(rootHelp, "show") {
		t.Fatalf("root help is missing global commands:\n%s", rootHelp)
	}
	for _, unavailable := range []string{"topics", "new", "reply", "delete-reply"} {
		if helpHasCommand(rootHelp, unavailable) {
			t.Fatalf("root help exposes location-specific command %q:\n%s", unavailable, rootHelp)
		}
	}

	member.output = nil
	member.location = boardNode(service.Board{ID: 1, Slug: "general"})
	member.help(nil)
	boardHelp := strings.Join(member.output, "\n")
	if !helpHasCommand(boardHelp, "new") || helpHasCommand(boardHelp, "topics") || helpHasCommand(boardHelp, "reply") {
		t.Fatalf("board help has incorrect commands:\n%s", boardHelp)
	}

	member.output = nil
	member.location = topicNode(service.Board{ID: 1, Slug: "general"}, 12)
	member.help(nil)
	threadHelp := strings.Join(member.output, "\n")
	if !strings.Contains(threadHelp, "reply") || !strings.Contains(threadHelp, "delete-reply") || !strings.Contains(threadHelp, "delete-topic") {
		t.Fatalf("topic help is missing topic commands:\n%s", threadHelp)
	}
	if helpHasCommand(threadHelp, "new") || helpHasCommand(threadHelp, "topics") {
		t.Fatalf("topic help exposes board-only commands:\n%s", threadHelp)
	}

	admin := &sessionModel{user: service.User{Role: service.RoleAdmin}, location: rootNode()}
	admin.help([]string{"board", "archive"})
	adminHelp := strings.Join(admin.output, "\n")
	if !strings.Contains(adminHelp, "Usage: board archive <slug>") || !strings.Contains(adminHelp, "Permission: administrator") {
		t.Fatalf("subcommand help is missing registry metadata:\n%s", adminHelp)
	}
}

func helpHasCommand(help, name string) bool {
	for _, line := range strings.Split(help, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == name {
			return true
		}
	}
	return false
}

func TestExecuteUsesAvailableHandlersAndPermissions(t *testing.T) {
	member := &sessionModel{user: service.User{Role: service.RoleMember}, location: boardsListNode()}
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


func TestAdminBoardListUsesFilesystemListing(t *testing.T) {
	root := rootNode()
	admin := &sessionModel{
		user: service.User{Role: service.RoleAdmin},
		boards: &testBoardService{boards: []service.BoardListing{
			{ID: 1, Slug: "general", Name: "General", Status: service.BoardActive},
			{ID: 2, Slug: "development", Name: "Development", Status: service.BoardActive},
		}},
		location: root,
		filesystemRoot: root,
	}

	admin.execute("board list")
	if got, want := strings.Join(admin.output, "\n"), "general/\ndevelopment/"; got != want {
		t.Fatalf("board list output = %q, want filesystem entries %q", got, want)
	}
}

func TestFilesystemCommandsSeparateListingFromDisplay(t *testing.T) {
	board := service.Board{
		ID: 1, Slug: "general", Name: "General",
		Description: "Board description", Status: service.BoardActive,
	}
	boardService := &testBoardService{
		boards: []service.BoardListing{{
			ID: board.ID, Slug: board.Slug, Name: board.Name,
			Description: board.Description, Status: board.Status,
		}},
		boardDetails: map[string]service.Board{"general": board},
		threads: map[int][]service.ThreadListing{
			1: {{ID: 42, Title: "Topic title"}},
		},
		threadDetails: map[int]service.Thread{
			42: {
				ID: 42, BoardID: 1, Title: "Topic title",
				Author: service.User{Username: "alice"}, Body: "Opening post",
			},
		},
		comments: map[int][]service.Comment{
			42: {{
				ID: 9, ThreadID: 42, Body: "A reply",
				Author: service.User{Username: "bob"},
			}},
		},
	}
	root := rootNode()
	member := &sessionModel{
		user: service.User{Role: service.RoleMember},
		boards: boardService,
		location: root,
		filesystemRoot: root,
	}

	member.execute("show")
	if got := strings.Join(member.output, "\n"); got != "Nothing to show at /." {
		t.Fatalf("show at root = %q, want a clear no-content message", got)
	}
	member.output = nil
	member.execute("show /boards general")
	if got := strings.Join(member.output, "\n"); got != "Usage: show [path]" {
		t.Fatalf("show with multiple arguments = %q, want usage error", got)
	}

	member.output = nil
	member.execute("ls")
	if got := strings.Join(member.output, "\n"); got != "boards/" {
		t.Fatalf("ls at root = %q, want only the immediate boards/ entry", got)
	}

	member.output = nil
	member.execute("cd /boards")
	if len(member.output) != 0 || member.currentLocation().Path() != "/boards" {
		t.Fatalf("cd should only change location: output=%v path=%q", member.output, member.currentLocation().Path())
	}
	member.execute("ls")
	if got := strings.Join(member.output, "\n"); got != "general/" {
		t.Fatalf("ls /boards = %q, want general/", got)
	}

	member.output = nil
	member.execute("show")
	boardsContent := strings.Join(member.output, "\n")
	for _, want := range []string{"SLUG", "general", "General", "active", "Board description"} {
		if !strings.Contains(boardsContent, want) {
			t.Errorf("show at /boards is missing %q:\n%s", want, boardsContent)
		}
	}

	member.output = nil
	member.execute("cd general")
	if len(member.output) != 0 || member.currentLocation().Path() != "/boards/general" {
		t.Fatalf("cd into board should not display it: output=%v path=%q", member.output, member.currentLocation().Path())
	}
	member.execute("show")
	boardContent := strings.Join(member.output, "\n")
	for _, want := range []string{"General — Board description", "Board status: active", "ID    TOPIC", "42", "Topic title"} {
		if !strings.Contains(boardContent, want) {
			t.Errorf("show on a board is missing %q:\n%s", want, boardContent)
		}
	}

	member.output = nil
	member.execute("ls")
	if got := strings.Join(member.output, "\n"); got != "42" {
		t.Fatalf("ls in board = %q, want topic entry 42", got)
	}

	member.output = nil
	member.execute("cd 42")
	if len(member.output) != 0 || member.currentLocation().Path() != "/boards/general/42" {
		t.Fatalf("cd into topic should only change location: output=%v path=%q", member.output, member.currentLocation().Path())
	}
	member.execute("show /boards/")
	boardsContent = strings.Join(member.output, "\n")
	if !strings.Contains(boardsContent, "SLUG") || !strings.Contains(boardsContent, "general") {
		t.Fatalf("show /boards/ from a topic should display the board listing:\n%s", boardsContent)
	}

	member.output = nil
	member.execute("show")
	topicContent := strings.Join(member.output, "\n")
	for _, want := range []string{"Topic title", "Opening post", "A reply"} {
		if !strings.Contains(topicContent, want) {
			t.Errorf("show on a topic is missing %q:\n%s", want, topicContent)
		}
	}
}

func TestCompletionUsesAvailableCommands(t *testing.T) {
	member := &sessionModel{user: service.User{Role: service.RoleMember}, location: rootNode()}
	rootCommands := member.completionCandidates(nil, "")
	if !contains(rootCommands, "pwd") || !contains(rootCommands, "recent") || contains(rootCommands, "new") || contains(rootCommands, "reply") {
		t.Fatalf("root completion has incorrect commands: %v", rootCommands)
	}

	member.location = boardNode(service.Board{ID: 1, Slug: "general"})
	boardCommandsForUser := member.completionCandidates(nil, "")
	if !contains(boardCommandsForUser, "new") || contains(boardCommandsForUser, "topics") || contains(boardCommandsForUser, "reply") {
		t.Fatalf("board completion has incorrect commands: %v", boardCommandsForUser)
	}

	member.location = topicNode(service.Board{ID: 1, Slug: "general"}, 12)
	threadCommandsForUser := member.completionCandidates(nil, "")
	if !contains(threadCommandsForUser, "reply") || !contains(threadCommandsForUser, "delete-reply") || contains(threadCommandsForUser, "new") {
		t.Fatalf("topic completion has incorrect commands: %v", threadCommandsForUser)
	}

	admin := &sessionModel{user: service.User{Role: service.RoleAdmin}, location: rootNode()}
	adminCommands := admin.completionCandidates(nil, "")
	if !contains(adminCommands, "board") || !contains(adminCommands, "invite") {
		t.Fatalf("admin completion is missing privileged commands: %v", adminCommands)
	}
	subcommands := admin.completionCandidates([]string{"board"}, "")
	if !contains(subcommands, "create") || !contains(subcommands, "archive") || contains(subcommands, "restore") {
		t.Fatalf("admin completion has incorrect board subcommands: %v", subcommands)
	}
}

func TestFilesystemPathTreeBuildsCanonicalPaths(t *testing.T) {
	root := rootNode()
	boards := boardsListNode()
	board := boardNode(service.Board{ID: 4, Slug: "general"})
	topic := topicNode(service.Board{ID: 4, Slug: "general"}, 12)

	if root.Path() != "/" || boards.Path() != "/boards" || board.Path() != "/boards/general" {
		t.Fatalf("unexpected paths: root=%q boards=%q board=%q", root.Path(), boards.Path(), board.Path())
	}
	if topic.Path() != "/boards/general/12" {
		t.Fatalf("unexpected canonical topic path: %q", topic.Path())
	}
	if topic.Parent().Path() != board.Path() {
		t.Fatal("topic parent was not preserved")
	}
	if board.Parent().Path() != "/boards" || boards.Parent().Path() != "/" {
		t.Fatal("path-node ancestry was not preserved")
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

func rootNode() *pathNode {
	return &pathNode{object: rootDirectory{}}
}

func boardsListNode() *pathNode {
	root := rootNode()
	return &pathNode{name: "boards", object: boardsDirectory{}, parent: root}
}

func boardNode(board service.Board) *pathNode {
	boards := boardsListNode()
	return &pathNode{name: board.Slug, object: boardObject{board: board}, parent: boards}
}

func topicNode(board service.Board, id int) *pathNode {
	boardPath := boardNode(board)
	return &pathNode{name: strconv.Itoa(id), object: topicObject{id: id}, parent: boardPath}
}
