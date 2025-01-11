package parser

import (
	"bytes"
	"errors"
	"fmt"
)

var FOOTER = []uint8{0, 1, 0, 0, 0, 0, 0, 0}

type CommandItem struct {
	commandType int
	offsetEnd   int
}

type CommandList struct {
	entryIdx     int
	offsetEnd    int
	finalCommand bool
	commands     []CommandItem
}

func unpackInt32() int {
	return 4
}

func unpackInt8() int {
	return 1
}

func unpackVector() int {
	return 12
}

func unpackFloat() int {
	return 4
}

var REFINERS = map[int][]func() int{
	0: {
		unpackInt32,
		unpackInt32,
		unpackInt32,
		unpackInt32,
		unpackVector,
		unpackFloat,
		unpackInt32,
		unpackInt32,
		unpackInt32,
	},
	1: {unpackInt32, unpackInt32, unpackInt32},
	2: {unpackInt32, unpackInt32, unpackInt32, unpackInt32, unpackInt8, unpackInt8},
	3: {
		unpackInt32,
		unpackInt32,
		unpackInt32,
		unpackVector,
		unpackInt32,
		unpackInt32,
		unpackFloat,
		unpackInt32,
		unpackInt32,
		unpackInt32,
		unpackInt32,
	},
	4: {unpackInt32, unpackInt32, unpackVector, unpackFloat, unpackInt32, unpackInt32},
	7: {unpackInt32, unpackInt32, unpackInt8},
	9: {unpackInt32, unpackInt32},
	12: {
		unpackInt32,
		unpackInt32,
		unpackInt32,
		unpackVector,
		unpackVector,
		unpackInt32,
		unpackInt32,
		unpackFloat,
		unpackInt32,
		unpackInt32,
		unpackInt8,
	},
	13: {unpackInt32, unpackInt32, unpackInt32, unpackInt32, unpackFloat},
	14: {unpackInt32, unpackInt32},
	16: {unpackInt32, unpackInt32, unpackInt32, unpackInt32, unpackInt32, unpackInt8},
	18: {unpackInt32, unpackInt32, unpackInt32},
	19: {unpackInt32, unpackInt32, unpackInt32, unpackInt32, unpackFloat, unpackFloat, unpackInt8},
	23: {unpackInt32, unpackInt32, unpackInt32, unpackInt8, unpackInt8},
	25: {unpackInt32, unpackInt32, unpackInt8, unpackInt8, unpackInt32},
	26: {unpackInt32, unpackInt32, unpackInt8, unpackInt32},
	34: {unpackInt32, unpackInt32},
	35: {unpackInt32, unpackInt32, unpackInt32},
	37: {unpackInt32, unpackInt32, unpackInt8, unpackInt32},
	38: {unpackInt32, unpackInt32, unpackInt32},
	41: {
		unpackInt32,
		unpackInt32,
		unpackInt32,
		unpackInt32,
		unpackInt32,
		unpackInt32,
		unpackInt32,
		unpackInt32,
		unpackInt32,
		unpackInt32,
		unpackInt32,
		unpackInt8,
	},
	44: {unpackInt32, unpackInt32, unpackInt32, unpackInt32},
	45: {unpackInt32, unpackInt32, unpackInt32, unpackInt32, unpackInt32},
	48: {unpackInt32, unpackInt32, unpackInt32, unpackInt32},
	53: {unpackInt32, unpackInt32, unpackInt32},
	55: {unpackInt32, unpackInt32, unpackVector},
	66: {unpackInt32, unpackInt32, unpackInt32},
	67: {unpackInt32, unpackInt32, unpackInt8},
	68: {unpackInt32, unpackInt32, unpackVector, unpackVector},
	69: {unpackInt32, unpackInt32, unpackInt32, unpackVector, unpackVector},
	71: {unpackInt32, unpackInt32},
	72: {unpackInt32, unpackInt32, unpackInt32, unpackInt8},
	75: {unpackInt32, unpackInt32, unpackInt32, unpackInt32},
}

type FooterNotFoundError int

func (err FooterNotFoundError) Error() string {
	return fmt.Sprintf("Footer not found searching at offset=%v", int(err))
}

type UnkNotEqualTo1Error int

func (err UnkNotEqualTo1Error) Error() string {
	return fmt.Sprintf("The unknown byte in footer search did not equal 1 at offset %v", int(err))
}

func ParseCommandList(data *[]byte, headerEndOffset int) ([]CommandList, error) {
	offset := bytes.Index((*data)[headerEndOffset:], FOOTER)

	if offset == -1 {
		return nil, FooterNotFoundError(offset)
	}

	firstFootEnd, err := findFooterOffset(data, headerEndOffset+offset)
	if err != nil {
		return nil, err
	}
	offset = firstFootEnd + 5
	lastIndex := 1
	commandList := make([]CommandList, 0)

	for {
		if offset == len(*data)-1 {
			// We've reached the end!
			break
		}
		item, err := parseCommandList(data, offset)
		if err != nil {
			return commandList, err
		}
		commandList = append(commandList, item)
		if item.finalCommand {
			break
		}
		lastIndex += 1
		if item.entryIdx != lastIndex {
			return commandList, errors.New("entryIdx was not sequential!")
		}
		offset = item.offsetEnd
	}

	return commandList, nil
}

func findFooterOffset(data *[]byte, offset int) (int, error) {
	derefedData := *data
	// early := derefedData[offset:offset+10]
	extraByteCount := derefedData[offset]
	offset += 1
	extraByteNumbers := make([]uint8, extraByteCount)
	for i := 0; i < int(extraByteCount); i++ {
		extraByteNumbers[i] = derefedData[offset+i]
	}
	if extraByteCount > 0 {
		fmt.Printf("foot has %v extra bytes: %v\n", extraByteCount, extraByteNumbers)
	}

	unk := derefedData[offset]
	if unk != 1 {
		return -1, UnkNotEqualTo1Error(offset)
	}

	offset += 9
	oneFourthFooterLength := readUint16(data, offset)
	offset += 4
	endOffset := offset + 4*int(oneFourthFooterLength)
	// late = derefedData[offset:endOffset]
	return endOffset, nil
}

func parseCommandList(data *[]byte, offset int) (CommandList, error) {
	/*
	   Parses a command item, which is actually a list of commands. The first int is a bit mask. Valid values:
	   1
	   32
	   64
	   128
	*/
	derefedData := *data
	entryType := readUint32(data, offset)
	// fmt.Printf("Parsing command list at offset=%v entryType=%v\n", offset, entryType)
	offset += 4
	// earlyByte = data[offset]
	offset += 1

	if entryType&225 != entryType {
		return CommandList{}, errors.New(fmt.Sprintf("Bad entry type, masking to 224 doesn't work for %v", entryType))
	}
	if entryType&96 == 96 {
		return CommandList{}, errors.New("96 entryType does't make sense")
	}

	if entryType&1 == 0 {
		offset += 4
	} else {
		offset += 1
	}

	commands := make([]CommandItem, 0)

	if entryType&96 != 0 {
		numItems := 0
		if entryType&32 != 0 {
			numItems = int(derefedData[offset])
			offset += 1
		} else if entryType&64 != 0 {
			numItems = int(readUint32(data, offset))
			offset += 4
		}

		for i := 0; i < numItems; i++ {
			command, err := parseGameCommand(data, offset)
			if err != nil {
				return CommandList{}, err
			}
			commands = append(commands, command)
			offset = command.offsetEnd
		}
	}

	selectedUints := make([]uint32, 0)
	if entryType&128 != 0 {
		numItems := int(derefedData[offset])
		offset += 1
		for i := 0; i < numItems; i++ {
			selectedUints = append(selectedUints, readUint32(data, offset))
			offset += 4
		}
	}

	// Check if the last command is the resign command
	for _, cmd := range commands {
		if cmd.commandType == 16 {
			// Resign command issued
			return CommandList{
				-1,
				offset,
				true,
				commands,
			}, nil
		}
	}

	footerEndOffset, err := findFooterOffset(data, offset)
	if err != nil {
		return CommandList{}, err
	}
	offset = footerEndOffset
	entryIdx := readUint32(data, offset)
	offset += 4
	finalByte := derefedData[offset]
	if finalByte != 0 {
		return CommandList{}, errors.New("Final byte doesn't equal 0!")
	}
	offset += 1

	return CommandList{
		int(entryIdx),
		offset,
		false,
		commands,
	}, nil
}

func parseGameCommand(data *[]byte, offset int) (CommandItem, error) {
	derefedData := *data
	commandType := int(derefedData[offset+1])
	// fmt.Printf("Parsing game command with type=%v at offset=%v\n", commandType, offset)
	tenBytesOffset := offset
	offset += 10
	if commandType == 14 {
		offset += 20
	} else {
		offset += 8
	}

	three := readUint32(data, offset)
	if three != uint32(3) {
		return CommandItem{}, errors.New("Expecting three while parsing game command")
	}
	offset += 4
	playerId := -1
	if commandType == 19 {
		playerId = int(derefedData[tenBytesOffset+7])
		offset += 4
	} else {
		one := readUint16(data, offset)
		if one != uint16(1) {
			return CommandItem{}, errors.New("Expecting one while parsing game command")
		}
		offset += 4
		playerId = int(readUint16(data, offset))
		if playerId > 12 {
			return CommandItem{}, errors.New("Player id must be 12 or less")
		}
		offset += 4
	}
	offset += 4
	numUnits := readUint16(data, offset)
	offset += 4
	sourceUnits := make([]uint16, numUnits)
	for i := 0; i < int(numUnits); i++ {
		sourceUnits = append(sourceUnits, readUint16(data, offset))
		offset += 4
	}

	numVectors := readUint16(data, offset)
	offset += 4
	for i := 0; i < int(numVectors); i++ {
		// TODO: parse vectors
		offset += 12
	}

	numPreArgumentBytes := 13 + readUint16(data, offset)
	offset += 4
	for i := 0; i < int(numPreArgumentBytes); i++ {
		// TODO: Do something with prearugment bytes
	}
	offset += int(numPreArgumentBytes)

	refiner, exists := REFINERS[int(commandType)]
	if !exists {
		return CommandItem{}, errors.New(fmt.Sprintf("Refiner not defined for commandType=%v", commandType))
	}

	for _, f := range refiner {
		offset += f()
	}

	fmt.Printf("Parsing game command with type=%v for player playerId=%v\n", commandType, playerId)
	return CommandItem{commandType, offset}, nil
}
