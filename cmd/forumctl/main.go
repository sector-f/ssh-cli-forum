package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/charmbracelet/ssh"
	"github.com/sector-f/ssh-cli-forum/forum"
	"github.com/sector-f/ssh-cli-forum/sqlite"
	"go.uber.org/zap"

	"github.com/spf13/cobra"
)

var (
	dbPath string
	logger *zap.Logger
)

func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
	defer logger.Sync()

	var rootCmd = &cobra.Command{
		Use:   "forumctl",
		Short: "CLI tool for managing the forum database",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				return fmt.Errorf("--db flag is required")
			}
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "", "Path to sqlite database")

	// Invites
	var inviteCmd = &cobra.Command{Use: "invite", Short: "Manage invites"}
	var inviteList = &cobra.Command{
		Use:   "list",
		Short: "List all invites",
		RunE:  listInvites,
	}
	var inviteCreate = &cobra.Command{
		Use:   "create [code]",
		Short: "Create an invite",
		Args:  cobra.ExactArgs(1),
		RunE:  createInvite,
	}
	inviteCreate.Flags().Duration("duration", 7*24*time.Hour, "How long until invite expires")
	inviteCmd.AddCommand(inviteList, inviteCreate)
	rootCmd.AddCommand(inviteCmd)

	// Users
	var userCmd = &cobra.Command{Use: "user", Short: "Manage users"}
	var userList = &cobra.Command{
		Use:   "list",
		Short: "List all users",
		RunE:  listUsers,
	}
	var userCreate = &cobra.Command{
		Use:   "create [username] [pubkey]",
		Short: "Create a user",
		Args:  cobra.ExactArgs(2),
		RunE:  createUser,
	}
	userCreate.Flags().Bool("admin", false, "Make the new user an admin")
	userCmd.AddCommand(userList, userCreate)
	rootCmd.AddCommand(userCmd)

	// Boards
	var boardCmd = &cobra.Command{Use: "board", Short: "Manage boards"}
	var boardList = &cobra.Command{
		Use:   "list",
		Short: "List all boards",
		RunE:  listBoards,
	}
	var boardCreate = &cobra.Command{
		Use:   "create [name]",
		Short: "Create a board",
		Args:  cobra.ExactArgs(1),
		RunE:  createBoard,
	}
	boardCmd.AddCommand(boardList, boardCreate)
	rootCmd.AddCommand(boardCmd)

	// Threads
	var threadCmd = &cobra.Command{Use: "thread", Short: "Manage threads"}
	var threadList = &cobra.Command{
		Use:   "list",
		Short: "List threads (optionally filter by board ID)",
		RunE:  listThreads,
	}
	threadList.Flags().Int("board", 0, "Board ID to filter by")
	threadCmd.AddCommand(threadList)
	rootCmd.AddCommand(threadCmd)

	// Comments
	var commentCmd = &cobra.Command{Use: "comment", Short: "Manage comments"}
	var commentList = &cobra.Command{
		Use:   "list",
		Short: "List comments (optionally filter by thread ID)",
		RunE:  listComments,
	}
	commentList.Flags().Int("thread", 0, "Thread ID to filter by")
	commentCmd.AddCommand(commentList)
	rootCmd.AddCommand(commentCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

// === Invite Handlers ===

func listInvites(cmd *cobra.Command, args []string) error {
	s, err := sqlite.Open(logger, dbPath)
	if err != nil {
		return err
	}
	invites, err := s.ListInvites()
	if err != nil {
		return err
	}
	for _, i := range invites {
		fmt.Printf("%s | Created: %s | Expires: %s\n", i.Code, i.CreatedAt.Format(time.RFC3339), i.ExpiresAt.Format(time.RFC3339))
	}
	return nil
}

func createInvite(cmd *cobra.Command, args []string) error {
	s, err := sqlite.Open(logger, dbPath)
	if err != nil {
		return err
	}
	code := args[0]
	dur, err := cmd.Flags().GetDuration("duration")
	if err != nil {
		return err
	}
	expiresAt := time.Now().Add(dur)

	invite, err := s.CreateInvite(code, expiresAt)
	if err != nil {
		return err
	}
	fmt.Printf("Created invite: %s (expires %s)\n", invite.Code, invite.ExpiresAt.Format(time.RFC3339))
	return nil
}

// === User Handlers ===

func listUsers(cmd *cobra.Command, args []string) error {
	s, err := sqlite.Open(logger, dbPath)
	if err != nil {
		return err
	}
	users, err := s.ListUsers()
	if err != nil {
		return err
	}
	for _, u := range users {
		fmt.Printf("ID: %d | %s | Admin: %t | Invite: %v\n", u.ID, u.Username, u.IsAdmin, u.Invite)
	}
	return nil
}

func createUser(cmd *cobra.Command, args []string) error {
	s, err := sqlite.Open(logger, dbPath)
	if err != nil {
		return err
	}
	username := args[0]
	pubKeyString := args[1]

	pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubKeyString))
	if err != nil {
		return err
	}

	isAdmin, _ := cmd.Flags().GetBool("admin")
	user, err := s.CreateUser(username, pubKey, "", isAdmin)
	if err != nil {
		return err
	}
	fmt.Printf("Created user: %s (ID %d)\n", user.Username, user.ID)
	return nil
}

// === Board Handlers ===

func listBoards(cmd *cobra.Command, args []string) error {
	s, err := sqlite.Open(logger, dbPath)
	if err != nil {
		return err
	}
	boards, err := s.ListBoards()
	if err != nil {
		return err
	}
	for _, b := range boards {
		fmt.Printf("ID: %d | %s\n", b.ID, b.Name)
	}
	return nil
}

func createBoard(cmd *cobra.Command, args []string) error {
	s, err := sqlite.Open(logger, dbPath)
	if err != nil {
		return err
	}
	name := args[0]
	board, err := s.CreateBoard(name)
	if err != nil {
		return err
	}
	fmt.Printf("Created board: %s (ID %d)\n", board.Name, board.ID)
	return nil
}

// === Thread Handlers ===

func listThreads(cmd *cobra.Command, args []string) error {
	s, err := sqlite.Open(logger, dbPath)
	if err != nil {
		return err
	}
	boardID, _ := cmd.Flags().GetInt("board")
	var threads []*forum.Thread
	if boardID > 0 {
		threads, err = s.ListThreads(boardID)
	} else {
		// You could add a fallback to list all threads here if desired
		return fmt.Errorf("please provide --board to filter threads")
	}
	if err != nil {
		return err
	}
	for _, t := range threads {
		fmt.Printf("ID: %d | Title: %s | BoardID: %d | AuthorID: %d\n",
			t.ID, t.Title, t.Board.ID, t.Author.ID)
	}
	return nil
}

// === Comment Handlers ===

func listComments(cmd *cobra.Command, args []string) error {
	s, err := sqlite.Open(logger, dbPath)
	if err != nil {
		return err
	}
	threadID, _ := cmd.Flags().GetInt("thread")
	if threadID == 0 {
		return fmt.Errorf("please provide --thread to filter comments")
	}
	comments, err := s.ListComments(threadID)
	if err != nil {
		return err
	}
	for _, c := range comments {
		fmt.Printf("ID: %d | AuthorID: %d | Posted: %s\n%s\n\n",
			c.ID, c.Author.ID, c.PostedAt.Format(time.RFC3339), c.Body)
	}
	return nil
}
