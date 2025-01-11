package parser

import (
	"errors"
	"fmt"
	"os"
)

func Parse(replayPath string) error {
	data, err := os.ReadFile(replayPath)
	if err != nil {
		return err
	}

	data, err = Decompressl33t(&data)
	if err != nil {
		return err
	}

	rootNode := ParseHeader(&data)

	buildString, err := readBuildString(&data, rootNode)
	if err != nil {
		return err
	}
	fmt.Println(buildString)

	_, err = parseProfileKeys(&data, rootNode)
	if err != nil {
		return err
	}
	// printProfileKeys(profileKeys)

	fmt.Println(77777, len(data))
	_, err = ParseCommandList(&data, rootNode.endOffset())
	if err != nil {
		return err
	}

	return nil
}

func isRootNode(node Node) bool {
	return node.token == ROOT_NODE_TOKEN
}

func readBuildString(data *[]byte, node Node) (string, error) {
	/*
	   Finds the FH node, then reads the string at the FH node offset to get the build information. There is other
	   information in the FH node, but I don't know what it is.
	*/
	if !isRootNode(node) {
		return "", NotRootNodeError(node)
	}
	children := node.getChildren("FH")
	if len(children) == 0 {
		return "", NoChildNodesError("FH")
	}
	if len(children) > 1 {
		return "", MultipleNodesError("FH")
	}

	fhNode := children[0]
	position := fhNode.offset + DATA_OFFSET
	return readString(data, position).value, nil

}

type ProfileKey struct {
	Type      string
	EndOffset int
	StringVal string
	Uint32Val uint32
	Int16Val  int16
	Int32Val  int32
	BoolVal   bool
}

var KEYTYPE_PARSE_MAP = map[int]func(*[]byte, int, string) ProfileKey{
	1:  parseInt32,
	2:  parseInt32,
	3:  parseGameSyncState,
	4:  parseInt16,
	6:  parseBool,
	10: parseString,
}

func parseInt32(data *[]byte, position int, _ string) ProfileKey {
	i := readInt32(data, position)
	return ProfileKey{
		Type:      "int32",
		EndOffset: position + 4, // Skip 2 for the int and 2 null padding bytes
		Int32Val:  i,
	}

}

func parseGameSyncState(_ *[]byte, position int, _ string) ProfileKey {
	return ProfileKey{
		Type:      "gamesyncstate",
		EndOffset: position + 8,
	}
}

func parseInt16(data *[]byte, position int, _ string) ProfileKey {
	i := readInt16(data, position)
	return ProfileKey{
		Type:      "uint16",
		EndOffset: position + 2,
		Int16Val:  i,
	}
}

func parseBool(data *[]byte, position int, _ string) ProfileKey {
	b := readBool(data, position)
	return ProfileKey{
		Type:      "bool",
		EndOffset: position + 1,
		BoolVal:   b,
	}
}

func parseString(data *[]byte, position int, keyname string) ProfileKey {
	value := readString(data, position)
	position = value.endOffset
	return ProfileKey{
		Type:      "string",
		EndOffset: position,
		StringVal: value.value,
	}
}

func parseProfileKeys(data *[]byte, node Node) (map[string]ProfileKey, error) {
	if !isRootNode(node) {
		return nil, NotRootNodeError(node)
	}

	children := node.getChildren("MP", "ST")
	if len(children) == 0 {
		return nil, NoChildNodesError("MP/ST")
	}

	if len(children) > 1 {
		return nil, MultipleNodesError("MP/ST")
	}

	stNode := children[0]
	// Skip the token (2), data length (4) + 6 null padding bytes
	position := stNode.offset + 10
	numKeys := readInt32(data, position)
	position += 4 // Moving up 4 for the above int32 read

	profileKeys := make(map[string]ProfileKey)
	for i := int32(0); i < numKeys; i++ {
		keyname := readString(data, position)
		keytype := readInt32(data, keyname.endOffset)
		position = keyname.endOffset + 4 // Skip 4 null bytes after the keytype

		parseFunc, exists := KEYTYPE_PARSE_MAP[int(keytype)]
		if !exists {
			return profileKeys, errors.New(fmt.Sprintf("%v not found in keytype parse map", keytype))
		}

		profileKey := parseFunc(data, position, keyname.value)
		profileKeys[keyname.value] = profileKey
		position = profileKey.EndOffset
	}
	return profileKeys, nil

}

func printProfileKeys(profileKeys map[string]ProfileKey) {
	for keyname, profileKey := range profileKeys {
		fmt.Printf("keyname=%v", keyname)
		switch profileKey.Type {
		case "string":
			fmt.Printf("value=%v", profileKey.StringVal)
		case "int32":
			fmt.Printf("value=%v", profileKey.Int32Val)
		case "bool":
			fmt.Printf("value=%v", profileKey.BoolVal)
		}
		fmt.Printf("\n")
	}
}
