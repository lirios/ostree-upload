// SPDX-FileCopyrightText: 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/lirios/ostree-upload/internal/logger"
	"github.com/lirios/ostree-upload/internal/ostree"
	"github.com/lirios/ostree-upload/internal/push"
	"github.com/lirios/ostree-upload/internal/receiver"
)

// Generate token command
func genTokenCmd() *cobra.Command {
	var (
		configPath string
		verbose    bool
	)

	var cmd = &cobra.Command{
		Use:   "gentoken",
		Short: "Creates a new API token",
		Long:  "Generates a token that gives access to the API.",
		Run: func(cmd *cobra.Command, args []string) {
			// Toggle debug output
			logger.SetVerbose(verbose)

			// Validate arguments
			if len(configPath) == 0 {
				logger.Fatal("Path to configuration file is mandatory")
				return
			}

			// Open configuration file
			config, err := receiver.CreateConfig(configPath)
			if err != nil {
				logger.Fatalf("Cannot open configuration file: %v", err)
				return
			}

			// Generate token
			token, err := receiver.GenerateToken()
			if err != nil {
				logger.Fatalf("Failed to generate token: %v", err)
				return
			}

			// Save token to the configuration
			config.Tokens = append(config.Tokens, token)
			if err := config.Save(); err != nil {
				logger.Fatalf("Cannot save configuration file: %v", err)
				return
			}

			// Print token
			logger.Infof("Token: %s", token.Token)
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "ostree-upload.yaml", "path to configuration file")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "more messages during the build")

	return cmd
}

// Receive command
func receiveCmd() *cobra.Command {
	var (
		bindAddress string
		configPath  string
		verbose     bool
		repoPath    string
	)

	var cmd = &cobra.Command{
		Use:   "receive",
		Short: "Start the server",
		Run: func(cmd *cobra.Command, args []string) {
			// Toggle debug output
			logger.SetVerbose(verbose)

			// Check if reposotory path exists
			if _, err := os.Stat(repoPath); os.IsNotExist(err) {
				logger.Fatalf("Path \"%s\" doesn't exist", repoPath)
				return
			}

			// Queue
			queue, err := receiver.NewQueue()
			if err != nil {
				logger.Fatalf("Failed to create queue: %v", err)
				return
			}

			// Open repository
			repo, err := ostree.OpenRepo(repoPath)
			if err != nil {
				logger.Fatalf("Failed to open OSTree repository: %v", err)
				return
			}

			// Create temporary directory
			if err = receiver.CreateTempDirectory(repo); err != nil {
				logger.Fatalf("Failed to create temporary directory for OSTree repository: %v", err)
				return
			}

			// Open configuration file
			config, err := receiver.OpenConfig(configPath)
			if err != nil {
				logger.Fatalf("Cannot open configuration file: %v", err)
				return
			}

			appState := &receiver.AppState{Queue: queue, Repo: repo, Config: config}
			if err := receiver.StartServer(bindAddress, appState); err != nil {
				logger.Fatal(err)
				return
			}
		},
	}

	cmd.Flags().StringVarP(&configPath, "config", "c", "ostree-upload.yaml", "path to configuration file")
	cmd.Flags().StringVarP(&bindAddress, "address", "a", ":8080", "host name and port to bind")
	cmd.Flags().StringVarP(&repoPath, "repo", "r", "repo", "path to OSTree repository")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "more messages during the build")

	return cmd
}

// Push command
func pushCmd() *cobra.Command {
	var (
		url      string
		repoPath string
		token    string
		branches []string
		verbose  bool
		prune    bool
	)

	var cmd = &cobra.Command{
		Use:   "push",
		Short: "Push objects to the remote OSTree repository",
		Run: func(cmd *cobra.Command, args []string) {
			// Toggle debug output
			logger.SetVerbose(verbose)

			// Check the token
			if len(token) == 0 {
				logger.Fatal("Token is mandatory")
				return
			}

			if err := push.StartClient(url, token, repoPath, branches, prune); err != nil {
				logger.Fatal(err)
				return
			}
		},
	}

	cmd.Flags().StringVarP(&url, "address", "a", "http://localhost:8080", "host name and port of the server")
	cmd.Flags().StringVarP(&repoPath, "repo", "r", "repo", "path to OSTree repository")
	cmd.Flags().StringVarP(&token, "token", "t", "", "token to authenticate with the server")
	cmd.Flags().BoolVarP(&prune, "prune", "", false, "prune repository before the transfer happens")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "more messages during the build")
	cmd.Flags().StringSliceVarP(&branches, "branch", "b", []string{}, "branch to upload")

	return cmd
}

// Execute executes the root command.
func Execute() error {
	// Root command
	var rootCmd = &cobra.Command{
		Use:   "ostree-upload",
		Short: "Transfer local OSTree objects to a remote repository",
	}

	rootCmd.AddCommand(
		genTokenCmd(),
		receiveCmd(),
		pushCmd(),
	)

	return rootCmd.Execute()
}
