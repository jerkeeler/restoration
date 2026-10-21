package parser

import (
	"encoding/base64"
	"fmt"
)

// TERRAIN_GRIDS are the sub-nodes of BG/J1/TN, in file order. Each is a token, uint32 size, uint32 element
// count, then the elements. PT and PS match TT and TS at game start.
var TERRAIN_GRIDS = []string{"TT", "TS", "PT", "PS", "WI", "WT"}

const DRY_TILE = 255

// parseTerrain decodes BG/J1/TN, the map's tile grids and heightmap at game start.
//
// TN layout (build 606844):
//
//	21 bytes     unknown header
//	uint32       texture group count (PassableLand, Water, Underwater, Shoreline, NonPassableLand, Ice)
//	per group    string name, uint32 count, count texture name strings
//	uint32 x2    width, height in tiles
//	float32 x2   tile size, unknown (10.0)
//	TT           uint8 texture group per tile
//	TS           uint16 texture per tile, indexing into the tile's group
//	PT, PS       copies of TT and TS
//	WI           uint32 unknown, uint32 count, count water type name strings
//	WT           uint8 water type per tile, indexing WI, 255 = dry
//	uint32       vertex count, then (width+1)*(height+1) float32 heights
func parseTerrain(data *[]byte, rootNode Node) (ReplayMap, error) {
	tn, err := rootNode.getSingleChild("J1", "TN")
	if err != nil {
		return ReplayMap{}, err
	}

	position := tn.offset + DATA_OFFSET + 21
	textures, groupStarts, position := parseTextureGroups(data, position)
	width := int(readUint32(data, position))
	height := int(readUint32(data, position+4))
	tileSize := readFloat(data, position+8)
	position += 16
	tiles := width * height

	grids := make(map[string][]byte)
	for _, token := range TERRAIN_GRIDS {
		if position+DATA_OFFSET > tn.endOffset() || string((*data)[position:position+2]) != token {
			return ReplayMap{}, fmt.Errorf("expected terrain grid %v at offset=%v", token, position)
		}
		end := position + DATA_OFFSET + int(readUint32(data, position+2))
		grids[token] = (*data)[position+DATA_OFFSET : end]
		position = end
	}
	if len(grids["TT"]) != 4+tiles || len(grids["TS"]) != 4+2*tiles || len(grids["WT"]) != 4+tiles {
		return ReplayMap{}, fmt.Errorf("terrain grid sizes don't match %vx%v tiles", width, height)
	}

	textureGrid := grids["TS"]
	textureIdx := make([]int, tiles)
	for i, group := range grids["TT"][4:] {
		if int(group) >= len(groupStarts)-1 {
			return ReplayMap{}, fmt.Errorf("tile %v has unknown texture group %v", i, group)
		}
		textureIdx[i] = groupStarts[group] + int(readUint16(&textureGrid, 4+2*i))
		if textureIdx[i] >= groupStarts[group+1] {
			return ReplayMap{}, fmt.Errorf("tile %v has texture %v outside group %v", i, textureIdx[i], group)
		}
	}
	textureSources, tileTextures, err := paletteGrid(textureIdx, -1)
	if err != nil {
		return ReplayMap{}, err
	}
	texturePalette := make([]ReplayTexture, len(textureSources))
	for i, idx := range textureSources {
		texturePalette[i] = textures[idx]
	}

	waterIdx := make([]int, tiles)
	for i, waterType := range grids["WT"][4:] {
		waterIdx[i] = int(waterType)
	}
	waterTable := grids["WI"]
	waterNames, _ := parseStringList(&waterTable, 4)
	waterSources, tileWater, err := paletteGrid(waterIdx, DRY_TILE)
	if err != nil {
		return ReplayMap{}, err
	}
	waterPalette := make([]string, len(waterSources))
	for i, idx := range waterSources {
		if idx >= len(waterNames) {
			return ReplayMap{}, fmt.Errorf("water type %v out of range for %v names", idx, len(waterNames))
		}
		waterPalette[i] = waterNames[idx]
	}

	heights, err := parseHeights(data, position, (width+1)*(height+1), tn.endOffset())
	if err != nil {
		return ReplayMap{}, err
	}
	heightMin, heightMax, quantized := quantizeHeights(heights)

	return ReplayMap{
		Width:        width,
		Height:       height,
		TileSize:     tileSize,
		Textures:     texturePalette,
		TileTextures: base64.StdEncoding.EncodeToString(tileTextures),
		WaterTypes:   waterPalette,
		TileWater:    base64.StdEncoding.EncodeToString(tileWater),
		HeightMin:    heightMin,
		HeightMax:    heightMax,
		Heights:      base64.StdEncoding.EncodeToString(quantized),
	}, nil
}

// parseTextureGroups flattens every group's textures into one list. groupStarts[g] is the index of group g's first
// texture, with one extra trailing entry holding the total.
func parseTextureGroups(data *[]byte, position int) ([]ReplayTexture, []int, int) {
	groupCount := int(readUint32(data, position))
	position += 4
	textures := make([]ReplayTexture, 0)
	groupStarts := make([]int, 0, groupCount+1)
	for i := 0; i < groupCount; i++ {
		groupStarts = append(groupStarts, len(textures))
		group := readString(data, position)
		var names []string
		names, position = parseStringList(data, group.endOffset)
		for _, name := range names {
			textures = append(textures, ReplayTexture{Name: name, Group: group.value})
		}
	}
	return textures, append(groupStarts, len(textures)), position
}

// parseStringList reads a uint32 count followed by that many strings.
func parseStringList(data *[]byte, position int) ([]string, int) {
	count := int(readUint32(data, position))
	position += 4
	values := make([]string, count)
	for i := range values {
		str := readString(data, position)
		values[i] = str.value
		position = str.endOffset
	}
	return values, position
}

func parseHeights(data *[]byte, position int, expected int, upperBound int) ([]float32, error) {
	count := int(readUint32(data, position))
	if count != expected || position+4+4*count > upperBound {
		return nil, fmt.Errorf("expected %v terrain heights at offset=%v, found count=%v", expected, position, count)
	}
	heights := make([]float32, count)
	for i := range heights {
		heights[i] = readFloat(data, position+4+4*i)
	}
	return heights, nil
}

// paletteGrid re-indexes a per-tile lookup into a palette of only the indices in use, in first-seen order, so each
// tile fits in one byte. It returns the palette's source indices. Tiles equal to emptyIdx encode as DRY_TILE; pass
// -1 when every tile has a value.
func paletteGrid(indices []int, emptyIdx int) ([]int, []byte, error) {
	palette := make([]int, 0)
	paletteIdx := make(map[int]byte)
	grid := make([]byte, len(indices))
	for i, idx := range indices {
		if idx == emptyIdx {
			grid[i] = DRY_TILE
			continue
		}
		if _, exists := paletteIdx[idx]; !exists {
			if len(palette) == DRY_TILE {
				return nil, nil, fmt.Errorf("more than %v distinct terrain names on one map", DRY_TILE)
			}
			paletteIdx[idx] = byte(len(palette))
			palette = append(palette, idx)
		}
		grid[i] = paletteIdx[idx]
	}
	return palette, grid, nil
}

// quantizeHeights scales heights into one byte each. Map elevations span ~20 world units, so the rounding
// error stays under 0.05, well below anything a minimap shows.
func quantizeHeights(heights []float32) (float32, float32, []byte) {
	heightMin, heightMax := heights[0], heights[0]
	for _, h := range heights {
		heightMin, heightMax = min(heightMin, h), max(heightMax, h)
	}
	quantized := make([]byte, len(heights))
	span := heightMax - heightMin
	if span == 0 {
		return heightMin, heightMax, quantized
	}
	for i, h := range heights {
		quantized[i] = byte((h-heightMin)/span*255 + 0.5)
	}
	return heightMin, heightMax, quantized
}
