package parser

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

func formatRawDataToReplay(
	slim bool,
	stats bool,
	data *[]byte,
	rootNode *Node,
	profileKeys *map[string]ProfileKey,
	xmbMap *map[string]XmbFile,
	commandList *[]GameCommand,
) (ReplayFormatted, error) {

	buildString, err := readBuildString(data, *rootNode)
	if err != nil {
		return ReplayFormatted{}, err
	}
	slog.Debug(buildString)
	buildNumber := getBuildNumber(buildString)

	godsRootNode, err := parseXmb(data, (*xmbMap)["civs"])
	if err != nil {
		return ReplayFormatted{}, err
	}
	majorGodMap := buildGodMap(&godsRootNode)

	techTreeRootNode, err := parseXmb(data, (*xmbMap)["techtree"])
	if err != nil {
		return ReplayFormatted{}, err
	}

	losingPlayer, err := getLosingPlayer(commandList)
	slog.Debug("Losing player", "losingPlayer", losingPlayer)
	if err != nil {
		return ReplayFormatted{}, err
	}
	gameLengthSecs := (*commandList)[len(*commandList)-1].GameTimeSecs()
	players := getPlayers(profileKeys, &majorGodMap, losingPlayer, gameLengthSecs, commandList, &techTreeRootNode)
	slog.Debug("Game host time", "gameHostTime", (*profileKeys)["gamehosttime"])

	// Find winning team by filtering for winners and taking first player's team
	var winningTeam int
	for _, player := range players {
		if player.Winner {
			winningTeam = player.TeamId
			break
		}
	}

	gameOptions := getGameOptions(profileKeys)
	var gameCommands []ReplayGameCommand
	if !slim {
		protoRootNode, err := parseXmb(data, (*xmbMap)["proto"])
		if err != nil {
			return ReplayFormatted{}, err
		}
		powersRootNode, err := parseXmb(data, (*xmbMap)["powers"])
		if err != nil {
			return ReplayFormatted{}, err
		}

		gameCommands = formatCommandsToReplayFormat(
			commandList,
			&players,
			&techTreeRootNode,
			&protoRootNode,
			&powersRootNode,
		)
	}

	return ReplayFormatted{
		MapName:        (*profileKeys)["gamemapname"].StringVal,
		BuildNumber:    buildNumber,
		BuildString:    buildString,
		ParsedAt:       time.Now(),
		ParserVersion:  "0.1.0",
		GameLengthSecs: (*commandList)[len(*commandList)-1].GameTimeSecs(),
		GameSeed:       int((*profileKeys)["gamerandomseed"].Int32Val),
		WinningTeam:    winningTeam,
		GameOptions:    gameOptions,
		Players:        players,
		GameCommands:   gameCommands,
	}, nil
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

func getBuildNumber(buildString string) int {
	// A typical build string looks like this: "AoMRT_s.exe 512899 //stream/Athens/stable"
	// This function extract the build number from the string, by splitting the string and taking the second element.
	parts := strings.Split(buildString, " ")
	if len(parts) < 2 {
		return -1
	}

	buildNumber, err := strconv.Atoi(parts[1])
	if err != nil {
		return -1
	}
	return buildNumber
}

func formatCommandsToReplayFormat(
	commandList *[]GameCommand,
	players *[]ReplayPlayer,
	techTreeRootNode *XmbNode,
	protoRootNode *XmbNode,
	powers *XmbNode,
) []ReplayGameCommand {
	playerMap := make(map[int]ReplayPlayer)
	for _, player := range *players {
		playerMap[player.PlayerNum] = player
	}
	replayCommands := []ReplayGameCommand{}
	for _, command := range *commandList {
		// This is gross, for now, sorry.
		// TODO: Make this command list formatter better. Should this be a map of command types to formatter functions?
		// Similar to the refiners? Do I just enrich ReplayGameCommand with all optional fields, such as num units?
		if researchCmd, ok := command.(ResearchCommand); ok {

			replayCommands = append(replayCommands, ReplayGameCommand{
				GameTimeSecs: command.GameTimeSecs(),
				CommandType:  "research",
				Value:        techTreeRootNode.children[researchCmd.techId].attributes["name"],
				PlayerNum:    researchCmd.playerId,
			})
		}

		if prequeueTechCmd, ok := command.(PrequeueTechCommand); ok {
			replayCommands = append(replayCommands, ReplayGameCommand{
				GameTimeSecs: command.GameTimeSecs(),
				CommandType:  "prequeueTech",
				Value:        techTreeRootNode.children[prequeueTechCmd.techId].attributes["name"],
				PlayerNum:    prequeueTechCmd.playerId,
			})
		}

		if trainCmd, ok := command.(TrainCommand); ok {
			proto := protoRootNode.children[trainCmd.protoUnitId].attributes["name"]
			replayCommands = append(replayCommands, ReplayGameCommand{
				GameTimeSecs: command.GameTimeSecs(),
				CommandType:  "train",
				Value:        proto,
				PlayerNum:    trainCmd.playerId,
			})
		}

		if autoqueueCmd, ok := command.(AutoqueueCommand); ok {
			proto := protoRootNode.children[autoqueueCmd.protoUnitId].attributes["name"]
			replayCommands = append(replayCommands, ReplayGameCommand{
				GameTimeSecs: command.GameTimeSecs(),
				CommandType:  "autoqueue",
				Value:        proto,
				PlayerNum:    autoqueueCmd.playerId,
			})
		}

		if buildCmd, ok := command.(BuildCommand); ok {
			proto := protoRootNode.children[buildCmd.protoBuildingId].attributes["name"]
			replayCommands = append(replayCommands, ReplayGameCommand{
				GameTimeSecs: command.GameTimeSecs(),
				CommandType:  "build",
				Value:        proto,
				PlayerNum:    buildCmd.playerId,
			})
		}

		if godPowerCmd, ok := command.(ProtoPowerCommand); ok {
			power := powers.children[godPowerCmd.protoPowerId]
			var commandType string
			if _, ok := power.attributes["godpower"]; ok {
				commandType = "godPower"
			} else {
				commandType = "protoPower"
			}

			replayCommands = append(replayCommands, ReplayGameCommand{
				GameTimeSecs: command.GameTimeSecs(),
				CommandType:  commandType,
				Value:        power.attributes["name"],
				PlayerNum:    godPowerCmd.playerId,
			})
		}
	}
	if len(*commandList) > 0 {
		lastCommand := (*commandList)[len(*commandList)-1]
		slog.Debug("Last command", "playerId", lastCommand.PlayerId(), "commandType", lastCommand.CommandType())
	}
	return replayCommands
}

func getLosingPlayer(commandList *[]GameCommand) (int, error) {
	// TODO: Make this robust to team games, right now this assumes a 1v1 game, determine the losing player based on
	// the last command in the command list, which is, at the moment, always a resign command
	lastCommand := (*commandList)[len(*commandList)-1]
	if lastCommand.CommandType() != 16 {
		return -1, errors.New("last command is not a resign command")
	}
	return lastCommand.PlayerId(), nil
}

func buildGodMap(godRootNode *XmbNode) map[int]string {
	// Constructs a map of god id (index the god appears at in the XMB data) to god name
	godMap := make(map[int]string)
	godMap[0] = "Nature"
	godId := 1
	for _, child := range godRootNode.children {
		if child.elementName == "civ" {
			for _, elem := range child.children {
				if elem.elementName == "name" {
					godMap[godId] = elem.value
					godId++
				}
			}
		}
	}
	return godMap
}

func getPlayers(
	profileKeys *map[string]ProfileKey,
	majorGodMap *map[int]string,
	losingPlayer int,
	gameLengthSecs float64,
	commandList *[]GameCommand,
	techTreeRootNode *XmbNode,
) []ReplayPlayer {
	// Create a players slice, but checking if each player number exists in the profile keys. If it does, grab
	// the relevant keys from the profileKeys map to construct a ReplayPlayer.
	players := make([]ReplayPlayer, 0)
	for playerNum := 1; playerNum <= 12; playerNum++ {
		if playerExists(profileKeys, playerNum) {
			slog.Debug("Parsing player", "playerNum", playerNum)
			keys := *profileKeys
			playerPrefix := fmt.Sprintf("gameplayer%d", playerNum)
			profileId, err := strconv.Atoi(keys[fmt.Sprintf("%srlinkid", playerPrefix)].StringVal)
			if err != nil {
				slog.Error("Error parsing profile id", "error", err)
				continue
			}
			minorGods := getMinorGods(playerNum, commandList, techTreeRootNode)
			eAPM := getEAPM(playerNum, commandList, gameLengthSecs)
			players = append(players, ReplayPlayer{
				PlayerNum: playerNum,
				TeamId:    int(keys[fmt.Sprintf("%steamid", playerPrefix)].Int32Val),
				Name:      keys[fmt.Sprintf("%sname", playerPrefix)].StringVal,
				ProfileId: profileId,
				Color:     int(keys[fmt.Sprintf("%scolor", playerPrefix)].Int32Val),
				RandomGod: keys[fmt.Sprintf("%scivwasrandom", playerPrefix)].BoolVal,
				God:       (*majorGodMap)[int(keys[fmt.Sprintf("%sciv", playerPrefix)].Int32Val)],
				// TODO: Make this robust to team games, right now this assumes a 1v1 game
				Winner:    playerNum != losingPlayer,
				EAPM:      eAPM,
				MinorGods: minorGods,
			})
		}
	}
	return players
}

func playerExists(profileKeys *map[string]ProfileKey, playerNum int) bool {
	// If a player's pfentity key is populated in the profile keys, then the player exists.
	playerKey := fmt.Sprintf("gameplayer%dpfentity", playerNum)
	return (*profileKeys)[playerKey].StringVal != ""
}

func getMinorGods(playerNum int, commandList *[]GameCommand, techTreeRootNode *XmbNode) [3]string {
	// Filter to all Research/prequeue techs that are Age Up tech,
	ageUpTechs := []string{}
	for _, command := range *commandList {
		if command.PlayerId() != playerNum {
			continue
		}

		if researchCmd, ok := command.(ResearchCommand); ok {
			tech := techTreeRootNode.children[researchCmd.techId].attributes["name"]
			if isAgeUpTech(tech) {
				ageUpTechs = append(ageUpTechs, tech)
			}
		} else if prequeueTechCmd, ok := command.(PrequeueTechCommand); ok {
			tech := techTreeRootNode.children[prequeueTechCmd.techId].attributes["name"]
			if isAgeUpTech(tech) {
				ageUpTechs = append(ageUpTechs, tech)
			}
		}
	}

	slog.Debug("Age up techs", "playerNum", playerNum, "techs", ageUpTechs)

	// Find the last occurrence of each age type and removes the prefix to make it look pretty
	var classical, heroic, mythic string
	for _, tech := range ageUpTechs {
		if strings.HasPrefix(tech, "ClassicalAge") {
			classical = strings.TrimPrefix(tech, "ClassicalAge")
		} else if strings.HasPrefix(tech, "HeroicAge") {
			heroic = strings.TrimPrefix(tech, "HeroicAge")
		} else if strings.HasPrefix(tech, "MythicAge") {
			mythic = strings.TrimPrefix(tech, "MythicAge")
		}
	}

	return [3]string{classical, heroic, mythic}
}

func getEAPM(playerNum int, commandList *[]GameCommand, gameLengthSecs float64) float64 {
	actions := 0
	for _, command := range *commandList {
		if command.PlayerId() == playerNum && command.AffectsEAPM() {
			actions += 1
		}
	}

	gameLengthMins := gameLengthSecs / 60.0
	return float64(actions) / gameLengthMins
}

func isAgeUpTech(value string) bool {
	// If it starts with Classical, Heroic, or Mythic Age, return true
	ageUpPrefixes := []string{"ClassicalAge", "HeroicAge", "MythicAge"}
	for _, prefix := range ageUpPrefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func printXmbNode(node *XmbNode) {
	// Recursively prints the XMB node and its children, useful for debugging
	slog.Debug("XMB Node", "elementName", node.elementName, "value", node.value, "attributes", node.attributes)
	for _, child := range node.children {
		printXmbNode(child)
	}
}

func getGameOptions(profileKeys *map[string]ProfileKey) map[string]bool {
	keys := []string{
		"gameaivsai",
		"gameallowaiassist",
		"gameallowcheats",
		"gameallowtitans",
		"gameblockade",
		"gameconquest",
		"gamecontrolleronly",
		"gamefreeforall",
		"gameismpcoop",
		"gameismpscenario",
		"gamekoth",
		"gameludicrousmode",
		"gamemaprecommendedsettings",
		"gamemilitaryautoqueue",
		"gamenomadstart",
		"gameonevsall",
		"gameregicide",
		"gamerestored",
		"gamerestrictpause",
		"gamermdebug",
		"gamestorymode",
		"gamesuddendeath",
		"gameteambalanced",
		"gameteamlock",
		"gameteamsharepop",
		"gameteamshareres",
		"gameteamvictory",
		"gameusedenforcedagesettings",
	}
	gameOptions := make(map[string]bool)
	for _, key := range keys {
		gameOptions[key] = (*profileKeys)[key].BoolVal
	}
	return gameOptions
}
