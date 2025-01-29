package parser

import "errors"

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
	Refine(baseCommand BaseCommand, data *[]byte) RawGameCommand
	Format(baseCommand ReplayGameCommand, input FormatterInput) ReplayGameCommand
}

type CommandRefiner func(command BaseCommand) RawGameCommand

type BaseCommand struct {
	commandType  int
	playerId     int
	offset       int
	offsetEnd    int
	byteLength   int
	gameTimeSecs float64
	affectsEAPM  bool
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

func (cmd BaseCommand) Refine(baseCommand BaseCommand, data *[]byte) RawGameCommand {
	return errors.New("Refine not implemented for BaseCommand")
}

func (cmd BaseCommand) Format(baseCommand ReplayGameCommand, input FormatterInput) ReplayGameCommand {
	return ReplayGameCommand{
		GameTimeSecs: cmd.gameTimeSecs,
		CommandType:  "unknown",
		PlayerNum:    cmd.playerId,
		Value:        "unknown",
	}
}

type ResearchCommand struct {
	BaseCommand
	techId int32
}

type PrequeueTechCommand struct {
	BaseCommand
	techId int32
}

type TrainCommand struct {
	BaseCommand
	protoUnitId int32
	numUnits    int8
}

type AutoqueueCommand struct {
	BaseCommand
	protoUnitId int32
}

type BuildCommand struct {
	BaseCommand
	protoBuildingId int32
	queued          bool
	location        Vector3
}

type ProtoPowerCommand struct {
	BaseCommand
	protoPowerId int32
}

type TauntCommand struct {
	BaseCommand
	tauntId int32
}

type Action string

const (
	BuyAction  Action = "buy"
	SellAction Action = "sell"
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
	action       Action
	quantity     float32
}

type TownBellCommand struct {
	BaseCommand
}
