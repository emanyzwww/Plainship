package cmd

import "github.com/spf13/cobra"

func NewRootCmd() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "psc",
		Short: "PaperShip Client",
		Run: func(cmd *cobra.Command, args []string) {
		},
	}

	return rootCmd
}
