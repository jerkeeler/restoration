package parser

import (
	"log/slog"
)

func parseHeader(data *[]byte) Node {
	rootNode := newNode(data, 0)
	slog.Debug("Parsing header tree")
	parseTree(data, &rootNode)
	// printTree(rootNode)
	return rootNode
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

func parseTree(data *[]byte, parentNode *Node) {
	/*
	   Recursively build up the header tree using a breadth first search approach.
	*/
	position := parentNode.offset + 6
	for position < parentNode.endOffset() {
		nextNodeLoc := findTwoLetterSeq(data, position, parentNode.endOffset())
		if nextNodeLoc == -1 {
			break
		}

		childNode := newNode(data, nextNodeLoc)
		childNode.parent = parentNode
		if childNode.endOffset() > parentNode.endOffset() || childNode.path() == "BG/GM/GD/uI" {
			position = nextNodeLoc + 1
		} else {
			parentNode.children = append(parentNode.children, &childNode)
			position = childNode.endOffset()
		}
	}

	for _, child := range parentNode.children {
		if _, exists := NODES_WITH_SUBSTRUCTURE[child.token]; exists {
			parseTree(data, child)
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
	if len(bytes) != 2 {
		return false
	}

	// For now, we are skipping the values "kL" as that is not a valid node, but it now appears directly after the
	// BG => GM => GD node, which was breaking the XMB parsing logic.
	// TODO: Figure out what kL actually is, probably some data identifier, showcasing data length or something like
	// that.
	if bytes[0] == 107 && bytes[1] == 76 {
		slog.Debug("Skipping token kL")
		return false
	}
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
