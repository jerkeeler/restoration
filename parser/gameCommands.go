package parser

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
)

// =========================================================================
// The first part of this file contains the command type definitions and
// defines how they can be read and interpreted from the raw bytes.
// =========================================================================

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

func newBaseCommand(offset int, commandType int, playerId int, lastCommandListIdx int) BaseCommand {
	cmd := BaseCommand{
		offset:      offset,
		commandType: commandType,
		playerId:    playerId,
		// Set the game time to seconds, but using the index of the command list. Command lists occur every 1/20 of a second.
		// Basically, the game ticks every 1/20 of a second and batches commands that occur in between into one command list
		// so we can use the index of the command list to get the game time.
		gameTimeSecs: float64(lastCommandListIdx) / 20.0,
		affectsEAPM:  true,
	}
	return cmd
}

func enrichBaseCommand(baseCommand BaseCommand, byteLength int) BaseCommand {
	return BaseCommand{
		offset:       baseCommand.offset,
		commandType:  baseCommand.commandType,
		playerId:     baseCommand.playerId,
		gameTimeSecs: baseCommand.gameTimeSecs,
		byteLength:   byteLength,
		offsetEnd:    baseCommand.offset + byteLength,
		affectsEAPM:  baseCommand.affectsEAPM,
	}
}

var REFINERS = map[int]func(*[]byte, BaseCommand) GameCommand{
	// task
	0: func(data *[]byte, baseCommand BaseCommand) GameCommand {
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
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// research
	1: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		// The research command is 12 bytes in length, the last 4 bytes are an int32 representing the id of the tech
		// that was researched. The id maps to a string via the techtree XMB data stored in the header of the replay.
		// inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32}
		byteLength := 12
		techId := readInt32(data, baseCommand.offset+8)
		return ResearchCommand{
			BaseCommand: enrichBaseCommand(baseCommand, byteLength),
			techId:      techId,
		}
	},

	// train
	2: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		// The train commands contains 4 Int32s (16 bytes) and 2 Int8s (2 bytes). The 3rd, Int32 is the protoUnitId,
		// and the last Int8 is the number of units queued.
		byteLength := 18
		protoUnitId := readInt32(data, baseCommand.offset+8)
		numUnits := int8((*data)[baseCommand.offset+18])
		return TrainCommand{
			BaseCommand: enrichBaseCommand(baseCommand, byteLength),
			protoUnitId: protoUnitId,
			numUnits:    numUnits,
		}
	},

	// build
	3: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		// The build command is 52 bytes in length, consisting of the following values in sequence:
		// 4 int32s, 1 vector, 2 int32s 1 float, 4 int32s
		byteLength := 52
		// queued attribute comes from "preargument bytes", we will leave it as false for now
		// protoUnitId comes from the 3rd int32 in the command
		protoUnitId := readInt32(data, baseCommand.offset+8)
		location := readVector(data, baseCommand.offset+12)
		return BuildCommand{
			BaseCommand:     enrichBaseCommand(baseCommand, byteLength),
			protoBuildingId: protoUnitId,
			location:        location,
			queued:          false,
		}
	},

	4: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackVector, unpackFloat, unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		// Currently this command triggers a Task subtype move command immediately afterwards, so we don't want to double count
		baseCommand.affectsEAPM = false
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// delete
	7: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt8}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// stop
	9: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// useProtoPower
	12: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		// useProtoPower is 57 bytes in length consisting of:
		// 3 int32s, 2 vectors, 2 int32s, 1 float, 2 int32s, 1 int8
		// the last int32 is the protoPowerId that maps to a string god power via the proto XMB data
		// Proto powers are unit abilities that are cast by a unit manually by the player (e.g., if the player
		// casts the centaur rain of arrows power) AND god powers. The names are stored in the powers XMB data file
		// stored in the header of the replay.
		//
		// The two vectors are the target locations of the power being used, if it has a location. If it has a second
		// location (e.g., shifting sands, underworld, etc...) the second vector will be the second location.
		byteLength := 57
		protoPowerId := readInt32(data, baseCommand.offset+52)
		return ProtoPowerCommand{
			BaseCommand:  enrichBaseCommand(baseCommand, byteLength),
			protoPowerId: protoPowerId,
		}
	},

	// marketBuySellResources
	13: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32, unpackInt32, unpackFloat}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// ungarrison
	14: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// resign
	16: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32, unpackInt32, unpackInt32, unpackInt8}
		byteLength := 0
		for _, f := range inputTypes {
			slog.Debug("resign", "f", readUint32(data, baseCommand.offset+byteLength))
			byteLength += f()
		}
		baseCommand.affectsEAPM = false
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// unknown 18
	18: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// tribute
	19: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32, unpackInt32, unpackFloat, unpackFloat, unpackInt8}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// finishUnitTransform
	23: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32, unpackInt8, unpackInt8}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// setUnitStance
	25: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt8, unpackInt8, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// changeDiplomacy
	26: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt8, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// townBell
	34: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// autoScoutEvent
	35: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		baseCommand.affectsEAPM = false
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// changeControlGroupContents
	37: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt8, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		// Every time you change a control group, the game triggers one event per unit in the group (removing them) and then readds them all, with 1 event per unit
		// Including this would inflate CPM by a LOT.
		baseCommand.affectsEAPM = false
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// repair
	38: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// taunt
	41: func(data *[]byte, baseCommand BaseCommand) GameCommand {
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
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// cheat
	44: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// cancelQueuedItem
	45: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32, unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// setFormation
	48: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// startUnitTransform
	53: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		// debateable, selecting a lot of units and doing this creates one command per unit transformed
		baseCommand.affectsEAPM = false
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// unknown 55
	55: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackVector}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// autoqueue
	66: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		// The autoqueue command is 12 bytes in length, consisting of 3 int32s. The last int32 is the protoUnitId.
		// inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32}
		byteLength := 12
		protoUnitId := readInt32(data, baseCommand.offset+8)
		return AutoqueueCommand{
			BaseCommand: enrichBaseCommand(baseCommand, byteLength),
			protoUnitId: protoUnitId,
		}
	},

	// toggleAutoUnitAbility
	67: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt8}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// timeshift
	68: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackVector, unpackVector}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// buildWallConnector
	69: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32, unpackVector, unpackVector}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		// Making a simple wall puts out a LOT of these.
		baseCommand.affectsEAPM = false
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// seekShelter
	71: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		inputTypes := []func() int{unpackInt32, unpackInt32}
		byteLength := 0
		for _, f := range inputTypes {
			byteLength += f()
		}
		return enrichBaseCommand(baseCommand, byteLength)
	},

	// prequeueTech
	72: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		// The prequeTech command is 13 bytes in length, bytes 8-12 are an int32 representing the id of the tech
		// that was prequeued. The id maps to a string via the techtree XMB data stored in the header of the replay.
		// inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32, unpackInt8}

		byteLength := 13
		techId := readInt32(data, baseCommand.offset+8)
		return PrequeueTechCommand{
			BaseCommand: enrichBaseCommand(baseCommand, byteLength),
			techId:      techId,
		}
	},

	// prebuyGodPower
	75: func(data *[]byte, baseCommand BaseCommand) GameCommand {
		// The prebuyGodPower command is 16 bytes in length, consisting of 4 int32s. The 3rd xint32 is the protoPowerId.
		byteLength := 16
		return enrichBaseCommand(baseCommand, byteLength)
	},
}

// =========================================================================
// Below is the actual code to parse the command bytes into command types
// that can be used.
// =========================================================================

func parseGameCommands(data *[]byte, headerEndOffset int) ([]GameCommand, error) {
	offset := bytes.Index((*data)[headerEndOffset:], FOOTER)
	slog.Debug("Parsing command list", "offset", strconv.FormatInt(int64(headerEndOffset+offset), 16))

	if offset == -1 {
		return nil, FooterNotFoundError(offset)
	}

	firstFootEnd, err := findFooterOffset(data, headerEndOffset+offset)
	if err != nil {
		return nil, err
	}
	offset = firstFootEnd + 5
	lastIndex := 1
	commandList := make([]GameCommand, 0)

	for {
		if offset == len(*data)-1 {
			// We've reached the end!
			break
		}
		item, err := parseCommandList(data, offset, lastIndex)
		if err != nil {
			return commandList, err
		}
		// Add all the commands to the command list, flattening everything into a single list.
		commandList = append(commandList, item.commands...)
		if item.finalCommand {
			// We've reached the end! Someone resigned.
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
		slog.Debug("unk not equal to 1", "unk", unk)
		return -1, UnkNotEqualTo1Error(offset)
	}

	offset += 9
	oneFourthFooterLength := readUint16(data, offset)
	offset += 4
	endOffset := offset + 4*int(oneFourthFooterLength)
	// late = derefedData[offset:endOffset]
	return endOffset, nil
}

func parseCommandList(data *[]byte, offset int, lastCommandListIdx int) (CommandList, error) {
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
			command, err := parseGameCommand(data, offset, lastCommandListIdx)
			if err != nil {
				return CommandList{}, err
			}
			commands = append(commands, command)
			offset = command.OffsetEnd()
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
			slog.Debug("Resign command issued", "cmd", cmd)
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

func parseGameCommand(data *[]byte, offset int, lastCommandListIdx int) (GameCommand, error) {
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
	offset += 4
	if three != uint32(3) {
		return BaseCommand{}, fmt.Errorf("expecting three while parsing game command %v, three=%v", commandType, three)
	}

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

	baseCmd := newBaseCommand(offset, commandType, playerId, lastCommandListIdx)
	gameCommand := refiner(data, baseCmd)
	offset += gameCommand.ByteLength()

	// slog.Debug(fmt.Sprintf("Parsing game command with type=%v for player playerId=%v", commandType, playerId))
	return gameCommand, nil
}
