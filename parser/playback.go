package parser

func ParsePlaybackToJson(replayPath string, prettyPrint bool, isGzip bool) (string, error) {
	playback, err := ParsePlayback(replayPath, isGzip)
	if err != nil {
		return "", err
	}
	return marshalJson(playback, prettyPrint)
}

// ParsePlayback extracts what a web player needs to redraw a match: the terrain and objects at game start, plus
// every command with where on the map it happened.
func ParsePlayback(replayPath string, isGzip bool) (ReplayPlayback, error) {
	replay, err := decodeReplay(replayPath, isGzip)
	if err != nil {
		return ReplayPlayback{}, err
	}

	formatted, err := formatRawDataToReplay(
		true,
		false,
		&replay.data,
		&replay.rootNode,
		&replay.profileKeys,
		&replay.xmbMap,
		&replay.commandList,
	)
	if err != nil {
		return ReplayPlayback{}, err
	}

	input, err := parseFormatterInput(&replay.data, &replay.xmbMap)
	if err != nil {
		return ReplayPlayback{}, err
	}

	replayMap, err := parseTerrain(&replay.data, replay.rootNode)
	if err != nil {
		return ReplayPlayback{}, err
	}

	entities, err := parseEntities(&replay.data, replay.rootNode, input.protoRootNode)
	if err != nil {
		return ReplayPlayback{}, err
	}

	return ReplayPlayback{
		ParserVersion:  VERSION,
		MapName:        formatted.MapName,
		BuildNumber:    formatted.BuildNumber,
		GameLengthSecs: formatted.GameLengthSecs,
		Players:        formatted.Players,
		Map:            replayMap,
		Entities:       entities,
		Events:         formatPlaybackEvents(&replay.commandList, input),
	}, nil
}

func parseFormatterInput(data *[]byte, xmbMap *map[string]XmbFile) (FormatterInput, error) {
	nodes := make(map[string]*XmbNode)
	for _, name := range []string{"proto", "techtree", "powers"} {
		node, err := parseXmbIfPresent(data, xmbMap, name)
		if err != nil {
			return FormatterInput{}, err
		}
		nodes[name] = &node
	}
	return FormatterInput{
		protoRootNode:    nodes["proto"],
		techTreeRootNode: nodes["techtree"],
		powersRootNode:   nodes["powers"],
	}, nil
}

func formatPlaybackEvents(commandList *[]RawGameCommand, input FormatterInput) []PlaybackEvent {
	events := make([]PlaybackEvent, len(*commandList))
	for i, command := range *commandList {
		events[i] = formatPlaybackEvent(command, input)
	}
	return events
}

func formatPlaybackEvent(command RawGameCommand, input FormatterInput) PlaybackEvent {
	event := PlaybackEvent{
		GameTimeSecs: command.GameTimeSecs(),
		PlayerNum:    command.PlayerId(),
		CommandType:  commandTypeNames[command.CommandType()],
		Location:     commandLocation(command),
		Units:        command.SourceUnits(),
	}
	// Reuse Format so names match the parse output, e.g. godPower vs protoPower and XMB-resolved unit names.
	if formatted, ok := command.Format(input); ok {
		event.CommandType = formatted.CommandType
		event.Name = payloadName(formatted)
	}
	if task, ok := command.(TaskCommand); ok && task.targetId >= 0 {
		event.TargetId = &task.targetId
	}
	return event
}

func payloadName(formatted ReplayGameCommand) string {
	// resign and townBell use the player number as their payload, which the event already carries.
	if formatted.CommandType == "resign" || formatted.CommandType == "townBell" {
		return ""
	}
	switch payload := formatted.Payload.(type) {
	case string:
		return payload
	case BuildCommandPaylod:
		return payload.Name
	case ProtoPowerPayload:
		return payload.Name
	}
	return ""
}

// commandLocation returns where on the map a command happened: the command's own target when it has one, else the
// point the player clicked. Commands with neither, like train or research, return nil.
func commandLocation(command RawGameCommand) *Vector3 {
	if target, ok := targetLocation(command); ok && isOnMap(target) {
		return &target
	}
	if vectors := command.SourceVectors(); len(vectors) > 0 && isOnMap(vectors[0]) {
		return &vectors[0]
	}
	return nil
}

func targetLocation(command RawGameCommand) (Vector3, bool) {
	switch cmd := command.(type) {
	case TaskCommand:
		return cmd.targetLocation, true
	case BuildCommand:
		return cmd.location, true
	case ProtoPowerCommand:
		return cmd.location1, true
	case SetGatherPointCommand:
		return cmd.location, true
	case BuildWallConnectorCommand:
		return cmd.location, true
	}
	return Vector3{}, false
}

// isOnMap rejects the (-1, -1, -1) sentinel commands use when they have no location.
func isOnMap(location Vector3) bool {
	return location.X >= 0 && location.Z >= 0
}
