package parser

import (
	"log/slog"
)

// parseEntities decodes BG/J1/Z1, every object on the map at game start: trees, mines, hunt, settlements, and
// each player's starting buildings. Starting villagers are not in the snapshot.
//
// Z1 layout (build 606844): uint32 entity slot count, uint32 unknown, then one record per live entity. A record
// is an H1 node whose first child is EN, followed by P1:
//
//	EN   int32 id, uint32 unknown, uint32 owner, ..., float32 x, y, z at +32
//	P1   uint32 proto id
//
// We don't understand the bytes between records (per-entity trailing data and zero-filled deleted slots), so
// this scans for the H1-then-EN signature instead of walking a fixed stride.
func parseEntities(data *[]byte, rootNode Node, protoRootNode *XmbNode) ([]ReplayEntity, error) {
	z1, err := rootNode.getSingleChild("J1", "Z1")
	if err != nil {
		return nil, err
	}

	entities := make([]ReplayEntity, 0)
	end := z1.endOffset()
	position := z1.offset + DATA_OFFSET + 8
	for position+2*DATA_OFFSET <= end {
		if !isEntityRecord(data, position) {
			position++
			continue
		}

		recordEnd := position + DATA_OFFSET + int(readUint32(data, position+2))
		enBody := position + 2*DATA_OFFSET
		p1 := enBody + int(readUint32(data, position+DATA_OFFSET+2))
		if recordEnd > end || p1+DATA_OFFSET+4 > recordEnd || string((*data)[p1:p1+2]) != "P1" {
			slog.Debug("Skipping malformed entity record", "offset", position)
			position++
			continue
		}

		location := readVector(data, enBody+32)
		entities = append(entities, ReplayEntity{
			Id:        readInt32(data, enBody),
			PlayerNum: int(readUint32(data, enBody+8)),
			Name:      protoName(protoRootNode, readInt32(data, p1+DATA_OFFSET)),
			X:         location.X,
			Z:         location.Z,
		})
		position = recordEnd
	}
	return entities, nil
}

func isEntityRecord(data *[]byte, position int) bool {
	return string((*data)[position:position+2]) == "H1" &&
		string((*data)[position+DATA_OFFSET:position+DATA_OFFSET+2]) == "EN"
}
