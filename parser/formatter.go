package parser

import (
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
	commandList *[]RawGameCommand,
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

	losingTeams, err := getLosingTeams(commandList, profileKeys)
	slog.Debug("Losing teams", "losingTeams", losingTeams)
	if err != nil {
		return ReplayFormatted{}, err
	}
	gameLengthSecs := (*commandList)[len(*commandList)-1].GameTimeSecs()
	players := getPlayers(profileKeys, &majorGodMap, losingTeams, gameLengthSecs, commandList, &techTreeRootNode)
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
	addTechsToPlayers(&players, &gameCommands)

	formattedReplay := ReplayFormatted{
		MapName:        (*profileKeys)["gamemapname"].StringVal,
		BuildNumber:    buildNumber,
		BuildString:    buildString,
		ParsedAt:       time.Now(),
		ParserVersion:  VERSION,
		GameLengthSecs: (*commandList)[len(*commandList)-1].GameTimeSecs(),
		GameSeed:       int((*profileKeys)["gamerandomseed"].Int32Val),
		WinningTeam:    winningTeam,
		GameOptions:    gameOptions,
		Players:        players,
	}
	if !slim {
		formattedReplay.GameCommands = &gameCommands
	}
	if stats {
		formattedReplay.Stats = calcStats(&gameCommands, commandList)
	}

	return formattedReplay, nil
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
	commandList *[]RawGameCommand,
	players *[]ReplayPlayer,
	techTreeRootNode *XmbNode,
	protoRootNode *XmbNode,
	powers *XmbNode,
) []ReplayGameCommand {
	playerMap := make(map[int]ReplayPlayer)
	for _, player := range *players {
		playerMap[player.PlayerNum] = player
	}
	var replayCommands []ReplayGameCommand
	formatterInput := FormatterInput{
		protoRootNode:    protoRootNode,
		techTreeRootNode: techTreeRootNode,
		powersRootNode:   powers,
	}
	for _, command := range *commandList {
		formattedCommand, ok := command.Format(formatterInput)
		if ok {
			replayCommands = append(replayCommands, formattedCommand)
		}
	}
	if len(*commandList) > 0 {
		lastCommand := (*commandList)[len(*commandList)-1]
		slog.Debug("Last command", "playerId", lastCommand.PlayerId(), "commandType", lastCommand.CommandType())
	}
	return replayCommands
}

func getLosingTeams(commandList *[]RawGameCommand, profileKeys *map[string]ProfileKey) (map[int]bool, error) {
	// Gets all resign commands and returns the set of team ids of the players who resigned
	resigningPlayers := make(map[int]bool)

	// Find all players who issued resign commands
	for _, command := range *commandList {
		if command.CommandType() == 16 { // 16 is resign command type
			resigningPlayers[command.PlayerId()] = true
		}
	}

	// If no one resigned, return error
	if len(resigningPlayers) == 0 {
		return nil, fmt.Errorf("no resign commands found")
	}

	// Use a map to deduplicate team IDs
	teamIds := make(map[int]bool)

	// Get team IDs for all resigning players
	for playerId := range resigningPlayers {
		playerPrefix := fmt.Sprintf("gameplayer%d", playerId)
		teamIdKey := fmt.Sprintf("%steamid", playerPrefix)

		if teamId, ok := (*profileKeys)[teamIdKey]; ok {
			teamIds[int(teamId.Int32Val)] = true
		}
	}

	if len(teamIds) == 0 {
		return nil, fmt.Errorf("could not find team IDs for resigning players")
	}

	return teamIds, nil
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
	losingTeams map[int]bool,
	gameLengthSecs float64,
	commandList *[]RawGameCommand,
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
			teamId := int(keys[fmt.Sprintf("%steamid", playerPrefix)].Int32Val)
			if err != nil {
				slog.Error("Error parsing profile id", "error", err)
				continue
			}
			minorGods := getMinorGods(playerNum, commandList, techTreeRootNode)
			eAPM := getEAPM(playerNum, commandList, gameLengthSecs)
			players = append(players, ReplayPlayer{
				PlayerNum: playerNum,
				TeamId:    teamId,
				Name:      keys[fmt.Sprintf("%sname", playerPrefix)].StringVal,
				ProfileId: profileId,
				Color:     int(keys[fmt.Sprintf("%scolor", playerPrefix)].Int32Val),
				RandomGod: keys[fmt.Sprintf("%scivwasrandom", playerPrefix)].BoolVal,
				God:       (*majorGodMap)[int(keys[fmt.Sprintf("%sciv", playerPrefix)].Int32Val)],
				// TODO: Make this robust to team games, right now this assumes a 1v1 game
				Winner:    !losingTeams[teamId],
				EAPM:      eAPM,
				MinorGods: minorGods,
				CivList:   keys[fmt.Sprintf("%scivlist", playerPrefix)].StringVal,
			})
		}
	}
	return players
}

func playerExists(profileKeys *map[string]ProfileKey, playerNum int) bool {
	// If a player's pfentity key is populated in the profile keys, then the player exists.
	playerKey := fmt.Sprintf("gameplayer%dname", playerNum)
	return (*profileKeys)[playerKey].StringVal != ""
}

func getMinorGods(playerNum int, commandList *[]RawGameCommand, techTreeRootNode *XmbNode) [3]string {
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

func getEAPM(playerNum int, commandList *[]RawGameCommand, gameLengthSecs float64) float64 {
	// Track whether we've counted an action for each timestamp+command type combination
	actions := 0

	for _, command := range *commandList {
		if command.PlayerId() == playerNum && command.AffectsEAPM() {
			actions++
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

func addTechsToPlayers(players *[]ReplayPlayer, gameCommands *[]ReplayGameCommand) {
	slog.Debug("Adding techs to players")
	playerTechs := make(map[int][]interface{})
	for _, command := range *gameCommands {
		if command.CommandType == "godPower" {
			payload := command.Payload.(ProtoPowerPayload)
			if payload.Name == "TitanGate" {
				playerTechs[command.PlayerNum] = append(playerTechs[command.PlayerNum], payload.Name)
			}
		} else if command.CommandType == "build" {
			payload := command.Payload.(BuildCommandPaylod)
			if payload.Name == "Wonder" {
				playerTechs[command.PlayerNum] = append(playerTechs[command.PlayerNum], payload.Name)
			}
		}
	}

	// Use index-based iteration to modify the actual players
	for i := range *players {
		for _, tech := range playerTechs[(*players)[i].PlayerNum] {
			if tech == "TitanGate" {
				slog.Debug("Player has TitanGate", "playerNum", (*players)[i].PlayerNum)
				(*players)[i].Titan = true
			} else if tech == "Wonder" {
				slog.Debug("Player has Wonder", "playerNum", (*players)[i].PlayerNum)
				(*players)[i].Wonder = true
			}
		}
	}
}
