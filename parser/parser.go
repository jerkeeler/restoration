package parser

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
)

func ParseToJson(replayPath string, prettyPrint bool, slim bool, stats bool, isGzip bool) (string, error) {
	replayFormat, err := Parse(replayPath, slim, stats, isGzip)
	if err != nil {
		return "", err
	}

	var jsonBytes []byte
	if prettyPrint {
		jsonBytes, err = json.MarshalIndent(replayFormat, "", "    ")
	} else {
		jsonBytes, err = json.Marshal(replayFormat)
	}

	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

// Parse is the main entry point for the parser.
// Note that there are a LOT of opportunities to parallelize work in this parser using lightweight go routines. However,
// for now we will forego this optimizations until the parser becomes unreasonable slow. At a high level the only
// parallelization we will do will be at the replay level. Eventually the parser will allow you to provide a glob
// pattern or multiple files as input and each file will be parsed in its own go routine.
// If we do need to add more optimization, all of the recursive functions could easily spin up a go routine to parse its
// subtree.
func Parse(replayPath string, slim bool, stats bool, isGzip bool) (ReplayFormatted, error) {
	raw_data, err := os.ReadFile(replayPath)
	if err != nil {
		return ReplayFormatted{}, err
	}

	if isGzip {
		raw_data, err = DecompressGzip(&raw_data)

		if err != nil {
			return ReplayFormatted{}, err
		}
	}

	data, err := Decompressl33t(&raw_data)
	if err != nil {
		return ReplayFormatted{}, err
	}
	// saveHex(&data, "decompressed.hex")

	rootNode := parseHeader(&data)

	// Note, we are not parsing all XMB files here. We are parsing the map of XMB files so we know where they are.
	// Since the XMB files are large we'll saving parsing them until we need them and simply pass the map of XMB files
	// around instead.
	xmbMap, err := parseXmbMap(&data, rootNode)
	if err != nil {
		return ReplayFormatted{}, err
	}
	// for key, _ := range xmbMap {
	// 	fmt.Println(key)
	// }

	profileKeys, err := parseProfileKeys(&data, rootNode)
	if err != nil {
		return ReplayFormatted{}, err
	}
	//printProfileKeys(profileKeys)
	// for key, _ := range xmbMap {
	// 	fmt.Println("==========================")
	// 	fmt.Println(key)
	// }
	// techtreerootnode, err := parseXmb(&data, xmbMap["protounitcommands"])
	// if err != nil {
	// 	return ReplayFormatted{}, err
	// }
	// for _, child := range techtreerootnode.children {
	// 	fmt.Println(child)
	// }

	svBytes := bytes.Index(raw_data, []byte{0x73, 0x76}) // search for index of the "sv" bytes
	commandOffset := readUint32(&raw_data, svBytes+2)
	slog.Debug("commandOffset", "commandOffset", commandOffset)
	commandList, err := parseGameCommands(&raw_data, int(commandOffset))
	if err != nil {
		return ReplayFormatted{}, err
	}

	replayFormat, err := formatRawDataToReplay(slim, stats, &data, &rootNode, &profileKeys, &xmbMap, &commandList)
	if err != nil {
		return ReplayFormatted{}, err
	}

	return replayFormat, nil
}

func isRootNode(node Node) bool {
	return node.token == ROOT_NODE_TOKEN
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
	slog.Debug("Parsing profile keys from MP/ST node")
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
	// Useful for debugging, prints out all profile keys and their values
	for keyname, profileKey := range profileKeys {
		log := ""
		log += fmt.Sprintf("keyname=%v", keyname)
		switch profileKey.Type {
		case "string":
			log += fmt.Sprintf("value=%v", profileKey.StringVal)
		case "int32":
			log += fmt.Sprintf("value=%v", profileKey.Int32Val)
		case "bool":
			log += fmt.Sprintf("value=%v", profileKey.BoolVal)
		}
		slog.Debug(log)
	}
}

func saveHex(data *[]byte, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(*data)
	if err != nil {
		return err
	}

	return nil
}
