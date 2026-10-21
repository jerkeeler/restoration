package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/jerkeeler/restoration/parser"
	"github.com/spf13/cobra"
)

var playbackCmd = &cobra.Command{
	Use:   "playback",
	Short: "Extracts the map and located commands from a .mythrec for match playback",
	Long: `Extracts what a web player needs to redraw a match as json: the terrain, heightmap, and every
object on the map at game start, plus every command with its game time, player, and map location.`,
	Args: cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		absPath, err := validateAndExpandPath(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error with filepath: %v\n", err)
			os.Exit(1)
		}

		json, err := parser.ParsePlaybackToJson(absPath, prettyPrint, isGzip)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		writeJson(json)
		slog.Debug("Done extracting playback!")
	},
}

func init() {
	rootCmd.AddCommand(playbackCmd)
	playbackCmd.Flags().StringVarP(&outputPath, "output", "o", "", "Save the output JSON to the provided filepath")
	playbackCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Quiet mode, no output to standard output")
	playbackCmd.Flags().BoolVar(&prettyPrint, "pretty-print", false, "Pretty print the output JSON")
	playbackCmd.PreRun = resolveOutputPath
}
