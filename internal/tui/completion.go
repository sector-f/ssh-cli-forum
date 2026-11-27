package tui

import (
	"sort"
	"strconv"
	"strings"
	"unicode"
)

func (m *sessionModel) completeInput() {
	line := m.input.Value()
	before, prefix, tokenStart, ok := completionContext(line, m.input.Cursor())
	if !ok {
		return
	}

	candidates := m.completionCandidates(before, prefix)
	if len(candidates) == 0 {
		return
	}

	replacement := ""
	addSpace := false
	if len(candidates) == 1 {
		replacement = candidates[0]
		addSpace = !strings.HasSuffix(replacement, "/")
	} else {
		replacement = longestCommonPrefix(candidates)
		if len([]rune(replacement)) <= len([]rune(prefix)) {
			m.addOutput(strings.Join(candidates, "  "))
			return
		}
	}

	runes := []rune(line)
	newLine := string(runes[:tokenStart]) + replacement
	if addSpace {
		newLine += " "
	}
	m.input.SetValue(newLine)
	m.input.SetCursor(len([]rune(newLine)))
}

// completionContext returns complete arguments before the token at the cursor,
// the current token prefix, and its rune offset in the original input.
func completionContext(line string, cursor int) ([]string, string, int, bool) {
	runes := []rune(line)
	if cursor < 0 || cursor != len(runes) {
		return nil, "", 0, false
	}

	start := cursor
	for start > 0 && !unicode.IsSpace(runes[start-1]) {
		start--
	}
	prefix := string(runes[start:cursor])
	before := strings.Fields(string(runes[:start]))
	return before, prefix, start, true
}

func (m *sessionModel) completionCandidates(before []string, prefix string) []string {
	if len(before) == 0 {
		return matchingCandidates(commandNamesForUser(m), prefix)
	}

	cmd, ok := lookupCommand(before[0])
	if !ok || !m.hasPermission(cmd.Permission) {
		return nil
	}

	precedingArgs := before[1:]
	if len(cmd.Subcommands) > 0 {
		if len(precedingArgs) == 0 {
			var names []string
			for _, subcommand := range cmd.Subcommands {
				if m.hasPermission(effectivePermission(cmd, subcommand)) {
					names = append(names, subcommand.Name)
				}
			}
			return matchingCandidates(names, prefix)
		}

		subcommand, found := lookupSubcommand(cmd, precedingArgs[0])
		if !found || !m.hasPermission(effectivePermission(cmd, subcommand)) {
			return nil
		}
		cmd = subcommand
		precedingArgs = precedingArgs[1:]
	}

	argumentIndex := len(precedingArgs)
	if argumentIndex >= len(cmd.Arguments) {
		return nil
	}

	arg := cmd.Arguments[argumentIndex]
	candidates := append([]string(nil), arg.Candidates...)
	switch arg.Completion {
	case completionLocations:
		candidates = append(candidates, m.locationCandidates(prefix)...)
	case completionBoardSlugs:
		candidates = append(candidates, m.boardSlugs()...)
	case completionUsernames:
		candidates = append(candidates, m.usernames()...)
	case completionInviteCodes:
		candidates = append(candidates, m.inviteCodes()...)
	case completionTopicIDs:
		candidates = append(candidates, m.topicIDs()...)
	}
	return matchingCandidates(candidates, prefix)
}

func (m *sessionModel) locationCandidates(prefix string) []string {
	boards, err := m.boards.ListBoards(m.ctx)
	if err != nil {
		return nil
	}

	absolute := []string{"/", "/boards", "/recent"}
	for _, board := range boards {
		base := "/boards/" + board.Slug
		absolute = append(absolute, base+"/")
		if topics, _, err := m.boards.ListThreadsPage(m.ctx, board.ID, 1, 50); err == nil {
			for _, topic := range topics {
				absolute = append(absolute, base+"/"+strconv.Itoa(topic.ID))
			}
		}
	}
	if strings.HasPrefix(prefix, "/") {
		return absolute
	}

	relative := []string{".."}
	switch {
	case m.cwd == "/":
		relative = append(relative, "boards/", "recent")
	case m.cwd == "/boards":
		for _, board := range boards {
			relative = append(relative, board.Slug+"/")
		}
	case strings.HasPrefix(m.cwd, "/boards/") && strings.Count(m.cwd, "/") == 2:
		slug := strings.TrimPrefix(m.cwd, "/boards/")
		for _, board := range boards {
			if board.Slug != slug {
				continue
			}
			topics, _, err := m.boards.ListThreadsPage(m.ctx, board.ID, 1, 50)
			if err != nil {
				break
			}
			for _, topic := range topics {
				relative = append(relative, strconv.Itoa(topic.ID))
			}
		}
	}
	return append(absolute, relative...)
}

func (m *sessionModel) boardSlugs() []string {
	boards, err := m.boards.ListBoards(m.ctx)
	if err != nil {
		return nil
	}
	result := make([]string, 0, len(boards))
	for _, board := range boards {
		result = append(result, board.Slug)
	}
	return result
}

func (m *sessionModel) usernames() []string {
	users, err := m.users.ListUsers(m.ctx)
	if err != nil {
		return nil
	}
	result := make([]string, 0, len(users))
	for _, user := range users {
		result = append(result, user.Username)
	}
	return result
}

func (m *sessionModel) inviteCodes() []string {
	invites, err := m.users.ListInvites(m.ctx)
	if err != nil {
		return nil
	}
	result := make([]string, 0, len(invites))
	for _, invite := range invites {
		if invite.RedeemedBy == "" && invite.RevokedAt.IsZero() {
			result = append(result, invite.Code)
		}
	}
	return result
}

func (m *sessionModel) topicIDs() []string {
	topics, err := m.boards.RecentThreads(m.ctx, 100)
	if err != nil {
		return nil
	}
	result := make([]string, 0, len(topics))
	for _, topic := range topics {
		result = append(result, strconv.Itoa(topic.ID))
	}
	return result
}

func matchingCandidates(candidates []string, prefix string) []string {
	matches := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	prefix = strings.ToLower(prefix)
	for _, candidate := range candidates {
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		if strings.HasPrefix(strings.ToLower(candidate), prefix) {
			matches = append(matches, candidate)
		}
	}
	sort.Strings(matches)
	return matches
}

func longestCommonPrefix(values []string) string {
	if len(values) == 0 {
		return ""
	}
	prefix := values[0]
	for _, value := range values[1:] {
		for !strings.HasPrefix(value, prefix) && prefix != "" {
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix
}
