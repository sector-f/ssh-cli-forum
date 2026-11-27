package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/ssh"
	"github.com/sector-f/ssh-cli-forum/internal/service"
	"github.com/sector-f/ssh-cli-forum/internal/storage/sqlite"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var dbPath string
var logger *zap.Logger

func main() {
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer logger.Sync()

	root := &cobra.Command{
		Use:          "forumctl",
		Short:        "Manage the SSH community forum database",
		SilenceUsage: true,
	}
	root.PersistentFlags().StringVar(&dbPath, "db", "./service.db", "Path to the SQLite database")

	root.AddCommand(dbCommand(), adminCommand(), boardCommand(), inviteCommand(), userCommand(), moderationCommand())
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func open() (*sqlite.Service, error) { return sqlite.Open(logger, dbPath) }

func withDB(fn func(*sqlite.Service) error) error {
	s, err := open()
	if err != nil {
		return err
	}
	defer s.Close()
	return fn(s)
}

func dbCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "db", Short: "Database operations"}
	cmd.AddCommand(&cobra.Command{
		Use: "migrate", Short: "Apply pending database migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withDB(func(s *sqlite.Service) error {
				versions, err := s.MigrationVersions()
				if err != nil {
					return err
				}
				fmt.Printf("Database is up to date. Applied migrations: %v\n", versions)
				return nil
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use: "status", Short: "Show database migration status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withDB(func(s *sqlite.Service) error {
				versions, err := s.MigrationVersions()
				if err != nil {
					return err
				}
				fmt.Printf("Database: %s\nApplied migrations: %v\n", dbPath, versions)
				return nil
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use: "backup <destination>", Short: "Create a consistent SQLite backup",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withDB(func(s *sqlite.Service) error {
				if err := s.Backup(args[0]); err != nil {
					return err
				}
				fmt.Println("Backup written to", args[0])
				return nil
			})
		},
	})
	return cmd
}

func adminCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "admin", Short: "Administrator bootstrap operations"}
	var username, keyFile string
	bootstrap := &cobra.Command{
		Use: "bootstrap", Short: "Create the initial administrator account",
		RunE: func(cmd *cobra.Command, args []string) error {
			if username == "" || keyFile == "" {
				return fmt.Errorf("--username and --public-key-file are required")
			}
			data, err := os.ReadFile(keyFile)
			if err != nil {
				return err
			}
			key, _, _, _, err := ssh.ParseAuthorizedKey(data)
			if err != nil {
				return fmt.Errorf("parse public key: %w", err)
			}
			return withDB(func(s *sqlite.Service) error {
				if err := s.BootstrapAdmin(username, key); err != nil {
					return err
				}
				fmt.Printf("Created initial administrator account: %s\n", username)
				return nil
			})
		},
	}
	bootstrap.Flags().StringVar(&username, "username", "", "Administrator username")
	bootstrap.Flags().StringVar(&keyFile, "public-key-file", "", "Path to an authorized_keys-format public key")
	cmd.AddCommand(bootstrap)
	return cmd
}

func boardCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "board", Short: "Manage forum boards"}
	create := &cobra.Command{
		Use: "create <slug> <name> [description]", Short: "Create a board",
		Args: cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			description := ""
			if len(args) == 3 {
				description = args[2]
			}
			sortOrder, _ := cmd.Flags().GetInt("sort-order")
			return withDB(func(s *sqlite.Service) error {
				if err := s.CreateBoardWithDetails(args[0], args[1], description, sortOrder, 0); err != nil {
					return err
				}
				fmt.Printf("Created board: %s (%s)\n", args[1], args[0])
				return nil
			})
		},
	}
	create.Flags().Int("sort-order", 0, "Board display order")
	list := &cobra.Command{
		Use: "list", Short: "List boards",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withDB(func(s *sqlite.Service) error {
				boards, err := s.ListBoards()
				if err != nil {
					return err
				}
				fmt.Printf("%-20s %-30s %-12s %s\n", "SLUG", "NAME", "STATUS", "DESCRIPTION")
				for _, b := range boards {
					fmt.Printf("%-20s %-30s %-12s %s\n", b.Slug, b.Name, b.Status, b.Description)
				}
				return nil
			})
		},
	}
	update := &cobra.Command{
		Use: "update <slug> <name> <description>", Short: "Update board metadata",
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withDB(func(s *sqlite.Service) error {
				if err := s.UpdateBoard(args[0], args[1], args[2]); err != nil {
					return err
				}
				fmt.Println("Board updated.")
				return nil
			})
		},
	}
	cmd.AddCommand(create, list, update, boardStatusCommand("archive", service.BoardArchived), boardStatusCommand("restore", service.BoardActive))
	return cmd
}

func boardStatusCommand(name, status string) *cobra.Command {
	return &cobra.Command{
		Use: name + " <slug>", Short: strings.Title(name) + " a board",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withDB(func(s *sqlite.Service) error {
				if err := s.SetBoardStatus(args[0], status); err != nil {
					return err
				}
				fmt.Printf("Board %s status set to %s.\n", args[0], status)
				return nil
			})
		},
	}
}

func inviteCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "invite", Short: "Manage invitation codes"}
	var duration time.Duration
	var admin bool
	create := &cobra.Command{
		Use: "create", Short: "Generate a new invitation code",
		RunE: func(cmd *cobra.Command, args []string) error {
			if duration <= 0 {
				return fmt.Errorf("--expires-in must be positive")
			}
			code, err := randomInvite()
			if err != nil {
				return err
			}
			expires := time.Now().Add(duration)
			return withDB(func(s *sqlite.Service) error {
				if err := s.CreateInvite(code, expires, admin); err != nil {
					return err
				}
				fmt.Printf("Invitation created\nCode:    %s\nExpires: %s\nAdmin:   %t\n", code, expires.UTC().Format(time.RFC3339), admin)
				return nil
			})
		},
	}
	create.Flags().DurationVar(&duration, "expires-in", 7*24*time.Hour, "Invitation lifetime (e.g. 168h)")
	create.Flags().BoolVar(&admin, "admin", false, "Grant administrator role on registration")
	list := &cobra.Command{
		Use: "list", Short: "List invitations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withDB(func(s *sqlite.Service) error {
				invites, err := s.ListInvites()
				if err != nil {
					return err
				}
				for _, i := range invites {
					status := "available"
					if i.RedeemedBy != "" {
						status = "redeemed by " + i.RedeemedBy
					}
					if !i.RevokedAt.IsZero() {
						status = "revoked"
					} else if time.Now().After(i.ExpiresAt) {
						status = "expired"
					}
					fmt.Printf("%s | %s | expires %s | admin=%t\n", i.Code, status, i.ExpiresAt.UTC().Format(time.RFC3339), i.IsAdmin)
				}
				return nil
			})
		},
	}
	revoke := &cobra.Command{
		Use: "revoke <code>", Short: "Revoke an invitation code",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withDB(func(s *sqlite.Service) error {
				if err := s.RevokeInvite(args[0]); err != nil {
					return err
				}
				fmt.Println("Invitation revoked.")
				return nil
			})
		},
	}
	cmd.AddCommand(create, list, revoke)
	return cmd
}

func userCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "user", Short: "Manage users"}
	cmd.AddCommand(&cobra.Command{
		Use: "list", Short: "List registered users",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withDB(func(s *sqlite.Service) error {
				users, err := s.ListUsers()
				if err != nil {
					return err
				}
				fmt.Printf("%-22s %-12s %-12s %s\n", "USERNAME", "ROLE", "STATUS", "JOINED")
				for _, u := range users {
					fmt.Printf("%-22s %-12s %-12s %s\n", u.Username, u.Role, u.Status, u.CreatedAt.Format("2006-01-02"))
				}
				return nil
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use: "set-role <username> <member|moderator|admin>", Short: "Change a user's role",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withDB(func(s *sqlite.Service) error {
				if err := s.SetUserRole(args[0], args[1]); err != nil {
					return err
				}
				fmt.Printf("Updated %s role to %s.\n", args[0], args[1])
				return nil
			})
		},
	})
	cmd.AddCommand(userStatusCommand("suspend", service.StatusSuspended), userStatusCommand("activate", service.StatusActive))
	return cmd
}

func userStatusCommand(name, status string) *cobra.Command {
	return &cobra.Command{
		Use: name + " <username>", Short: strings.Title(name) + " a user account",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withDB(func(s *sqlite.Service) error {
				if err := s.SetUserStatus(args[0], status); err != nil {
					return err
				}
				fmt.Printf("Updated %s status to %s.\n", args[0], status)
				return nil
			})
		},
	}
}

func moderationCommand() *cobra.Command {
	return &cobra.Command{
		Use: "modlog", Short: "Display recent moderation actions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withDB(func(s *sqlite.Service) error {
				entries, err := s.ListModerationLog(100)
				if err != nil {
					return err
				}
				for _, e := range entries {
					fmt.Printf("%s | %s | %s #%d | %s | %s\n", e.CreatedAt.Format(time.RFC3339), e.Action, e.TargetType, e.TargetID, e.Actor.Username, e.Reason)
				}
				return nil
			})
		},
	}
}

func randomInvite() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
