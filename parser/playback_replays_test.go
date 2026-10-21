package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParsePlaybackOnReplays runs against real replays, which aren't checked in. Point RESTORATION_TEST_REPLAYS at
// a directory of .mythrec or .mythrec.gz files from the current build to run it.
func TestParsePlaybackOnReplays(t *testing.T) {
	dir := os.Getenv("RESTORATION_TEST_REPLAYS")
	if dir == "" {
		t.Skip("RESTORATION_TEST_REPLAYS not set")
	}
	paths, err := filepath.Glob(filepath.Join(dir, "*.mythrec*"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no replays found in %v: %v", dir, err)
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			playback, err := ParsePlayback(path, strings.HasSuffix(path, ".gz"))
			if err != nil {
				t.Fatalf("ParsePlayback() error = %v", err)
			}
			checkMap(t, playback.Map)
			checkEveryPlayerHasATownCenter(t, playback)
			checkEventsOnMap(t, playback)
		})
	}
}

func checkMap(t *testing.T, replayMap ReplayMap) {
	t.Helper()
	tiles := replayMap.Width * replayMap.Height
	if tiles == 0 || replayMap.TileSize <= 0 {
		t.Fatalf("map is %vx%v with tile size %v", replayMap.Width, replayMap.Height, replayMap.TileSize)
	}
	textures := decodeBase64(t, replayMap.TileTextures)
	if len(textures) != tiles {
		t.Errorf("TileTextures has %v tiles, want %v", len(textures), tiles)
	}
	for _, idx := range textures {
		if int(idx) >= len(replayMap.Textures) {
			t.Fatalf("TileTextures index %v outside palette of %v", idx, len(replayMap.Textures))
		}
	}
	if water := decodeBase64(t, replayMap.TileWater); len(water) != tiles {
		t.Errorf("TileWater has %v tiles, want %v", len(water), tiles)
	}
	vertices := (replayMap.Width + 1) * (replayMap.Height + 1)
	if heights := decodeBase64(t, replayMap.Heights); len(heights) != 4*vertices {
		t.Errorf("Heights has %v bytes, want 4 per vertex for %v vertices", len(heights), vertices)
	}
}

func checkEveryPlayerHasATownCenter(t *testing.T, playback ReplayPlayback) {
	t.Helper()
	townCenters := make(map[int]int)
	for _, entity := range playback.Entities {
		if entity.Name == "TownCenter" {
			townCenters[entity.PlayerNum]++
		}
	}
	for _, player := range playback.Players {
		if townCenters[player.PlayerNum] != 1 {
			t.Errorf("player %v has %v starting town centers, want 1", player.PlayerNum, townCenters[player.PlayerNum])
		}
	}
}

func checkEventsOnMap(t *testing.T, playback ReplayPlayback) {
	t.Helper()
	if len(playback.Events) == 0 {
		t.Fatal("no events")
	}
	worldWidth := float32(playback.Map.Width) * playback.Map.TileSize
	worldHeight := float32(playback.Map.Height) * playback.Map.TileSize
	for _, event := range playback.Events {
		if event.CommandType == "" {
			t.Errorf("event at %vs has no command type", event.GameTimeSecs)
		}
		if event.CommandType == "build" && (event.Name == "" || event.Location == nil) {
			t.Errorf("build at %vs is missing a name or location: %+v", event.GameTimeSecs, event)
		}
		if loc := event.Location; loc != nil && (loc.X > worldWidth || loc.Z > worldHeight) {
			t.Errorf("%v at %vs is off the %vx%v map: %+v", event.CommandType, event.GameTimeSecs, worldWidth, worldHeight, *loc)
		}
	}
}
