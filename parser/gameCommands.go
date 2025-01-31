package parser

import (
	"log/slog"
	"strconv"
)

// =========================================================================
// The file contains the command type definitions and
// defines how they can be read and interpreted from the raw bytes.
// =========================================================================

type CommandFactory struct {
	refiners map[int]RefineableCommand
}

func NewCommandFactory() *CommandFactory {
	return &CommandFactory{
		refiners: make(map[int]RefineableCommand),
	}
}

func (cf *CommandFactory) Get(cmdType int) (RefineFunc, bool) {
	// Get a command type
	refiner, exists := cf.refiners[cmdType]
	if exists {
		return refiner.Refine, true
	}
	return nil, false
}

func (cf *CommandFactory) Register(cmdType int, refiner RefineableCommand) {
	// Register a new command type
	if _, exists := cf.refiners[cmdType]; !exists {
		cf.refiners[cmdType] = refiner
	} else {
		slog.Warn("Command type already registered", "type", cmdType)
	}
}

func BuildCommandFactory() *CommandFactory {
	factory := NewCommandFactory()

	factory.Register(0, TaskCommand{})
	factory.Register(1, ResearchCommand{})
	factory.Register(2, TrainCommand{})
	factory.Register(3, BuildCommand{})
	factory.Register(4, SetGatherPointCommand{})
	factory.Register(7, DeleteCommand{})
	factory.Register(9, StopCommand{})
	factory.Register(12, ProtoPowerCommand{})
	factory.Register(13, BuySellResourcesCommand{})
	factory.Register(14, UngarrisonCommand{})
	factory.Register(16, ResignCommand{})
	factory.Register(18, UnknownCommand18{})
	factory.Register(19, TributeCommand{})
	factory.Register(23, FinishUnitTransformCommand{})
	factory.Register(25, SetUnitStanceCommand{})
	factory.Register(26, ChangeDiplomacyCommand{})
	factory.Register(34, TownBellCommand{})
	factory.Register(35, AutoScoutEventCommand{})
	factory.Register(37, ChangeControlGroupContentsCommand{})
	factory.Register(38, RepairCommand{})
	factory.Register(41, TauntCommand{})
	factory.Register(44, CheatCommand{})
	factory.Register(45, CancelQueuedItemCommand{})
	factory.Register(48, SetFormationCommand{})
	factory.Register(53, StartUnitTransformCommand{})
	factory.Register(55, UnknownCommand55{})
	factory.Register(66, AutoqueueCommand{})
	factory.Register(67, ToggleAutoUnitAbilityCommand{})
	factory.Register(68, TimeShiftCommand{})
	factory.Register(69, BuildWallConnectorCommand{})
	factory.Register(71, SeekShelterCommand{})
	factory.Register(72, PrequeueTechCommand{})
	factory.Register(75, PrebuyGodPowerCommand{})

	return factory
}

// CommandFactoryInstance is a singleton of our CommandFactory as we only need to build up this map once and then
// use it everywhere
var CommandFactoryInstance = BuildCommandFactory()

// Utility functions for easily getting the bytes consumed by each type of input without doing any parsing.
// used to keep track of commands where we know the types, but don't want to implement a full refiner yet.
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

// ===============================
// RawGameCommand types
// ===============================

type FormatterInput struct {
	protoRootNode    *XmbNode
	techTreeRootNode *XmbNode
	powersRootNode   *XmbNode
}

type RawGameCommand interface {
	CommandType() int
	OffsetEnd() int
	PlayerId() int
	ByteLength() int
	GameTimeSecs() float64
	AffectsEAPM() bool
	Format(input FormatterInput) (ReplayGameCommand, bool)
}

type RefineFunc func(baseCommand *BaseCommand, data *[]byte) RawGameCommand
type RefineableCommand interface {
	Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand
}

type BaseCommand struct {
	commandType      int
	playerId         int
	offset           int
	offsetEnd        int
	byteLength       int
	gameTimeSecs     float64
	affectsEAPM      bool
	sourceUnits      *[]uint32
	sourceVectors    *[]Vector3
	preArgumentBytes *[]uint8
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

func (cmd BaseCommand) Format(input FormatterInput) (ReplayGameCommand, bool) {
	return ReplayGameCommand{}, false
}

func enrichBaseCommand(baseCommand *BaseCommand, byteLength int) {
	baseCommand.byteLength = byteLength
	baseCommand.offsetEnd = baseCommand.offset + byteLength
}

// ========================================================================
//  Actual command implementations
// ========================================================================

// ========================================================================
// 0 - task
// ========================================================================

type TaskCommand struct {
	BaseCommand
}

func (cmd TaskCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
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
	cmd.byteLength = byteLength
	enrichBaseCommand(baseCommand, byteLength)
	return TaskCommand{*baseCommand}
}

// ========================================================================
// 1 - research
// ========================================================================

type ResearchCommand struct {
	BaseCommand
	techId int32
}

func (cmd ResearchCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	// The research command is 12 bytes in length, the last 4 bytes are an int32 representing the id of the tech
	// that was researched. The id maps to a string via the techtree XMB data stored in the header of the replay.
	// inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32}
	byteLength := 12
	enrichBaseCommand(baseCommand, byteLength)
	return ResearchCommand{
		BaseCommand: *baseCommand,
		techId:      readInt32(data, baseCommand.offset+8),
	}
}

func (cmd ResearchCommand) Format(input FormatterInput) (ReplayGameCommand, bool) {
	return ReplayGameCommand{
		GameTimeSecs: cmd.GameTimeSecs(),
		PlayerNum:    cmd.PlayerId(),
		CommandType:  "research",
		Payload:      input.techTreeRootNode.children[cmd.techId].attributes["name"],
	}, true
}

// ========================================================================
// 2 - train
// ========================================================================

type TrainCommand struct {
	BaseCommand
	protoUnitId int32
	numUnits    int8
}

func (cmd TrainCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	// The train commands contains 4 Int32s (16 bytes) and 2 Int8s (2 bytes). The 3rd, Int32 is the protoUnitId,
	// and the last Int8 is the number of units queued.
	byteLength := 18
	enrichBaseCommand(baseCommand, byteLength)
	protoUnitId := readInt32(data, baseCommand.offset+8)
	numUnits := int8((*data)[baseCommand.offset+18])
	return TrainCommand{
		BaseCommand: *baseCommand,
		protoUnitId: protoUnitId,
		numUnits:    numUnits,
	}
}

func (cmd TrainCommand) Format(input FormatterInput) (ReplayGameCommand, bool) {
	proto := input.protoRootNode.children[cmd.protoUnitId].attributes["name"]
	return ReplayGameCommand{
		GameTimeSecs: cmd.GameTimeSecs(),
		PlayerNum:    cmd.PlayerId(),
		CommandType:  "train",
		Payload:      proto,
	}, true
}

// ========================================================================
// 3 - build
// ========================================================================

type BuildCommand struct {
	BaseCommand
	protoBuildingId int32
	queued          bool
	location        Vector3
}

func (cmd BuildCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	// The build command is 52 bytes in length, consisting of the following values in sequence:
	// 4 int32s, 1 vector, 2 int32s 1 float, 4 int32s
	byteLength := 52
	enrichBaseCommand(baseCommand, byteLength)
	// queued attribute comes from "preargument bytes", we will leave it as false for now
	// protoUnitId comes from the 3rd int32 in the command
	protoBuildingId := readInt32(data, baseCommand.offset+8)
	location := readVector(data, baseCommand.offset+12)
	queued := (*baseCommand.preArgumentBytes)[0]&2 != 0
	return BuildCommand{
		BaseCommand:     *baseCommand,
		protoBuildingId: protoBuildingId,
		location:        location,
		queued:          queued,
	}
}

type BuildCommandPaylod struct {
	Name     string
	Location Vector3
	Queued   bool
}

func (cmd BuildCommand) Format(input FormatterInput) (ReplayGameCommand, bool) {
	proto := input.protoRootNode.children[cmd.protoBuildingId].attributes["name"]
	return ReplayGameCommand{
		GameTimeSecs: cmd.GameTimeSecs(),
		PlayerNum:    cmd.PlayerId(),
		CommandType:  "build",
		Payload: BuildCommandPaylod{
			Name:     proto,
			Location: cmd.location,
			Queued:   cmd.queued,
		},
	}, true
}

// ========================================================================
// 4- setGatherPoint
// ========================================================================

type SetGatherPointCommand struct {
	BaseCommand
}

func (cmd SetGatherPointCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	inputTypes := []func() int{unpackInt32, unpackInt32, unpackVector, unpackFloat, unpackInt32, unpackInt32}
	byteLength := 0
	for _, f := range inputTypes {
		byteLength += f()
	}
	enrichBaseCommand(baseCommand, byteLength)
	// Currently this command triggers a Task subtype move command immediately afterwards, so we don't want to double count
	baseCommand.affectsEAPM = false
	return SetGatherPointCommand{*baseCommand}
}

// ========================================================================
// 7 - delete
// ========================================================================

type DeleteCommand struct {
	BaseCommand
}

func (cmd DeleteCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt8}
	byteLength := 0
	for _, f := range inputTypes {
		byteLength += f()
	}
	enrichBaseCommand(baseCommand, byteLength)
	return DeleteCommand{*baseCommand}
}

// ========================================================================
// 9 - stop
// ========================================================================

type StopCommand struct {
	BaseCommand
}

func (cmd StopCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	inputTypes := []func() int{unpackInt32, unpackInt32}
	byteLength := 0
	for _, f := range inputTypes {
		byteLength += f()
	}
	enrichBaseCommand(baseCommand, byteLength)
	return StopCommand{*baseCommand}
}

// ========================================================================
// 12 - useProtoPower
// ========================================================================

type ProtoPowerCommand struct {
	BaseCommand
	protoPowerId int32
	location1    Vector3
	location2    Vector3
}

func (cmd ProtoPowerCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
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
	enrichBaseCommand(baseCommand, byteLength)
	protoPowerId := readInt32(data, baseCommand.offset+52)
	location1 := readVector(data, baseCommand.offset+12)
	location2 := readVector(data, baseCommand.offset+24)
	return ProtoPowerCommand{
		BaseCommand:  *baseCommand,
		protoPowerId: protoPowerId,
		location1:    location1,
		location2:    location2,
	}
}

type ProtoPowerPayload struct {
	Name      string
	Location1 Vector3
	Location2 Vector3
}

func (cmd ProtoPowerCommand) Format(input FormatterInput) (ReplayGameCommand, bool) {
	power := input.powersRootNode.children[cmd.protoPowerId]
	var commandType string
	if _, ok := power.attributes["godpower"]; ok {
		commandType = "godPower"
	} else {
		commandType = "protoPower"
	}
	return ReplayGameCommand{
		GameTimeSecs: cmd.GameTimeSecs(),
		PlayerNum:    cmd.PlayerId(),
		CommandType:  commandType,
		Payload: ProtoPowerPayload{
			Name:      power.attributes["name"],
			Location1: cmd.location1,
			Location2: cmd.location2,
		},
	}, true
}

// ========================================================================
// 13 - marketBuySellResources
// ========================================================================

type MarketAction string

const (
	BuyAction  MarketAction = "buy"
	SellAction MarketAction = "sell"
)

type ResourceType string

const (
	FoodResource    ResourceType = "food"
	WoodResource    ResourceType = "wood"
	UnknownResource ResourceType = "unknown"
)

type BuySellResourcesCommand struct {
	BaseCommand
	resourceType ResourceType
	action       MarketAction
	quantity     float32
}

func (cmd BuySellResourcesCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	// marketBuySellResources is 20 bytes in length, consisting of 4 int32s and 1 float. The 3rd int32 is the
	// resource type and the float is how much of that resource is being bought/sold
	byteLength := 20
	enrichBaseCommand(baseCommand, byteLength)
	resourceId := readInt32(data, baseCommand.offset+8)

	var resourceType ResourceType
	if resourceId == 1 {
		resourceType = WoodResource
	} else if resourceId == 2 {
		resourceType = FoodResource
	} else {
		slog.Warn("Unknown resource type", "resourceId", resourceId)
		resourceType = UnknownResource
	}

	quantity := readFloat(data, baseCommand.offset+16)
	action := BuyAction
	if quantity < 0 {
		action = SellAction
		quantity = -quantity
	}

	return BuySellResourcesCommand{
		BaseCommand:  *baseCommand,
		resourceType: resourceType,
		action:       action,
		quantity:     quantity,
	}
}

func (cmd BuySellResourcesCommand) Format(input FormatterInput) (ReplayGameCommand, bool) {
	return ReplayGameCommand{
		GameTimeSecs: cmd.GameTimeSecs(),
		PlayerNum:    cmd.PlayerId(),
		CommandType:  "marketBuySell",
		Payload: struct {
			ResourceType string
			Action       string
			Quantity     float32
		}{
			ResourceType: string(cmd.resourceType),
			Action:       string(cmd.action),
			Quantity:     cmd.quantity,
		},
	}, true
}

// ========================================================================
// 14 - ungarrison
// ========================================================================

type UngarrisonCommand struct {
	BaseCommand
}

func (cmd UngarrisonCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	inputTypes := []func() int{unpackInt32, unpackInt32}
	byteLength := 0
	for _, f := range inputTypes {
		byteLength += f()
	}
	enrichBaseCommand(baseCommand, byteLength)
	return UngarrisonCommand{*baseCommand}
}

// ========================================================================
// 16 - resign
// ========================================================================

type ResignCommand struct {
	BaseCommand
}

func (cmd ResignCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32, unpackInt32, unpackInt32, unpackInt8}
	byteLength := 0
	for _, f := range inputTypes {
		//slog.Debug("resign", "f", readUint32(data, baseCommand.offset+byteLength))
		byteLength += f()
	}
	enrichBaseCommand(baseCommand, byteLength)
	baseCommand.affectsEAPM = false
	return ResignCommand{*baseCommand}
}

func (cmd ResignCommand) Format(input FormatterInput) (ReplayGameCommand, bool) {
	return ReplayGameCommand{
		GameTimeSecs: cmd.GameTimeSecs(),
		PlayerNum:    cmd.PlayerId(),
		CommandType:  "resign",
		Payload:      strconv.Itoa(cmd.PlayerId()),
	}, true
}

// ========================================================================
// 18 - Unknown
// ========================================================================

type UnknownCommand18 struct {
	BaseCommand
}

func (cmd UnknownCommand18) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32}
	byteLength := 0
	for _, f := range inputTypes {
		byteLength += f()
	}
	enrichBaseCommand(baseCommand, byteLength)
	return UnknownCommand18{*baseCommand}
}

// ========================================================================
// 19 - Tribute
// ========================================================================

type TributeCommand struct {
	BaseCommand
}

func (cmd TributeCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32, unpackInt32, unpackFloat, unpackFloat, unpackInt8}
	byteLength := 0
	for _, f := range inputTypes {
		byteLength += f()
	}
	enrichBaseCommand(baseCommand, byteLength)
	return TributeCommand{*baseCommand}
}

// ========================================================================
// 23 - finishUnitTransform
// ========================================================================

type FinishUnitTransformCommand struct {
	BaseCommand
}

func (cmd FinishUnitTransformCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32, unpackInt8, unpackInt8}
	byteLength := 0
	for _, f := range inputTypes {
		byteLength += f()
	}
	enrichBaseCommand(baseCommand, byteLength)
	return FinishUnitTransformCommand{*baseCommand}
}

// ========================================================================
// 25 - setUnitStance
// ========================================================================

type SetUnitStanceCommand struct {
	BaseCommand
}

func (cmd SetUnitStanceCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt8, unpackInt8, unpackInt32}
	byteLength := 0
	for _, f := range inputTypes {
		byteLength += f()
	}
	enrichBaseCommand(baseCommand, byteLength)
	return SetUnitStanceCommand{*baseCommand}
}

// ========================================================================
// 26 - changeDiplomacy
// ========================================================================

type ChangeDiplomacyCommand struct {
	BaseCommand
}

func (cmd ChangeDiplomacyCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt8, unpackInt32}
	byteLength := 0
	for _, f := range inputTypes {
		byteLength += f()
	}
	enrichBaseCommand(baseCommand, byteLength)
	return ChangeDiplomacyCommand{*baseCommand}
}

// ========================================================================
// 34 - townBell
// ========================================================================

type TownBellCommand struct {
	BaseCommand
}

func (cmd TownBellCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	// The townBell command is 8 bytes long, consisting of 2 int32s.
	byteLength := 8
	enrichBaseCommand(baseCommand, byteLength)
	return TownBellCommand{
		BaseCommand: *baseCommand,
	}
}

func (cmd TownBellCommand) Format(input FormatterInput) (ReplayGameCommand, bool) {
	return ReplayGameCommand{
		GameTimeSecs: cmd.GameTimeSecs(),
		PlayerNum:    cmd.PlayerId(),
		CommandType:  "townBell",
		Payload:      strconv.Itoa(cmd.PlayerId()),
	}, true
}

// ========================================================================
// 35 - autoScoutEvent
// ========================================================================

type AutoScoutEventCommand struct {
	BaseCommand
}

func (cmd AutoScoutEventCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	// The autoScoutEvent is 12 bytes long, consisting of 3 int32s.
	byteLength := 12
	enrichBaseCommand(baseCommand, byteLength)
	baseCommand.affectsEAPM = false
	return AutoScoutEventCommand{*baseCommand}
}

// ========================================================================
// 37 - changeControlGroupContents
// ========================================================================

type ChangeControlGroupContentsCommand struct {
	BaseCommand
}

func (cmd ChangeControlGroupContentsCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt8, unpackInt32}
	byteLength := 0
	for _, f := range inputTypes {
		byteLength += f()
	}
	enrichBaseCommand(baseCommand, byteLength)
	// Every time you change a control group, the game triggers one event per unit in the group (removing them) and then readds them all, with 1 event per unit
	// Including this would inflate CPM by a LOT.
	baseCommand.affectsEAPM = false
	return ChangeControlGroupContentsCommand{*baseCommand}
}

// ========================================================================
// 38 - repair
// ========================================================================

type RepairCommand struct {
	BaseCommand
}

func (cmd RepairCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32}
	byteLength := 0
	for _, f := range inputTypes {
		byteLength += f()
	}
	enrichBaseCommand(baseCommand, byteLength)
	return RepairCommand{*baseCommand}
}

// ========================================================================
// 41 - taunt
// ========================================================================

type TauntCommand struct {
	BaseCommand
	tauntId int32
}

func (cmd TauntCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	// The taunt command is 41 bytes long, consisting of 10 int32s, followed by 1 int8.
	// The 3rd int32 is the tauntIdo.
	byteLength := 41
	enrichBaseCommand(baseCommand, byteLength)
	tauntId := readInt32(data, baseCommand.offset+8)
	return TauntCommand{
		BaseCommand: *baseCommand,
		tauntId:     tauntId,
	}
}

func (cmd TauntCommand) Format(input FormatterInput) (ReplayGameCommand, bool) {
	return ReplayGameCommand{
		GameTimeSecs: cmd.GameTimeSecs(),
		PlayerNum:    cmd.PlayerId(),
		CommandType:  "taunt",
		Payload:      strconv.Itoa(int(cmd.tauntId)),
	}, true
}

// ========================================================================
// 44 - cheat
// ========================================================================

type CheatCommand struct {
	BaseCommand
	cheatId int32
}

func (cmd CheatCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	// Cheat command is 16 bytes long, consisting of 4 int32s. The 3rd int32 is the cheatId. Which can be
	// converted to a string via XMB data
	byteLength := 16
	enrichBaseCommand(baseCommand, byteLength)
	cheatId := readInt32(data, baseCommand.offset+8)
	return CheatCommand{
		BaseCommand: *baseCommand,
		cheatId:     cheatId,
	}
}

func (cmd CheatCommand) Format(input FormatterInput) (ReplayGameCommand, bool) {
	// TODO: Look up cheat name in XMB data
	return ReplayGameCommand{
		GameTimeSecs: cmd.GameTimeSecs(),
		PlayerNum:    cmd.PlayerId(),
		CommandType:  "cheat",
		Payload:      strconv.Itoa(int(cmd.cheatId)),
	}, true
}

// ========================================================================
// 45 - cancelQueuedItem
// ========================================================================

type CancelQueuedItemCommand struct {
	BaseCommand
}

func (cmd CancelQueuedItemCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32, unpackInt32, unpackInt32}
	byteLength := 0
	for _, f := range inputTypes {
		byteLength += f()
	}
	enrichBaseCommand(baseCommand, byteLength)
	return CancelQueuedItemCommand{*baseCommand}
}

// ========================================================================
// 48 - setFormation
// ========================================================================

type SetFormationCommand struct {
	BaseCommand
	formation string
}

func (cmd SetFormationCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	// The setFormation command is 16 bytes in length, consisting of 4 int32s. The 3rd int32 is the formationId
	byteLength := 16
	enrichBaseCommand(baseCommand, byteLength)
	formationId := readInt32(data, baseCommand.offset+8)
	var formation string
	switch formationId {
	case 0:
		formation = "line"
	case 1:
		formation = "box"
	case 2:
		formation = "spread"
	default:
		formation = "unknown"
		slog.Warn("Unknown formation", "formationId", formationId)
	}

	return SetFormationCommand{
		BaseCommand: *baseCommand,
		formation:   formation,
	}
}

func (cmd SetFormationCommand) Format(input FormatterInput) (ReplayGameCommand, bool) {
	return ReplayGameCommand{
		GameTimeSecs: cmd.GameTimeSecs(),
		PlayerNum:    cmd.PlayerId(),
		CommandType:  "setFormation",
		Payload:      cmd.formation,
	}, true
}

// ========================================================================
// 53 - startUnitTranform
// ========================================================================

type StartUnitTransformCommand struct {
	BaseCommand
}

func (cmd StartUnitTransformCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32}
	byteLength := 0
	for _, f := range inputTypes {
		byteLength += f()
	}
	enrichBaseCommand(baseCommand, byteLength)
	// debateable, selecting a lot of units and doing this creates one command per unit transformed
	baseCommand.affectsEAPM = false
	return StartUnitTransformCommand{*baseCommand}
}

// ========================================================================
// 55 - Unknown
// ========================================================================

type UnknownCommand55 struct {
	BaseCommand
}

func (cmd UnknownCommand55) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	inputTypes := []func() int{unpackInt32, unpackInt32, unpackVector}
	byteLength := 0
	for _, f := range inputTypes {
		byteLength += f()
	}
	enrichBaseCommand(baseCommand, byteLength)
	return UnknownCommand55{*baseCommand}
}

// ========================================================================
// 66 - Autoqueue
// ========================================================================

type AutoqueueCommand struct {
	BaseCommand
	protoUnitId int32
}

func (cmd AutoqueueCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	// The autoqueue command is 12 bytes in length, consisting of 3 int32s. The last int32 is the protoUnitId.
	// inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32}
	byteLength := 12
	enrichBaseCommand(baseCommand, byteLength)
	protoUnitId := readInt32(data, baseCommand.offset+8)
	return AutoqueueCommand{
		BaseCommand: *baseCommand,
		protoUnitId: protoUnitId,
	}
}

func (cmd AutoqueueCommand) Format(input FormatterInput) (ReplayGameCommand, bool) {
	proto := input.protoRootNode.children[cmd.protoUnitId].attributes["name"]
	return ReplayGameCommand{
		GameTimeSecs: cmd.GameTimeSecs(),
		PlayerNum:    cmd.PlayerId(),
		CommandType:  "autoqueue",
		Payload:      proto,
	}, true
}

// ========================================================================
// 67 - toggleAutoUnitAbility
// ========================================================================

type ToggleAutoUnitAbilityCommand struct {
	BaseCommand
}

func (cmd ToggleAutoUnitAbilityCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt8}
	byteLength := 0
	for _, f := range inputTypes {
		byteLength += f()
	}
	enrichBaseCommand(baseCommand, byteLength)
	return ToggleAutoUnitAbilityCommand{*baseCommand}
}

// ========================================================================
// 68 - timeshift
// ========================================================================

type TimeShiftCommand struct {
	BaseCommand
	location Vector3
}

func (cmd TimeShiftCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	// The timeshift command is 32 bytes in length consisting of 2 int32s and 2 vectors. However, none of these bytes
	// correspond to the command, instead the location of the timeshift is stored in the sourceVectors
	byteLength := 32
	enrichBaseCommand(baseCommand, byteLength)
	location := (*baseCommand.sourceVectors)[0]
	return TimeShiftCommand{
		BaseCommand: *baseCommand,
		location:    location,
	}
}

func (cmd TimeShiftCommand) Format(input FormatterInput) (ReplayGameCommand, bool) {
	return ReplayGameCommand{
		GameTimeSecs: cmd.GameTimeSecs(),
		PlayerNum:    cmd.PlayerId(),
		CommandType:  "timeShift",
		Payload:      cmd.location,
	}, true
}

// ========================================================================
// 69 - buildWallConnector
// ========================================================================

type BuildWallConnectorCommand struct {
	BaseCommand
}

func (cmd BuildWallConnectorCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32, unpackVector, unpackVector}
	byteLength := 0
	for _, f := range inputTypes {
		byteLength += f()
	}
	enrichBaseCommand(baseCommand, byteLength)
	// Making a simple wall puts out a LOT of these.
	baseCommand.affectsEAPM = false
	return BuildWallConnectorCommand{*baseCommand}
}

// ========================================================================
// 71 - seekShelter
// ========================================================================

type SeekShelterCommand struct {
	BaseCommand
}

func (cmd SeekShelterCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	inputTypes := []func() int{unpackInt32, unpackInt32}
	byteLength := 0
	for _, f := range inputTypes {
		byteLength += f()
	}
	enrichBaseCommand(baseCommand, byteLength)
	return SeekShelterCommand{*baseCommand}
}

// ========================================================================
// 72 - prequeueTech
// ========================================================================

type PrequeueTechCommand struct {
	BaseCommand
	techId int32
}

func (cmd PrequeueTechCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	// The prequeTech command is 13 bytes in length, bytes 8-12 are an int32 representing the id of the tech
	// that was prequeued. The id maps to a string via the techtree XMB data stored in the header of the replay.
	// inputTypes := []func() int{unpackInt32, unpackInt32, unpackInt32, unpackInt8}
	byteLength := 13
	enrichBaseCommand(baseCommand, byteLength)
	techId := readInt32(data, baseCommand.offset+8)
	return PrequeueTechCommand{
		BaseCommand: *baseCommand,
		techId:      techId,
	}
}

func (cmd PrequeueTechCommand) Format(input FormatterInput) (ReplayGameCommand, bool) {
	return ReplayGameCommand{
		GameTimeSecs: cmd.GameTimeSecs(),
		PlayerNum:    cmd.PlayerId(),
		CommandType:  "prequeueTech",
		Payload:      input.techTreeRootNode.children[cmd.techId].attributes["name"],
	}, true
}

// ========================================================================
// 75 - prebuyGodPower
// ========================================================================

type PrebuyGodPowerCommand struct {
	BaseCommand
}

func (cmd PrebuyGodPowerCommand) Refine(baseCommand *BaseCommand, data *[]byte) RawGameCommand {
	// The prebuyGodPower command is 16 bytes in length, consisting of 4 int32s. The 3rd xint32 is the protoPowerId.
	byteLength := 16
	enrichBaseCommand(baseCommand, byteLength)
	return PrebuyGodPowerCommand{*baseCommand}
}
