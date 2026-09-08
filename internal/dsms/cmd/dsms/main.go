// Package main is the DSMS native CLI.
// Ultra-lightweight command-line interface for the
// Database Self-Management System + Intelligence Engine.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"cosca/internal/dsms/cli"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "dsms",
		Short: "DSMS — Database Self-Management System",
		Long: `DSMS — Database Self-Management System
Sistema de auto-gerenciamento de banco de dados + Intelligence Engine determinístico.
100% local, zero LLM, zero custo.`,
		Version: "1.0.0",
	}

	// Subcommands
	rootCmd.AddCommand(cli.TrainCmd())
	rootCmd.AddCommand(cli.ScanCmd())
	rootCmd.AddCommand(cli.RulesCmd())
	rootCmd.AddCommand(cli.ServeCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}