package cmd

import (
	"fmt"
	"os"

	"github.com/jerkeeler/restoration/parser"
	"github.com/spf13/cobra"
)

var (
	prefix string
	suffix string
)

var renameCmd = &cobra.Command{
	Use:   "rename [directory]",
	Short: "Renames all .mythrec (or .mythrec.gz) in a directory based on player names",
	Long: `This command will rename replay files in a directory based on the player names in the .mythrec file.

Only files ending in .mthyrec (or .mythrec.gz if the is-gzip flag is set) will be renamed. All other files will
be ignored. This will override the existing files in the directory.

You can optionally provide a prefix and/or suffix that will be added to the renamed files.
	`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		inputDir := args[0]

		// Validate directory exists
		if fileInfo, err := os.Stat(inputDir); err != nil || !fileInfo.IsDir() {
			fmt.Fprintf(os.Stderr, "error: '%s' is not a valid directory\n", inputDir)
			os.Exit(1)
		}

		err := parser.RenameRecFiles(inputDir, isGzip, prefix, suffix)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(renameCmd)
	renameCmd.Flags().StringVar(&prefix, "prefix", "", "Prefix to add to renamed files")
	renameCmd.Flags().StringVar(&suffix, "suffix", "", "Suffix to add to renamed files (before the extension)")
}
