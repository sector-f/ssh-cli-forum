package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

)

func (m *sessionModel) applyModeration(args []string, action func(int, string) error) {
	if len(args) < 1 {
		m.addOutput("A topic ID is required.")
		return
	}
	id, err := strconv.Atoi(args[0])
	if err != nil {
		m.addOutput("Invalid topic ID.")
		return
	}
	reason := strings.TrimSpace(strings.Join(args[1:], " "))
	if err := action(id, reason); err != nil {
		m.fail(err)
		return
	}
	m.addOutput("Moderation action completed.")
}

func (m *sessionModel) listAdminBoards() {
	boards, err := m.boards.ListBoards(m.ctx)
	if err != nil {
		m.fail(err)
		return
	}
	m.printBoards(boards)
}

func (m *sessionModel) createAdminBoard(args []string) {
	description := ""
	if len(args) > 2 {
		description = strings.Join(args[2:], " ")
	}
	if err := m.boards.CreateBoardWithDetails(m.ctx, args[0], args[1], description, 0, m.user.ID); err != nil {
		m.fail(err)
		return
	}
	m.addOutput("Board created: " + args[0])
}

func (m *sessionModel) updateAdminBoard(args []string) {
	if err := m.boards.UpdateBoard(m.ctx, args[0], args[1], strings.Join(args[2:], " ")); err != nil {
		m.fail(err)
		return
	}
	m.addOutput("Board updated.")
}

func (m *sessionModel) setAdminBoardStatus(args []string, status string) {
	if err := m.boards.SetBoardStatus(m.ctx, args[0], status); err != nil {
		m.fail(err)
		return
	}
	m.addOutput("Board status updated.")
}

func (m *sessionModel) listAdminInvites() {
	invites, err := m.users.ListInvites(m.ctx)
	if err != nil {
		m.fail(err)
		return
	}
	for _, invite := range invites {
		status := "available"
		if invite.RedeemedBy != "" {
			status = "redeemed by " + invite.RedeemedBy
		}
		if !invite.RevokedAt.IsZero() {
			status = "revoked"
		} else if time.Now().After(invite.ExpiresAt) {
			status = "expired"
		}
		m.addOutput(fmt.Sprintf("%s  %-20s expires %s  admin=%t", invite.Code, status, invite.ExpiresAt.Format("2006-01-02 15:04"), invite.IsAdmin))
	}
}

func (m *sessionModel) createAdminInvite(args []string) {
	duration := 7 * 24 * time.Hour
	admin := false
	for _, arg := range args {
		if arg == "--admin" {
			admin = true
			continue
		}
		d, err := time.ParseDuration(arg)
		if err != nil {
			m.addOutput("Invalid duration: " + arg)
			return
		}
		duration = d
	}
	if duration <= 0 {
		m.addOutput("Invite duration must be positive.")
		return
	}
	code, err := newInviteCode()
	if err == nil {
		err = m.users.CreateInvite(m.ctx, code, time.Now().Add(duration), admin)
	}
	if err != nil {
		m.fail(err)
		return
	}
	m.addOutput("Invitation created: " + code)
	m.addOutput("Expires: " + time.Now().Add(duration).Format(time.RFC3339))
}

func (m *sessionModel) revokeAdminInvite(args []string) {
	if err := m.users.RevokeInvite(m.ctx, args[0]); err != nil {
		m.fail(err)
		return
	}
	m.addOutput("Invitation revoked.")
}

func (m *sessionModel) setAdminUserRole(args []string) {
	if err := m.users.SetUserRole(m.ctx, args[0], args[1]); err != nil {
		m.fail(err)
		return
	}
	m.addOutput("User updated.")
}

func (m *sessionModel) setAdminUserStatus(args []string, status string) {
	if err := m.users.SetUserStatus(m.ctx, args[0], status); err != nil {
		m.fail(err)
		return
	}
	m.addOutput("User updated.")
}


func (m *sessionModel) showModlog(args []string) {
	entries, err := m.boards.ListModerationLog(m.ctx, 100)
	if err != nil {
		m.fail(err)
		return
	}
	if len(entries) == 0 {
		m.addOutput("No moderation actions recorded.")
		return
	}
	for _, e := range entries {
		m.addOutput(fmt.Sprintf("%s  %s  %s #%d  by %s  %s", e.CreatedAt.Format("2006-01-02 15:04"), e.Action, e.TargetType, e.TargetID, e.Actor.Username, e.Reason))
	}
}

func (m *sessionModel) listUsers() {
	users, err := m.users.ListUsers(m.ctx)
	if err != nil {
		m.fail(err)
		return
	}
	m.addOutput("USERNAME              ROLE       STATUS")
	for _, u := range users {
		m.addOutput(fmt.Sprintf("%-21s %-10s %s", u.Username, u.Role, u.Status))
	}
}
