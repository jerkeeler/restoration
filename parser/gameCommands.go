package parser

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
)

var FOOTER = []uint8{0, 1, 0, 0, 0, 0, 0, 0}

// =========================================================================
// The first part of this file contains the command type definitions and
// defines how they can be read and interpreted from the raw bytes.
// =========================================================================

type GameCommand interface {
	CommandType() int
	OffsetEnd() int
	PlayerId() int
	ByteLength() int
}

type BaseCommand struct {
	commandType int
	playerId    int
	offsetEnd   int
	byteLength  int
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

type ResearchCommand struct {
	BaseCommand
	techId int32
}

type CommandList struct {
	entryIdx     int
	offsetEnd    int
	finalCommand bool
	commands     []GameCommand
}

// TODO: Implement actual refiners. Right now all of these functions purely return how many
// bytes are consumed for a given command type, this is a good litmus test to make sure
// we can read the whole file. Next step is to to actually convert these into useable objects.
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

var REFINERS = map[int]func(*[]byte, int, int) GameCommand{
	// task
	0: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{
			unpackInt32,
			unpackInt32,
			unpackInt32,
			unpackInt32,
			unpackVector,
			unpackFloat,
			unpackInt32,
			unpackInt32,
			unpackInt32,
		}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 0,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// research
	1: func(data *[]byte, offset int, playerId int) GameCommand {
		// The research command is 12 bytes in length, the last 4 bytes are an int32 representing the id of the tech
		// that was researched.
		// inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32}
		byteLength := 12
		techId := readInt32(data, offset+8)
		return ResearchCommand{
			BaseCommand: BaseCommand{
				commandType: 1,
				playerId:    playerId,
				byteLength:  byteLength,
				offsetEnd:   offset + byteLength,
			},
			techId: techId,
		}
	},

	// train
	2: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{
			unpackInt32,
			unpackInt32,
			unpackInt32,
			unpackInt32,
			unpackInt8,
			unpackInt8,
		}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 2,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// build
	3: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{
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
		}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 3,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	4: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackVector, unpackFloat, unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 4,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// delete
	7: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt8}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 7,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// stop
	9: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 9,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// useProtoPower
	12: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{
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
		}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 12,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// marketBuySellResources
	13: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32, unpackInt32, unpackFloat}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 13,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// ungarrison
	14: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 14,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// resign
	16: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32, unpackInt32, unpackInt32, unpackInt8}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 16,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// unknown 18
	18: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 18,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// tribute
	19: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32, unpackInt32, unpackFloat, unpackFloat, unpackInt8}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 19,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// finishUnitTransform
	23: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32, unpackInt8, unpackInt8}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 23,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// setUnitStance
	25: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt8, unpackInt8, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 25,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// changeDiplomacy
	26: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt8, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 26,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// townBell
	34: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 34,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// autoScoutEvent
	35: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 35,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// changeControlGroupContents
	37: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt8, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 37,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// repair
	38: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 38,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// taunt
	41: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{
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
		}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 41,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// cheat
	44: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 44,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// cancelQueuedItem
	45: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32, unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 45,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// setFormation
	48: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 48,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// startUnitTransform
	53: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 53,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// unknown 55
	55: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackVector}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 55,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// autoqueue
	66: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 66,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// toggleAutoUnitAbility
	67: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt8}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 67,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// timeshift
	68: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackVector, unpackVector}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 68,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// buildWallConnector
	69: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32, unpackVector, unpackVector}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 69,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// seekShelter
	71: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 71,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// prequeueTech
	72: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32, unpackInt8}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 72,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},

	// prebuyGodPower
	75: func(data *[]byte, offset int, playerId int) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return BaseCommand{
			commandType: 75,
			playerId:    playerId,
			byteLength:  byteLength,
			offsetEnd:   offset + byteLength,
		}
	},
}

// =========================================================================
// Below is the actual code to parse the command bytes into command types
// that can be used.
// =========================================================================

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
	slog.Debug("Parsing command list", "offset", headerEndOffset+offset)

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
			return commandList, fmt.Errorf("entryIdx was not sequential, item.entryIdx=%v, lastIndex=%v", item.entryIdx, lastIndex)
		}
		offset = item.offsetEnd
	}

	return commandList, nil
}

func findFooterOffset(data *[]byte, offset int) (int, error) {
	/*
		Each set of commands is followd by a "FOOTER" (footer is probably not the correct term) the demarcates the
		end of the command sequence and the beginning of the next. This function finds end of this footer and the
		beginning of the next command.
	*/
	derefedData := *data
	// early := derefedData[offset:offset+10]
	extraByteCount := derefedData[offset]
	offset += 1
	extraByteNumbers := make([]uint8, extraByteCount)
	for i := 0; i < int(extraByteCount); i++ {
		extraByteNumbers[i] = derefedData[offset+i]
	}
	if extraByteCount > 0 {
		slog.Debug(fmt.Sprintf("foot has %v extra bytes: %v", extraByteCount, extraByteNumbers))
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
	   Parses a command list. The first int is a bit mask. Valid values:
	   1
	   32
	   64
	   128
	*/
	derefedData := *data
	entryType := readUint32(data, offset)
	// slog.Debug(fmt.Sprintf("Parsing command list at offset=%v entryType=%v", offset, entryType))
	offset += 4
	// earlyByte = data[offset]
	offset += 1

	if entryType&225 != entryType {
		return CommandList{}, fmt.Errorf("bad entry type, masking to 224 doesn't work for %v", entryType)
	}
	if entryType&96 == 96 {
		return CommandList{}, errors.New("96 entryType does't make sense")
	}

	if entryType&1 == 0 {
		offset += 4
	} else {
		offset += 1
	}

	commands := make([]GameCommand, 0)

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
			offset = command.OffsetEnd()
		}
	}

	for _, cmd := range commands {
		if _, ok := cmd.(ResearchCommand); ok {
			// It's a ResearchCommand
			// slog.Debug("TechId", "techId", researchCmd.techId, "playerId", researchCmd.playerId)
		}
	}

	// TODO: Do something with selectedUints
	// selectedUints := make([]uint32, 0)
	if entryType&128 != 0 {
		numItems := int(derefedData[offset])
		offset += 1
		for i := 0; i < numItems; i++ {
			// selectedUints = append(selectedUints, readUint32(data, offset))
			offset += 4
		}
	}

	// TODO: Remove this and modify this to work for more than 1v1 replays.
	// Check if the last command is the resign command. Right now, the code panics because it cannot find a footer
	// after the resign command is issued. I haven't tried running this on team games yet, but I imagine that
	// it might work correctly. We'll need a smarter way to determine the end of the command stream.
	for _, cmd := range commands {
		if cmd.CommandType() == 16 {
			// Resign command issued, return the command list
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
	// Right after the footer is the "entry index" which is basically the index of this command sequence.
	// All CommandList commands should be ascending sequence order.
	entryIdx := readUint32(data, offset)
	offset += 4
	finalByte := derefedData[offset]
	if finalByte != 0 {
		return CommandList{}, fmt.Errorf("final byte doesn't equal 0, finalByte=%v", finalByte)
	}
	offset += 1

	return CommandList{
		int(entryIdx),
		offset,
		false,
		commands,
	}, nil
}

func parseGameCommand(data *[]byte, offset int) (GameCommand, error) {
	/*
		Parses a direct game command and does some sanity checking of bytes. This commnad goes through
		a refiner defined in the REFINERS variable. If a refiner doesn't exists for the command type then
		this function will fail.
	*/
	derefedData := *data
	commandType := int(derefedData[offset+1])
	// slog.Debug(fmt.Sprintf("Parsing game command with type=%v at offset=%v", commandType, offset))
	tenBytesOffset := offset
	offset += 10
	if commandType == 14 {
		offset += 20
	} else {
		offset += 8
	}

	three := readUint32(data, offset)
	if three != uint32(3) {
		return BaseCommand{}, fmt.Errorf("expecting three while parsing game command, three=%v", three)
	}
	offset += 4
	playerId := -1
	if commandType == 19 {
		playerId = int(derefedData[tenBytesOffset+7])
		offset += 4
	} else {
		one := readUint16(data, offset)
		if one != uint16(1) {
			return BaseCommand{}, fmt.Errorf("expecting one while parsing game command, one=%v", one)
		}
		offset += 4
		playerId = int(readUint16(data, offset))
		if playerId > 12 {
			return BaseCommand{}, fmt.Errorf("player id must be 12 or less, playerId=%v", playerId)
		}
		offset += 4
	}
	offset += 4
	numUnits := readUint16(data, offset)
	offset += 4

	// TODO: Use sourceUnits for something?
	// sourceUnits := make([]uint16, numUnits)
	for i := 0; i < int(numUnits); i++ {
		// sourceUnits = append(sourceUnits, readUint16(data, offset))
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
		return BaseCommand{}, fmt.Errorf("refiner not defined for commandType=%v", commandType)
	}

	gameCommand := refiner(data, offset, playerId)
	offset += gameCommand.ByteLength()

	// slog.Debug(fmt.Sprintf("Parsing game command with type=%v for player playerId=%v", commandType, playerId))
	return gameCommand, nil
}
