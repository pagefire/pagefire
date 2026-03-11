package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pagefire/pagefire/internal/app"
	"github.com/pagefire/pagefire/internal/auth"
	"github.com/pagefire/pagefire/internal/store"
	"github.com/pagefire/pagefire/internal/store/sqlite"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var version = "dev"

func main() {
	root := &cobra.Command{
		Use:     "pagefire",
		Short:   "On-call + monitoring + status pages in a single binary",
		Version: version,
	}

	serve := &cobra.Command{
		Use:   "serve",
		Short: "Start the PageFire server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := app.LoadConfig()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			a, err := app.New(cfg)
			if err != nil {
				return fmt.Errorf("initializing app: %w", err)
			}

			return a.Run(context.Background())
		},
	}

	adminCmd := &cobra.Command{
		Use:   "admin",
		Short: "Admin management commands",
	}

	adminCreate := &cobra.Command{
		Use:   "create",
		Short: "Create an admin user (interactive)",
		RunE:  runAdminCreate,
	}
	adminCreate.Flags().String("email", "", "Admin email address")
	adminCreate.Flags().String("name", "", "Admin display name")
	adminCreate.Flags().String("password", "", "Admin password (omit for interactive prompt)")

	adminCmd.AddCommand(adminCreate)
	root.AddCommand(serve, adminCmd)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func runAdminCreate(cmd *cobra.Command, args []string) error {
	cfg, err := app.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	s, err := sqlite.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer s.Close()

	if err := s.Migrate(context.Background()); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	reader := bufio.NewReader(os.Stdin)

	email, _ := cmd.Flags().GetString("email")
	if email == "" {
		fmt.Print("Email: ")
		email, _ = reader.ReadString('\n')
		email = strings.TrimSpace(email)
	}
	if email == "" {
		return fmt.Errorf("email is required")
	}

	name, _ := cmd.Flags().GetString("name")
	if name == "" {
		fmt.Print("Name: ")
		name, _ = reader.ReadString('\n')
		name = strings.TrimSpace(name)
	}
	if name == "" {
		return fmt.Errorf("name is required")
	}

	password, _ := cmd.Flags().GetString("password")
	if password == "" {
		fmt.Print("Password: ")
		pw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("reading password: %w", err)
		}
		password = string(pw)

		fmt.Print("Confirm password: ")
		pw2, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("reading password confirmation: %w", err)
		}
		if string(pw2) != password {
			return fmt.Errorf("passwords do not match")
		}
	}

	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	user := &store.User{
		Name:         name,
		Email:        email,
		Role:         store.RoleAdmin,
		Timezone:     "UTC",
		PasswordHash: hash,
		IsActive:     true,
	}

	if err := s.Users().Create(context.Background(), user); err != nil {
		return fmt.Errorf("creating user: %w", err)
	}

	fmt.Printf("Admin user created: %s (%s)\n", user.Name, user.Email)
	fmt.Printf("ID: %s\n", user.ID)
	return nil
}
