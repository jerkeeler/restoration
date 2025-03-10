package cmd

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

var isGzip bool = false

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
	verbose := false
	rootCmd.PersistentFlags().BoolVar(&isGzip, "is-gzip", false, "Indicates whether the input files are compressed with gzip")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		opts := &slog.HandlerOptions{
			Level: slog.LevelError,
		}
		if verbose {
			opts.Level = slog.LevelDebug
		}

		logger := slog.New(slog.NewTextHandler(os.Stdout, opts))
		slog.SetDefault(logger)
	}
}
