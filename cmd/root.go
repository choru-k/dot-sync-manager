package cmd

import (
	"github.com/spf13/cobra"
)

var (
	configFile string
	verbose    bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "dsm",
	Short: "Dotfile Sync Manager",
	Long: `Dotfile Sync Manager (DSM) automatically syncs dotfiles between machines
using Git. Manage your dotfiles with simple commands.

Use "dsm help <command>" for more information about a command.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "Path to configuration file")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
}
