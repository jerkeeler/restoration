package parser

import (
	"fmt"
	"strconv"
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

type Vector3 struct {
	X int32
	Y int32
	Z int32
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

// ===============================
// GameCommand types
// ===============================

type GameCommand interface {
	CommandType() int
	OffsetEnd() int
	PlayerId() int
	ByteLength() int
	GameTimeSecs() float64
	AffectsEAPM() bool
}

type BaseCommand struct {
	commandType  int
	playerId     int
	offset       int
	offsetEnd    int
	byteLength   int
	gameTimeSecs float64
	affectsEAPM  bool
}

func (cmd BaseCommand) CommandType() int {
	return cmd.commandType
}

func (cmd BaseCommand) OffsetEnd() int {
	return cmd.offsetEnd
}

func (cmd BaseCommand) PlayerId() int {
	return cmd.playerId
}

func (cmd BaseCommand) ByteLength() int {
	return cmd.byteLength
}

func (cmd BaseCommand) GameTimeSecs() float64 {
	return cmd.gameTimeSecs
}

func (cmd BaseCommand) AffectsEAPM() bool {
	return cmd.affectsEAPM
}

type ResearchCommand struct {
	BaseCommand
	techId int32
}

type PrequeueTechCommand struct {
	BaseCommand
	techId int32
}

type TrainCommand struct {
	BaseCommand
	protoUnitId int32
	numUnits    int8
}

type AutoqueueCommand struct {
	BaseCommand
	protoUnitId int32
}

type BuildCommand struct {
	BaseCommand
	protoBuildingId int32
	queued          bool
	location        Vector3
}

type ProtoPowerCommand struct {
	BaseCommand
	protoPowerId int32
}

type CommandList struct {
	entryIdx     int
	offsetEnd    int
	finalCommand bool
	commands     []GameCommand
}

type FooterNotFoundError int

func (err FooterNotFoundError) Error() string {
	return fmt.Sprintf("Footer not found searching at offset=%v", int(err))
}

type UnkNotEqualTo1Error int

func (err UnkNotEqualTo1Error) Error() string {
	return fmt.Sprintf("The unknown byte in footer search did not equal 1 at offset %v", int(err))
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
	GameCommands   []ReplayGameCommand
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
}

type ReplayGameCommand struct {
	GameTimeSecs float64
	CommandType  string
	PlayerNum    int
	Value        string
}
