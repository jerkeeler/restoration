package parser

import (
	"fmt"
	"log/slog"
)

func parseXmbMap(data *[]byte, rootNode Node) (map[string]XmbFile, error) {
	slog.Debug("Parsing XMB data set from nodes GM/GD/gd")
	children := rootNode.getChildren("GM", "GD", "gd")
	xmbMap := make(map[string]XmbFile)
	for _, child := range children {
		offset := child.offset + 2 + 4 // Skipping 2 bytes for the token + 4 bytes for the data length

		// First byte unknown
		offset += 1

		// Second byte is the number of XMB files stored in this node
		numFiles := readUint32(data, offset)
		offset += 4
		// slog.Debug("Num Files", "numFiles", numFiles)

		for i := uint32(0); i < numFiles; i++ {
			var xmbName RecString
			if numFiles > 1 {
				// Read two strings, keep the second as xmbName
				str1 := readString(data, offset)
				xmbName = readString(data, str1.endOffset)
				offset = xmbName.endOffset
			} else {
				// If there is only one XMB file, it is stored 20 bytes after the start of the node
				xmbName = readString(data, offset+20)
			}
			// slog.Debug("XMB Name", "xmbName", xmbName.value)
			xmbMap[xmbName.value] = XmbFile{
				name:   xmbName.value,
				offset: offset,
			}
		}
		dataLength := readUint32(data, offset+2)
		offset += int(dataLength) + DATA_OFFSET
	}
	return xmbMap, nil
}

func parseXmb(data *[]byte, xmbFile XmbFile) (XmbNode, error) {
	offset := xmbFile.offset
	x1 := readUint16(data, offset)
	if x1 != 12632 {
		return XmbNode{}, fmt.Errorf("x1 not equal to 12632 (X1) at offset=%v, x1=%v", offset, x1)
	}
	offset += 6
	xr := readUint16(data, offset)
	if xr != 21080 {
		return XmbNode{}, fmt.Errorf("xr not equal to 21080 (XR) at offset=%v, xr=%v", offset, xr)
	}
	offset += 2
	unk1 := readUint32(data, offset)
	if unk1 != 4 {
		return XmbNode{}, fmt.Errorf("unk1 not equal to 4 at offset=%v, unk1=%v", offset, unk1)
	}
	offset += 4

	version := readUint32(data, offset)
	if version != 8 {
		return XmbNode{}, fmt.Errorf("version not equal to 8 at offset=%v, version=%v", offset, version)
	}
	offset += 4

	numElements := readUint32(data, offset)
	offset += 4
	elements := make([]string, numElements)
	// slog.Debug("Num Elements", "numElements", numElements)
	for i := uint32(0); i < numElements; i++ {
		str := readString(data, offset)
		offset = str.endOffset
		// slog.Debug("Element", "element", str.value)
		elements[i] = str.value
	}

	numAttributes := readUint32(data, offset)
	offset += 4
	attributes := make([]string, numAttributes)
	// slog.Debug("Num Attributes", "numAttributes", numAttributes)
	for i := uint32(0); i < numAttributes; i++ {
		str := readString(data, offset)
		offset = str.endOffset
		// slog.Debug("Attribute", "attribute", str.value)
		attributes[i] = str.value
	}

	rootNode, err := parseXmbNode(data, offset, elements, attributes)
	if err != nil {
		return XmbNode{}, err
	}
	return rootNode, nil
}

func parseXmbNode(data *[]byte, offset int, elements []string, attributes []string) (XmbNode, error) {
	// This is a recursive function that parses the XMB node and all of its children
	initialOffset := offset

	// Verify the node is valid, we expect each node to start with XN
	xn := readUint16(data, offset)
	offset += 2
	if xn != 20056 {
		return XmbNode{}, fmt.Errorf("xn not equal to 20056 (XN) at offset=%v, xn=%v", offset, xn)
	}

	offset += 4 // skip 4 unknown bytes

	parsedValue := readString(data, offset)
	offset = parsedValue.endOffset
	// slog.Debug("Parsed Value", "parsedValue", parsedValue.value)

	nameIdx := readUint32(data, offset)
	offset += 4
	elementName := elements[nameIdx]
	// slog.Debug("Element Name", "elementName", elementName)
	offset += 4 // skip 4 unknown bytes

	numAttributes := readUint32(data, offset)
	offset += 4
	attributeNames := make([]string, numAttributes)
	attributeValues := make([]string, numAttributes)

	for i := uint32(0); i < numAttributes; i++ {
		attributeName := attributes[readUint32(data, offset)]
		offset += 4
		attributeValue := readString(data, offset)
		offset = attributeValue.endOffset
		attributeNames[i] = attributeName
		attributeValues[i] = attributeValue.value
		// slog.Debug("Attribute Name", "attributeName", attributeName, "attributeValue", attributeValue.value)
	}

	numChildren := readUint32(data, offset)
	offset += 4
	children := make([]*XmbNode, numChildren)
	for i := uint32(0); i < numChildren; i++ {
		childNode, err := parseXmbNode(data, offset, elements, attributes)
		if err != nil {
			return XmbNode{}, err
		}
		children[i] = &childNode
		offset = childNode.endOffset
	}

	attributesMap := make(map[string]string)
	for i, attributeName := range attributeNames {
		attributesMap[attributeName] = attributeValues[i]
	}

	return XmbNode{
		elementName: elementName,
		value:       parsedValue.value,
		attributes:  attributesMap,
		children:    children,
		offset:      initialOffset,
		endOffset:   offset,
	}, nil
}
