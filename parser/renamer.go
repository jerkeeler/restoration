package parser

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

func RenameRecFiles(dir string, isGzip bool, prefix string, suffix string) error {
	slog.Info("Renaming replays in directory", "directory", dir, "isGzip", isGzip)

	// Determine file extension to search for
	extension := ".mythrec"
	if isGzip {
		extension += ".gz"
	}

	replayFiles := []string{}
	// Walk through directory
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip if not a file or doesn't have correct extension
		if info.IsDir() || !strings.HasSuffix(path, extension) {
			return nil
		}
		replayFiles = append(replayFiles, path)
		return nil
	})
	if err != nil {
		return err
	}

	// Create error channel and WaitGroup, increment wait group for each file, then wait for the waitgroup to finish
	errChan := make(chan error, len(replayFiles))
	var wg sync.WaitGroup

	slog.Debug("Found replay files", "numFiles", len(replayFiles))
	for _, file := range replayFiles {
		wg.Add(1)

		// Yay go concurrency! Huzzah! We can use this same method for replay parsing and output in the future
		go func(inputFilepath string) {
			defer wg.Done()

			replay, err := Parse(inputFilepath, true, false, isGzip)
			if err != nil {
				errChan <- fmt.Errorf("error parsing %s: %w", inputFilepath, err)
				return
			}

			playerNames := []string{}
			for _, player := range replay.Players {
				playerNames = append(playerNames, player.Name)
			}

			// Create base filename with player names
			baseFilename := strings.Join(playerNames, "_vs_")

			// Add prefix and suffix if provided
			if prefix != "" {
				baseFilename = prefix + baseFilename
			}
			if suffix != "" {
				baseFilename = baseFilename + suffix
			}

			// Add extension
			filename := baseFilename + extension
			newFilepath := filepath.Join(dir, filename)

			slog.Info("Renaming file",
				"oldPath", filepath.Base(inputFilepath),
				"newPath", filepath.Base(newFilepath),
			)
			if err := os.Rename(inputFilepath, newFilepath); err != nil {
				errChan <- fmt.Errorf("error renaming %s: %w", inputFilepath, err)
				return
			}
		}(file)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}
