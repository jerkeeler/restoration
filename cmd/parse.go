package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jerkeeler/restoration/parser"
	"github.com/spf13/cobra"
)

var outputPath string
var quiet bool = false
var prettyPrint bool = false
var slim bool = false
var stats bool = false

// parseCmd represents the parse command
var parseCmd = &cobra.Command{
	Use:   "parse",
	Short: "Parses .mythrec files to human-readable json",
	Long:  `Parses .mythrec files to human-readable json`,
	Args:  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		if stats && slim {
			fmt.Fprintf(os.Stderr, "error: you cannot use stats and slim mode together\n")
			os.Exit(1)
			return
		}

		absPath, err := validateAndExpandPath(args[0])
		if err != nil {
			fmt.Printf("Error with filepath: %v\n", err)
			os.Exit(1)
			return
		}

		json, err := parser.ParseToJson(absPath, prettyPrint, slim, stats, isGzip)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
			return
		}
		if !quiet {
			fmt.Println(json)
		}

		if outputPath != "" && json != "" {
			slog.Debug("outputPath", "outputPath", outputPath)
			err = os.WriteFile(outputPath, []byte(json), 0644)
			if err != nil {
				fmt.Printf("Error writing to file: %v\n", err)
				os.Exit(1)
				return
			}
		}

		slog.Debug("Done parsing!")
	},
}

func init() {
	rootCmd.AddCommand(parseCmd)
	parseCmd.Flags().StringVarP(&outputPath, "output", "o", "", "Save the output JSON to the provided filepath")
	parseCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Quiet mode, no output to standard output")
	parseCmd.Flags().BoolVar(&prettyPrint, "pretty-print", false, "Pretty print the output JSON")
	parseCmd.Flags().BoolVar(&slim, "slim", false, "Slim mode, don't output game commands")
	parseCmd.Flags().BoolVar(
		&stats,
		"stats",
		false,
		"Stats mode, add stats to the output, you cannot use this with slim mode",
	)

	parseCmd.PreRun = func(cmd *cobra.Command, args []string) {
		if outputPath == "" {
			return
		}
		outputPath = filepath.Clean(outputPath)
		absPath, err := filepath.Abs(outputPath)
		if err != nil {
			return
		}
		outputPath = absPath
	}
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
