package parser

const MAX_SCAN_OFFSET = 50
const DATA_OFFSET = 6
const ROOT_NODE_TOKEN = "BG"
const VERSION = "v0.3.1"

var FOOTER = []uint8{0x19, 0x0, 0x0, 0x0}

var NODES_WITH_SUBSTRUCTURE = map[string]struct{}{
	"BG": {},
	"J1": {},
	"PL": {},
	"BP": {},
	"MP": {},
	"GM": {},
	"GD": {},
}
