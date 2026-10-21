package parser

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ===============================
// Common types
// ===============================

type RecString struct {
	value     string
	endOffset int
}

type NotL33t string

func (err NotL33t) Error() string {
	return string(err)
}

// Vector3 is a world-space position. X and Z span the map floor; Y is elevation.
type Vector3 struct {
	X float32
	Y float32
	Z float32
}

// ===============================
// Node and header types
// ===============================

type Node struct {
	token    string
	offset   int
	size     uint32
	parent   *Node
	children []*Node
}

func (node Node) endOffset() int {
	return node.offset + int(node.size) + DATA_OFFSET
}

func (node Node) path() string {
	/*
	   A string representing the "path" of the node based on its and its parents tokens. For example, if this node
	   has a token of JK and the parent is AB the path would be AB/JK.
	*/
	if node.parent == nil {
		return node.token
	}

	return node.parent.path() + "/" + node.token
}

func (node Node) getChildren(path ...string) []Node {
	/*
	   Get the children of this node that match the give path. Some paths have more than one node. For example, there
	   are multiple nodes with the XN/XN/XN path.
	*/
	if len(path) == 0 {
		return []Node{node}
	}

	nodes := make([]Node, 0)
	for _, child := range node.children {
		if child.token == path[0] {
			nodes = append(nodes, child.getChildren(path[1:]...)...)
		}
	}
	return nodes
}

func (node Node) getSingleChild(path ...string) (Node, error) {
	children := node.getChildren(path...)
	if len(children) == 0 {
		return Node{}, NoChildNodesError(strings.Join(path, "/"))
	}
	if len(children) > 1 {
		return Node{}, MultipleNodesError(strings.Join(path, "/"))
	}
	return children[0], nil
}

func (node Node) String() string {
	return node.path() + fmt.Sprintf(
		" -- offset=%s end_offset=%s size=%d children=%d",
		strconv.FormatInt(int64(node.offset), 16),
		strconv.FormatInt(int64(node.endOffset()), 16),
		node.size,
		len(node.children),
	)
}

type NoChildNodesError string

func (err NoChildNodesError) Error() string {
	return fmt.Sprintf("No child node found searching for: %v", string(err))
}

type MultipleNodesError string

func (err MultipleNodesError) Error() string {
	return fmt.Sprintf("Multiple child nodes found for %v, but expected only 1!", string(err))
}

type NotRootNodeError Node

func (node NotRootNodeError) Error() string {
	errString := fmt.Sprintf("%v is not a root node! Root node must be %v", node.token, ROOT_NODE_TOKEN)
	return errString
}

type CommandList struct {
	entryIdx  int
	offsetEnd int
	commands  []RawGameCommand
}

type FooterNotFoundError int

func (err FooterNotFoundError) Error() string {
	return fmt.Sprintf("Footer not found searching at offset=%v", int(err))
}

type UnkNotExpectedValueError int

func (err UnkNotExpectedValueError) Error() string {
	return fmt.Sprintf("The unknown byte in footer search did not equal 0 or 1 at offset %v", strconv.FormatInt(int64(err), 16))
}

// ===============================
// XMB types
// ===============================

type XmbFile struct {
	name   string
	offset int
	length uint32
}

type XmbNode struct {
	offset      int
	endOffset   int
	elementName string
	value       string
	attributes  map[string]string
	children    []*XmbNode
}

// =============================================================================================
// Replay formats, parser output, the human readable output, good for use in other applications
// =============================================================================================

type ReplayFormatted struct {
	MapName        string
	BuildNumber    int
	BuildString    string
	ParsedAt       time.Time
	ParserVersion  string
	GameLengthSecs float64
	GameSeed       int
	WinningTeam    int
	GameOptions    map[string]bool
	Players        []ReplayPlayer
	Stats          *map[int]ReplayStats // Map of player number to stats
	GameCommands   *[]ReplayGameCommand
}

type ReplayPlayer struct {
	PlayerNum int
	TeamId    int
	Name      string
	ProfileId int
	Color     int
	RandomGod bool
	God       string
	Winner    bool
	EAPM      float64
	MinorGods [3]string
	Titan     bool
	Wonder    bool
	CivList   string `json:"civ_list"`
}

type ReplayGameCommand struct {
	GameTimeSecs float64
	PlayerNum    int
	CommandType  string
	Payload      interface{}
}

type ReplayStats struct {
	Trade           TradeStats
	UnitCounts      map[string]int
	BuildingCounts  map[string]int
	GodPowerCounts  map[string]int
	FormationCounts map[string]int
	TauntCounts     map[string]int
	TechsResearched []string
	EAPM            []float64
	Timelines       Timelines
}

type TradeStats struct {
	ResourcesSold   map[string]float32
	ResourcesBought map[string]float32
}

type TechItem struct {
	Name         string
	GameTimeSecs float64
}

type GodPowerItem struct {
	Name         string
	GameTimeSecs float64
}

type Timelines struct {
	UnitCounts      []map[string]int
	BuildingCounts  []map[string]int
	TechsPrequeued  []TechItem
	TechsResearched []TechItem
	GodPowers       []GodPowerItem
}

// =============================================================================================
// Playback output: everything a web player needs to redraw the map and replay each command
// =============================================================================================

type ReplayPlayback struct {
	ParserVersion  string
	MapName        string
	BuildNumber    int
	GameLengthSecs float64
	Players        []ReplayPlayer
	Map            ReplayMap
	Entities       []ReplayEntity
	Events         []PlaybackEvent
}

// ReplayMap is the terrain at game start. Tile grids are base64 with one byte per tile, ordered x-major
// (index = tileX*Height + tileZ). World position = tile index * TileSize.
type ReplayMap struct {
	Width        int
	Height       int
	TileSize     float32
	Textures     []ReplayTexture // Palette for TileTextures, holding only the textures this map uses
	TileTextures string
	WaterTypes   []string // Palette for TileWater
	TileWater    string   // 255 = dry
	HeightMin    float32
	HeightMax    float32
	// Heights has one byte per vertex, (Width+1)*(Height+1) of them, x-major.
	// Elevation = HeightMin + byte/255 * (HeightMax - HeightMin).
	Heights string
}

type ReplayTexture struct {
	Name  string
	Group string // PassableLand, Water, Underwater, Shoreline, NonPassableLand, or Ice
}

type ReplayEntity struct {
	Id        int32
	PlayerNum int // 0 = Mother Nature
	Name      string
	X         float32
	Z         float32
}

type PlaybackEvent struct {
	GameTimeSecs float64
	PlayerNum    int
	CommandType  string
	Name         string   `json:",omitempty"` // Unit, building, tech, or power the command refers to
	Location     *Vector3 `json:",omitempty"`
	TargetId     *int32   `json:",omitempty"` // Entity a task targets, matching ReplayEntity.Id or a unit made later
	Units        []uint32 `json:",omitempty"` // Entities the command was issued to
}
