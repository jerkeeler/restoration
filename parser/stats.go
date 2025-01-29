package parser

import (
	"math"
)

// What stats do I want?
// I want:
// 1. Resources traded by player
// 2. Total unit counts by player
// 3. Total building counts by player
// 4. GodPower usage by player???
// 5. Techs researched
// 6. Timelines:
//   a. Bucketed to 1 minute intervals
//   b. Player unit counts
//   c. Player building counts
//   d. Prequeued techs
//   e. Researched techs
// 7. eAPM throughout the game

func calcStats(commandList *[]ReplayGameCommand, rawCommandList *[]RawGameCommand) *map[int]ReplayStats {
	statsByPlayer := make(map[int]ReplayStats)
	commandsByPlayer := make(map[int][]ReplayGameCommand)
	for _, command := range *commandList {
		if _, exists := commandsByPlayer[command.PlayerNum]; !exists {
			commandsByPlayer[command.PlayerNum] = make([]ReplayGameCommand, 0)
		}
		commandsByPlayer[command.PlayerNum] = append(commandsByPlayer[command.PlayerNum], command)
	}

	rawCommandsByPlayer := make(map[int][]RawGameCommand)
	for _, command := range *rawCommandList {
		if _, exists := commandsByPlayer[command.PlayerId()]; !exists {
			rawCommandsByPlayer[command.PlayerId()] = make([]RawGameCommand, 0)
		}
		rawCommandsByPlayer[command.PlayerId()] = append(rawCommandsByPlayer[command.PlayerId()], command)
	}

	for playerNum, commands := range commandsByPlayer {
		statsByPlayer[playerNum] = calcStatsForPlayer(&commands, rawCommandsByPlayer[playerNum])
	}
	return &statsByPlayer
}

func calcStatsForPlayer(playerCommandList *[]ReplayGameCommand, rawPlayerCommandList []RawGameCommand) ReplayStats {
	totals := calcTotals(playerCommandList)
	timelines := calcTimelines(playerCommandList)

	return ReplayStats{
		Trade: TradeStats{
			ResourcesSold:   totals.Trade.ResourcesSold,
			ResourcesBought: totals.Trade.ResourcesBought,
		},
		UnitCounts:      totals.UnitCounts,
		BuildingCounts:  totals.BuildingCounts,
		GodPowerCounts:  totals.GodPowerCounts,
		TechsResearched: totals.TechsResearched,
		FormationCounts: totals.FormationCounts,
		EAPM:            calcEAPMOverTime(&rawPlayerCommandList),
		Timelines:       timelines.Timelines,
	}
}

func calcTotals(playerCommandList *[]ReplayGameCommand) ReplayStats {
	resourcesSold := make(map[string]float32)
	resourcesBought := make(map[string]float32)
	unitCounts := make(map[string]int)
	buildingCounts := make(map[string]int)
	godPowerCounts := make(map[string]int)
	techsResearched := make([]string, 0)
	seenTechs := make(map[string]bool)
	formationCounts := make(map[string]int)

	for _, command := range *playerCommandList {
		handleMarketBuySell("sell", &command, &resourcesSold)
		handleMarketBuySell("buy", &command, &resourcesBought)
		handleUnitCounts(&command, &unitCounts)
		handleBuildingCounts(&command, &buildingCounts)
		handleGodPowerCounts(&command, &godPowerCounts)
		handleFormationCount(&command, &formationCounts)
		techsResearched = techsResearch(&command, techsResearched, &seenTechs)
	}

	return ReplayStats{
		Trade: TradeStats{
			ResourcesSold:   resourcesSold,
			ResourcesBought: resourcesBought,
		},
		UnitCounts:      unitCounts,
		BuildingCounts:  buildingCounts,
		GodPowerCounts:  godPowerCounts,
		FormationCounts: formationCounts,
		TechsResearched: techsResearched,
	}
}

func calcTimelines(playerCommandList *[]ReplayGameCommand) ReplayStats {
	lastCommand := (*playerCommandList)[len(*playerCommandList)-1]
	minutes := int(math.Ceil(lastCommand.GameTimeSecs / 60.0))
	unitCounts := make([]map[string]int, minutes)
	buildingCounts := make([]map[string]int, minutes)
	techsPrequeued := make([]TechItem, 0)
	techsResearched := make([]TechItem, 0)
	godPowers := make([]GodPowerItem, 0)

	for _, command := range *playerCommandList {
		commandMinute := int(math.Ceil(command.GameTimeSecs / 60.0))
		if unitCounts[commandMinute-1] == nil {
			unitCounts[commandMinute-1] = make(map[string]int)
		}
		if buildingCounts[commandMinute-1] == nil {
			buildingCounts[commandMinute-1] = make(map[string]int)
		}
		handleUnitCounts(&command, &unitCounts[commandMinute-1])
		handleBuildingCounts(&command, &buildingCounts[commandMinute-1])
		techsPrequeued = getResearchOrPrequeue(&command, techsPrequeued, "prequeueTech")
		techsResearched = getResearchOrPrequeue(&command, techsResearched, "research")
		godPowers = handleGodPower(&command, godPowers)
	}

	return ReplayStats{
		Timelines: Timelines{
			UnitCounts:      unitCounts,
			BuildingCounts:  buildingCounts,
			TechsPrequeued:  techsPrequeued,
			TechsResearched: techsResearched,
			GodPowers:       godPowers,
		},
	}
}

func handleMarketBuySell(buySell string, command *ReplayGameCommand, resources *map[string]float32) {
	if command.CommandType == "marketBuySell" {
		payload := command.Payload.(BuySellResourcesPayload)
		if payload.Action == buySell {
			if _, exists := (*resources)[payload.ResourceType]; !exists {
				(*resources)[payload.ResourceType] = 0
			}
			(*resources)[payload.ResourceType] += payload.Quantity
		}
	}
}

func handleUnitCounts(command *ReplayGameCommand, unitCounts *map[string]int) {
	if command.CommandType == "train" {
		payload := command.Payload.(string)
		if _, exists := (*unitCounts)[payload]; !exists {
			(*unitCounts)[payload] = 0
		}
		(*unitCounts)[payload] += 1
	}
}

func handleBuildingCounts(command *ReplayGameCommand, buildingCounts *map[string]int) {
	if command.CommandType == "build" {
		payload := command.Payload.(BuildCommandPaylod)
		if _, exists := (*buildingCounts)[payload.Name]; !exists {
			(*buildingCounts)[payload.Name] = 0
		}
		(*buildingCounts)[payload.Name] += 1
	}
}

func handleGodPowerCounts(command *ReplayGameCommand, godPowerCounts *map[string]int) {
	if command.CommandType == "godPower" {
		payload := command.Payload.(ProtoPowerPayload)
		if _, exists := (*godPowerCounts)[payload.Name]; !exists {
			(*godPowerCounts)[payload.Name] = 0
		}
		(*godPowerCounts)[payload.Name] += 1
	}
}

func handleFormationCount(command *ReplayGameCommand, formationCount *map[string]int) {
	if command.CommandType == "setFormation" {
		payload := command.Payload.(string)
		if _, exists := (*formationCount)[payload]; !exists {
			(*formationCount)[payload] = 0
		}
		(*formationCount)[payload] += 1
	}
}

func techsResearch(command *ReplayGameCommand, techsResearched []string, seenTechs *map[string]bool) []string {
	if command.CommandType == "research" || command.CommandType == "prequeueTech" {
		payload := command.Payload.(string)
		if _, exists := (*seenTechs)[payload]; !exists {
			(*seenTechs)[payload] = true
			techsResearched = append(techsResearched, payload)
		}
	}
	return techsResearched
}

func getResearchOrPrequeue(command *ReplayGameCommand, techsResearched []TechItem, matching string) []TechItem {
	if command.CommandType == matching {
		payload := command.Payload.(string)
		techsResearched = append(techsResearched, TechItem{
			Name:         payload,
			GameTimeSecs: command.GameTimeSecs,
		})
	}
	return techsResearched
}

func handleGodPower(command *ReplayGameCommand, godPowers []GodPowerItem) []GodPowerItem {
	if command.CommandType == "godPower" {
		payload := command.Payload.(ProtoPowerPayload)
		godPowers = append(godPowers, GodPowerItem{
			Name:         payload.Name,
			GameTimeSecs: command.GameTimeSecs,
		})
	}
	return godPowers
}

func calcEAPMOverTime(rawCommandList *[]RawGameCommand) []float64 {
	lastCommand := (*rawCommandList)[len(*rawCommandList)-1]
	minutes := int(math.Ceil(lastCommand.GameTimeSecs() / 60.0))
	eapm := make([]float64, minutes)

	for _, command := range *rawCommandList {
		commandMinute := int(math.Ceil(command.GameTimeSecs() / 60.0))
		eapm[commandMinute-1] += 1
	}

	return eapm
}
