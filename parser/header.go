package parser

import (
	"fmt"
	"log/slog"
)

const OUTER_HIERARCHY_START_OFFSET = 257
const MAX_SCAN_OFFSET = 50
const DATA_OFFSET = 6
const ROOT_NODE_TOKEN = "BG"

var NODES_WITH_SUBSTRUCTURE = map[string]struct{}{
	"BG": {},
	"J1": {},
	"PL": {},
	"BP": {},
	"MP": {},
	"GM": {},
	"GD": {},
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

type Node struct {
	token    string
	offset   int
	size     uint32
	parent   *Node
	children []*Node
}

func ParseHeader(data *[]byte) Node {
	rootNode := newNode(data, OUTER_HIERARCHY_START_OFFSET)
	slog.Debug("Parsing header tree")
	createTree(data, &rootNode)
	// printTree(rootNode)
	return rootNode
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
		" -- offset=%d end_offset=%d size=%d children=%d",
		node.offset,
		node.endOffset(),
		node.size,
		len(node.children),
	)
}

func printTree(node Node) {
	/*
	   Prints the tree structure of the node and its children. Useful for debugging.
	*/
	slog.Debug(node.String())
	for _, child := range node.children {
		printTree(*child)
	}
}

func newNode(data *[]byte, offset int) Node {
	/*
		Creates a new Node by reading in the token and data length at a given offset. Createas
		a Node with default values of nil parent and no children.
	*/
	derefedData := *data
	token := string(derefedData[offset : offset+2])
	dataLength := readUint32(data, offset+2)
	return Node{
		token,
		offset,
		dataLength,
		nil,
		make([]*Node, 0),
	}
}

func createTree(data *[]byte, parentNode *Node) {
	/*
	   Recursively build up the header tree using a breadth first search approach.
	*/
	position := parentNode.offset + 6
	for position < parentNode.endOffset() {
		nextNodeLoc := findTwoLetterSeq(data, position, parentNode.endOffset())
		if nextNodeLoc == -1 {
			break
		}

		position = nextNodeLoc
		childNode := newNode(data, position)
		childNode.parent = parentNode
		parentNode.children = append(parentNode.children, &childNode)
		position = childNode.endOffset()
	}

	for _, child := range parentNode.children {
		if _, exists := NODES_WITH_SUBSTRUCTURE[child.token]; exists {
			createTree(data, child)
		}
	}

}

func findTwoLetterSeq(data *[]byte, offset int, upperBound int) int {
	/*
	   Sequential searches for the sequence b"<ASCII><ASCII>" starting at the given offset.
	   This is done by scanning one byte at a time.
	*/
	derefedData := *data
	if upperBound == -1 {
		upperBound = len(derefedData)
	}

	if data == nil || upperBound-offset < 2 || offset >= len(derefedData) {
		return -1
	}

	position := offset
	byte1 := derefedData[position]
	byte2 := derefedData[position+1]

	for {
		if position >= upperBound {
			break
		}

		if areBytesValidTokens(byte1, byte2) {
			return position
		}

		position += 1
		if position+1 > len(derefedData)-1 {
			break
		}
		byte1 = byte2
		byte2 = derefedData[position+1]

		if position > offset+MAX_SCAN_OFFSET {
			break
		}

	}
	return -1
}

func areBytesValidTokens(bytes ...byte) bool {
	/*
		Determine whether any byte sequence is a valid Node token. Although there are no restrictions
		on this function, a valid token must only be two bytes.
	*/
	allValid := true
	for _, b := range bytes {
		allValid = allValid && validAlphaNumericAscii(b)
	}
	return allValid
}

func validAlphaNumericAscii(char byte) bool {
	/*
		Verifies whether a given byte is either: uppercase ASCII, lowercase ASCII, or ASCII digit
	*/
	if char >= 65 && char <= 90 {
		// Uppercase ASCII
		return true
	} else if char >= 97 && char <= 122 {
		// Lowercase ASCII
		return true
	} else if char >= 48 && char <= 57 {
		// ASCII digit
		return true
	}
	return false
}
