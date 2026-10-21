package parser

import (
	"encoding/base64"
	"reflect"
	"strings"
	"testing"
)

type terrainFixture struct {
	gridTokens  []string
	textureIdx  []uint16
	heightCount uint32
}

func validTerrainFixture() terrainFixture {
	return terrainFixture{
		gridTokens:  TERRAIN_GRIDS,
		textureIdx:  []uint16{1, 0, 0, 1},
		heightCount: 9,
	}
}

// build writes a 2x2 TN node: tile 2 is a cliff from the NonPassableLand group and tile 1 holds River water.
func (f terrainFixture) build() []byte {
	body := (&byteWriter{}).raw(make([]byte, 21)...).u32(3)
	body.str("PassableLand").u32(2).str("grass").str("sand")
	body.str("Water").u32(1).str("water")
	body.str("NonPassableLand").u32(1).str("cliff")
	body.u32(2).u32(2).f32(2).f32(10)

	types := (&byteWriter{}).u32(4).raw(0, 0, 2, 0).buf
	textures := (&byteWriter{}).u32(4)
	for _, idx := range f.textureIdx {
		textures.u16(idx)
	}
	waterNames := (&byteWriter{}).u32(0).u32(2).str("Lake").str("River").buf
	water := (&byteWriter{}).u32(4).raw(DRY_TILE, 1, DRY_TILE, DRY_TILE).buf
	grids := [][]byte{types, textures.buf, types, textures.buf, waterNames, water}
	for i, token := range f.gridTokens {
		body.node(token, grids[i])
	}

	body.u32(f.heightCount)
	for i := 0; i < 9; i++ {
		body.f32(float32(i) * 1.5)
	}
	return (&byteWriter{}).node("TN", body.buf).buf
}

func decodeBase64(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("invalid base64 %q: %v", s, err)
	}
	return b
}

func TestParseTerrain(t *testing.T) {
	data := validTerrainFixture().build()
	got, err := parseTerrain(&data, rootWithChild(data, "TN"))
	if err != nil {
		t.Fatalf("parseTerrain() error = %v", err)
	}

	if got.Width != 2 || got.Height != 2 || got.TileSize != 2 {
		t.Errorf("dims = %vx%v tile size %v, want 2x2 tile size 2", got.Width, got.Height, got.TileSize)
	}
	wantTextures := []ReplayTexture{
		{Name: "sand", Group: "PassableLand"},
		{Name: "grass", Group: "PassableLand"},
		{Name: "cliff", Group: "NonPassableLand"},
	}
	if !reflect.DeepEqual(got.Textures, wantTextures) {
		t.Errorf("Textures = %v, want %v", got.Textures, wantTextures)
	}
	if tiles := decodeBase64(t, got.TileTextures); !reflect.DeepEqual(tiles, []byte{0, 1, 2, 0}) {
		t.Errorf("TileTextures = %v, want [0 1 2 0]", tiles)
	}
	if !reflect.DeepEqual(got.WaterTypes, []string{"River"}) {
		t.Errorf("WaterTypes = %v, want [River]", got.WaterTypes)
	}
	if water := decodeBase64(t, got.TileWater); !reflect.DeepEqual(water, []byte{DRY_TILE, 0, DRY_TILE, DRY_TILE}) {
		t.Errorf("TileWater = %v, want [255 0 255 255]", water)
	}
	wantHeights := &byteWriter{}
	for i := 0; i < 9; i++ {
		wantHeights.f32(float32(i) * 1.5)
	}
	if heights := decodeBase64(t, got.Heights); !reflect.DeepEqual(heights, wantHeights.buf) {
		t.Errorf("Heights = %v, want float32 bytes %v", heights, wantHeights.buf)
	}
}

func TestParseTerrainErrors(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(f *terrainFixture)
		wantErr string
	}{
		{
			name:    "missing grid token",
			mutate:  func(f *terrainFixture) { f.gridTokens = []string{"TT", "TS", "PT", "PS", "XX", "WT"} },
			wantErr: "expected terrain grid WI",
		},
		{
			name:    "texture outside its group",
			mutate:  func(f *terrainFixture) { f.textureIdx = []uint16{1, 0, 1, 1} },
			wantErr: "outside group 2",
		},
		{
			name:    "height count mismatch",
			mutate:  func(f *terrainFixture) { f.heightCount = 8 },
			wantErr: "expected 9 terrain heights",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := validTerrainFixture()
			tt.mutate(&fixture)
			data := fixture.build()
			_, err := parseTerrain(&data, rootWithChild(data, "TN"))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("parseTerrain() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestParseTerrainMissingNode(t *testing.T) {
	data := validTerrainFixture().build()
	_, err := parseTerrain(&data, rootWithChild(data, "Z1"))
	if err == nil || !strings.Contains(err.Error(), "J1/TN") {
		t.Errorf("parseTerrain() error = %v, want missing J1/TN", err)
	}
}

func TestPaletteGrid(t *testing.T) {
	palette, grid, err := paletteGrid([]int{7, 3, 7, DRY_TILE, 3}, DRY_TILE)
	if err != nil {
		t.Fatalf("paletteGrid() error = %v", err)
	}
	if !reflect.DeepEqual(palette, []int{7, 3}) {
		t.Errorf("palette = %v, want [7 3]", palette)
	}
	if !reflect.DeepEqual(grid, []byte{0, 1, 0, DRY_TILE, 1}) {
		t.Errorf("grid = %v, want [0 1 0 255 1]", grid)
	}
}

func TestPaletteGridKeepsIndex255WhenNoEmptyValue(t *testing.T) {
	palette, grid, err := paletteGrid([]int{255, 4}, -1)
	if err != nil {
		t.Fatalf("paletteGrid() error = %v", err)
	}
	if !reflect.DeepEqual(palette, []int{255, 4}) || !reflect.DeepEqual(grid, []byte{0, 1}) {
		t.Errorf("paletteGrid() = %v, %v, want [255 4], [0 1]", palette, grid)
	}
}

func TestPaletteGridTooManyValues(t *testing.T) {
	indices := make([]int, DRY_TILE+1)
	for i := range indices {
		indices[i] = i + 1000
	}
	if _, _, err := paletteGrid(indices, -1); err == nil {
		t.Error("paletteGrid() error = nil, want error for more than 255 distinct values")
	}
}
