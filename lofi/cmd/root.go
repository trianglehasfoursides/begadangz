package cmd

import "github.com/spf13/cobra"

var root = &cobra.Command{
	Use:   "begadangz",
	Short: "",
	Long:  "",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func Execute() {
	if err := root.Execute(); err != nil {
		panic(err.Error())
	}
}
