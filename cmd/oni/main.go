// Command oni is the OniWorks CLI — scaffold, migrate, serve, deploy, and more.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "oni",
	Short: "OniWorks CLI",
	Long: `oni — the OniWorks framework CLI

Scaffold, migrate, serve, and deploy your OniWorks application.
Run "oni help <command>" for details on any command.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func init() {
	// Core lifecycle
	rootCmd.AddCommand(newCmd, serveCmd, buildCmd, deployCmd)

	// Database
	rootCmd.AddCommand(migrateCmd, migrateRollbackCmd, migrateFreshCmd, migrateStatusCmd, dbSeedCmd)
	rootCmd.AddCommand(dbCreateCmd, dbDropCmd)

	// Routes & config
	rootCmd.AddCommand(routeListCmd, keyGenerateCmd)

	// Admin
	rootCmd.AddCommand(adminInstallCmd)

	// Queue & scheduler
	rootCmd.AddCommand(queueWorkCmd, queueRestartCmd, scheduleRunCmd)

	// Backup
	rootCmd.AddCommand(backupCmd, restoreCmd)

	// Ops
	rootCmd.AddCommand(healthCmd, docsServeCmd)

	// Secrets (group + colon-style aliases)
	rootCmd.AddCommand(secretsGroup, secretsSetCmd, secretsGetCmd)

	// make:* generators — registered both under the group AND directly
	rootCmd.AddCommand(
		makeGroup,
		makeControllerCmd,
		makeModelCmd,
		makeMigrationCmd,
		makeMiddlewareCmd,
		makeJobCmd,
		makeMailCmd,
		makeSeederCmd,
		makePolicyCmd,
		makeTestCmd,
		makeChannelCmd,
		makeResourceCmd,
	)
}
