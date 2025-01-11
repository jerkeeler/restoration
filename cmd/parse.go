package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jerkeeler/restoration/parser"
	"github.com/spf13/cobra"
)

// parseCmd represents the parse command
var parseCmd = &cobra.Command{
	Use:   "parse",
	Short: "Parses .mythrec files to human-readable json",
	Long:  `Parses .mythrec files to human-readable json`,
	Args:  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		absPath, err := validateAndExpandPath(args[0])
		if err != nil {
			fmt.Printf("Error with filepath: %v\n", err)
			return
		}
		err = parser.Parse(absPath)
		if err != nil {
			fmt.Printf("error: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(parseCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// parseCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// parseCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

type InvalidPath string

func (path InvalidPath) Error() string {
	return string(path)
}

func validateAndExpandPath(inputFilepath string) (string, error) {
	if inputFilepath == "" {
		return inputFilepath, InvalidPath("filepath is an empty string")
	}

	path := filepath.Clean(inputFilepath)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return inputFilepath, err
	}

	info, err := os.Stat(absPath)
	if err != nil && os.IsNotExist(err) {
		return inputFilepath, InvalidPath("file does not exist")
	} else if err != nil {
		return inputFilepath, err
	} else if info.IsDir() {
		return inputFilepath, InvalidPath("filepath is a directory!")
	}

	return absPath, nil
}
