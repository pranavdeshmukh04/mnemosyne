package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "0.1.0"

func main() {
	rootCmd := &cobra.Command{
		Use:   "mnemosyne",
		Short: "Local per-project memory system for Claude Code",
	}

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("mnemosyne v%s\n", version)
		},
	})

	rootCmd.Version = version

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
