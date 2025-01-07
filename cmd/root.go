package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "restoration",
	Short: "Parse and maninpulate Age of Mythology: Retold .mythrec files",
	Long: `A CLI parser of Age of Mythology: Retold .mythrec files. restoration's
main utility is parsing .mythrec files and output a large JSON file of the contents
to make the .mythrec more human readable and consumeable by other applications.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
}
