package parser

import (
	"reflect"
	"testing"
)

var offMap = Vector3{X: -1, Y: -1, Z: -1}

func testBase(commandType int, playerId int, units []uint32, vectors ...Vector3) BaseCommand {
	preArgumentBytes := []uint8{}
	return newBaseCommand(0, commandType, playerId, 40, &units, &vectors, &preArgumentBytes)
}

func int32Ptr(v int32) *int32 {
	return &v
}

func TestFormatPlaybackEvent(t *testing.T) {
	input := FormatterInput{
		protoRootNode:    protoCatalog("House", "VillagerGreek"),
		techTreeRootNode: &XmbNode{},
		powersRootNode: &XmbNode{children: []*XmbNode{
			{attributes: map[string]string{"name": "Bolt", "godpower": "1"}},
			{attributes: map[string]string{"name": "ShiftingSands", "godpower": "1"}},
		}},
	}
	tests := []struct {
		name    string
		command RawGameCommand
		want    PlaybackEvent
	}{
		{
			name: "task on the ground uses the clicked point",
			command: TaskCommand{
				BaseCommand:    testBase(0, 1, []uint32{71, 72}, Vector3{X: 10, Z: 20}),
				targetId:       -1,
				targetLocation: offMap,
			},
			want: PlaybackEvent{CommandType: "task", Location: &Vector3{X: 10, Z: 20}, Units: []uint32{71, 72}},
		},
		{
			name: "task on an entity uses the target's position",
			command: TaskCommand{
				BaseCommand:    testBase(0, 1, []uint32{71}, Vector3{X: 31, Z: 41}),
				targetId:       5,
				targetLocation: Vector3{X: 30, Y: 1, Z: 40},
			},
			want: PlaybackEvent{
				CommandType: "task",
				Location:    &Vector3{X: 30, Y: 1, Z: 40},
				TargetId:    int32Ptr(5),
				Units:       []uint32{71},
			},
		},
		{
			name: "build names the building",
			command: BuildCommand{
				BaseCommand:     testBase(3, 1, []uint32{80}),
				protoBuildingId: 0,
				location:        Vector3{X: 50, Y: 2, Z: 60},
			},
			want: PlaybackEvent{CommandType: "build", Name: "House", Location: &Vector3{X: 50, Y: 2, Z: 60}, Units: []uint32{80}},
		},
		{
			name: "god power without a target falls back to the clicked point",
			command: ProtoPowerCommand{
				BaseCommand:  testBase(12, 1, []uint32{1}, Vector3{X: 5, Z: 6}),
				protoPowerId: 0,
				location1:    offMap,
				location2:    offMap,
			},
			want: PlaybackEvent{CommandType: "godPower", Name: "Bolt", Location: &Vector3{X: 5, Z: 6}, Units: []uint32{1}},
		},
		{
			name: "two-point god power keeps both locations",
			command: ProtoPowerCommand{
				BaseCommand:  testBase(12, 1, []uint32{1}),
				protoPowerId: 1,
				location1:    Vector3{X: 5, Z: 6},
				location2:    Vector3{X: 90, Z: 120},
			},
			want: PlaybackEvent{
				CommandType: "godPower",
				Name:        "ShiftingSands",
				Location:    &Vector3{X: 5, Z: 6},
				Location2:   &Vector3{X: 90, Z: 120},
				Units:       []uint32{1},
			},
		},
		{
			name: "gather point uses its own location",
			command: SetGatherPointCommand{
				BaseCommand: testBase(4, 1, []uint32{0}),
				location:    Vector3{X: 7, Z: 8},
			},
			want: PlaybackEvent{CommandType: "setGatherPoint", Location: &Vector3{X: 7, Z: 8}, Units: []uint32{0}},
		},
		{
			name: "train has no location",
			command: TrainCommand{
				BaseCommand: testBase(2, 1, []uint32{0}),
				protoUnitId: 1,
				numUnits:    1,
			},
			want: PlaybackEvent{CommandType: "train", Name: "VillagerGreek", Units: []uint32{0}},
		},
		{
			name:    "resign leaves the name empty",
			command: ResignCommand{testBase(16, 1, nil)},
			want:    PlaybackEvent{CommandType: "resign"},
		},
		{
			name:    "unformatted commands take their registered name",
			command: StopCommand{testBase(9, 1, []uint32{71}, Vector3{X: 1, Z: 2})},
			want:    PlaybackEvent{CommandType: "stop", Location: &Vector3{X: 1, Z: 2}, Units: []uint32{71}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.want.GameTimeSecs = 2
			tt.want.PlayerNum = 1
			got := formatPlaybackEvent(tt.command, input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("formatPlaybackEvent() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestEveryRegisteredCommandHasAName(t *testing.T) {
	for commandType := range CommandFactoryInstance.refiners {
		if commandTypeNames[commandType] == "" {
			t.Errorf("command type %v is registered but missing from commandTypeNames", commandType)
		}
	}
}

func TestReadVectorDecodesFloats(t *testing.T) {
	data := (&byteWriter{}).vec(229, 4.5, -1).buf
	if got := readVector(&data, 0); got != (Vector3{X: 229, Y: 4.5, Z: -1}) {
		t.Errorf("readVector() = %+v, want {229 4.5 -1}", got)
	}
}
