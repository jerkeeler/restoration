package parser

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
)

func newBaseCommand(
	offset int,
	commandType int,
	playerId int,
	lastCommandListIdx int,
	sourceUnits *[]uint32,
	sourceVectors *[]Vector3,
	preArgumentBytes *[]uint8,
) BaseCommand {
	cmd := BaseCommand{
		offset:      offset,
		commandType: commandType,
		playerId:    playerId,
		// Set the game time to seconds, but using the index of the command list. Command lists occur every 1/20 of a second.
		// Basically, the game ticks every 1/20 of a second and batches commands that occur in between into one command list
		// so we can use the index of the command list to get the game time.
		gameTimeSecs:     float64(lastCommandListIdx) / 20.0,
		affectsEAPM:      true,
		sourceUnits:      sourceUnits,
		sourceVectors:    sourceVectors,
		preArgumentBytes: preArgumentBytes,
	}
	return cmd
}

// =========================================================================
// Below is the actual code to parse the command bytes into command types
// that can be used.
// =========================================================================

func parseGameCommands(data *[]byte, headerEndOffset int) ([]RawGameCommand, error) {
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
	commandList := make([]RawGameCommand, 0)

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

	commands := make([]RawGameCommand, 0)

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
	// All CommandList commands should be ascending sequence order. This the game tick at which this set of
	// commands occurred. The game seems to run at 20hz, so each entryIdx is 1/20th of a second. At all commands
	// in that same 1/20th of a second are grouped into the same command list.
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

func parseGameCommand(data *[]byte, offset int, lastCommandListIdx int) (RawGameCommand, error) {
	/*
		Parses a direct game command and does some sanity checking of bytes. This commnad goes through
		a refiner defined by the Refine function on the command type in gameCommands.go If a refiner doesn't exist
		for the command type then this function will fail.
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

	sourceUnits := make([]uint32, numUnits)
	for i := 0; i < int(numUnits); i++ {
		sourceUnits[i] = readUint32(data, offset)
		offset += 4
	}

	numVectors := readUint16(data, offset)
	offset += 4
	sourceVectors := make([]Vector3, numVectors)
	for i := 0; i < int(numVectors); i++ {
		sourceVectors[i] = readVector(data, offset)
		offset += 12
	}

	numPreArgumentBytes := 13 + readUint16(data, offset)
	offset += 4
	preArgumentBytes := make([]uint8, numPreArgumentBytes)
	for i := 0; i < int(numPreArgumentBytes); i++ {
		preArgumentBytes[i] = derefedData[offset+i]
	}
	offset += int(numPreArgumentBytes)

	refiner, exists := CommandFactoryInstance.Get(commandType)
	if !exists {
		return BaseCommand{}, fmt.Errorf("refiner not defined for commandType=%v", commandType)
	}

	baseCmd := newBaseCommand(
		offset,
		commandType,
		playerId,
		lastCommandListIdx,
		&sourceUnits,
		&sourceVectors,
		&preArgumentBytes,
	)
	gameCommand := refiner(&baseCmd, data)
	offset += gameCommand.ByteLength()

	// slog.Debug(fmt.Sprintf("Parsing game command with type=%v for player playerId=%v", commandType, playerId))
	return gameCommand, nil
}
