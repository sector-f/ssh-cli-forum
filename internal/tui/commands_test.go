package tui

import (
	"strings"
	"testing"

	"github.com/sector-f/ssh-cli-forum/internal/service"
)

func TestCommandRegistryMetadata(t *testing.T) {
	seen := make(map[string]bool)
	for _, cmd := range commandRegistry {
		if cmd.Name == "" || cmd.Description == "" || cmd.Group == "" {
			t.Errorf("command metadata is incomplete: %+v", cmd)
		}
		if seen[strings.ToLower(cmd.Name)] {
			t.Errorf("duplicate command name %q", cmd.Name)
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

func TestCommandPermissionsDriveVisibility(t *testing.T) {
	member := &sessionModel{user: service.User{Role: service.RoleMember}}
	moderator := &sessionModel{user: service.User{Role: service.RoleModerator}}
	admin := &sessionModel{user: service.User{Role: service.RoleAdmin}}

	memberNames := strings.Join(commandNamesForUser(member), " ")
	if strings.Contains(memberNames, "lock") || strings.Contains(memberNames, "board") || strings.Contains(memberNames, "invite") {
		t.Fatalf("member command list includes privileged commands: %s", memberNames)
	}

	moderatorNames := strings.Join(commandNamesForUser(moderator), " ")
	if !strings.Contains(moderatorNames, "lock") || strings.Contains(moderatorNames, "board") {
		t.Fatalf("moderator command list has incorrect permissions: %s", moderatorNames)
	}

	adminNames := strings.Join(commandNamesForUser(admin), " ")
	if !strings.Contains(adminNames, "board") || !strings.Contains(adminNames, "invite") || !strings.Contains(adminNames, "lock") {
		t.Fatalf("administrator command list is missing commands: %s", adminNames)
	}
}

func TestHelpIsGeneratedFromCommandRegistry(t *testing.T) {
	member := &sessionModel{user: service.User{Role: service.RoleMember}}
	member.help(nil)
	memberHelp := strings.Join(member.output, "\n")
	if !strings.Contains(memberHelp, "cd <path>") || !strings.Contains(memberHelp, "reply") {
		t.Fatalf("member help is missing ordinary commands:\n%s", memberHelp)
	}
	if strings.Contains(memberHelp, "lock <topic-id>") || strings.Contains(memberHelp, "board create") {
		t.Fatalf("member help exposes privileged commands:\n%s", memberHelp)
	}

	admin := &sessionModel{user: service.User{Role: service.RoleAdmin}}
	admin.help([]string{"board", "archive"})
	adminHelp := strings.Join(admin.output, "\n")
	if !strings.Contains(adminHelp, "Usage: board archive <slug>") || !strings.Contains(adminHelp, "Permission: administrator") {
		t.Fatalf("subcommand help is missing registry metadata:\n%s", adminHelp)
	}
}

func TestExecuteUsesRegisteredHandlerAndPermissions(t *testing.T) {
	member := &sessionModel{user: service.User{Role: service.RoleMember}, cwd: "/boards"}
	quit, _ := member.execute("pwd")
	if quit || len(member.output) != 1 || member.output[0] != "/boards" {
		t.Fatalf("pwd dispatch failed: quit=%t output=%v", quit, member.output)
	}

	quit, _ = member.execute("lock 42")
	if quit || !strings.Contains(strings.Join(member.output, "\n"), service.ErrPermissionDenied.Error()) {
		t.Fatalf("moderator command was not denied: quit=%t output=%v", quit, member.output)
	}

	quit, _ = member.execute("quit")
	if !quit {
		t.Fatal("quit alias did not invoke the exit handler")
	}
}

func TestCompletionUsesRegistryMetadata(t *testing.T) {
	member := &sessionModel{user: service.User{Role: service.RoleMember}}
	memberCommands := member.completionCandidates(nil, "")
	if !contains(memberCommands, "pwd") || contains(memberCommands, "lock") || contains(memberCommands, "board") {
		t.Fatalf("member completion has incorrect commands: %v", memberCommands)
	}

	admin := &sessionModel{user: service.User{Role: service.RoleAdmin}}
	adminCommands := admin.completionCandidates(nil, "")
	if !contains(adminCommands, "board") || !contains(adminCommands, "invite") {
		t.Fatalf("admin completion is missing privileged commands: %v", adminCommands)
	}
	subcommands := admin.completionCandidates([]string{"board"}, "")
	if !contains(subcommands, "create") || !contains(subcommands, "archive") {
		t.Fatalf("admin completion is missing board subcommands: %v", subcommands)
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
