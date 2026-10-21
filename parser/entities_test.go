package parser

import (
	"reflect"
	"testing"
)

// entityRecord writes a 1-byte prefix and an H1 node holding EN (id, owner, position) and a proto node.
func entityRecord(id int32, owner uint32, x, z float32, protoToken string, protoId uint32) []byte {
	en := (&byteWriter{}).i32(id).u32(0).u32(owner).raw(make([]byte, 20)...).vec(x, 4, z)
	en.raw(make([]byte, 98-len(en.buf))...)
	h1 := (&byteWriter{}).node("EN", en.buf).node(protoToken, (&byteWriter{}).u32(protoId).buf)
	return (&byteWriter{}).u8(1).node("H1", h1.buf).buf
}

func z1Node(records ...[]byte) []byte {
	body := (&byteWriter{}).u32(uint32(len(records))).u32(1)
	for _, record := range records {
		body.raw(record...)
	}
	return (&byteWriter{}).node("Z1", body.buf).buf
}

func protoCatalog(names ...string) *XmbNode {
	children := make([]*XmbNode, len(names))
	for i, name := range names {
		children[i] = &XmbNode{attributes: map[string]string{"name": name}}
	}
	return &XmbNode{children: children}
}

func TestParseEntities(t *testing.T) {
	data := z1Node(
		entityRecord(0, 1, 81, 217, "P1", 1),
		make([]byte, 40), // zero-filled deleted slots
		entityRecord(5, 0, 120.5, 33.25, "P1", 0),
		entityRecord(6, 0, 1, 1, "Q1", 0), // malformed: no P1 node
		entityRecord(7, 2, 10, 20, "P1", 99),
	)
	got, err := parseEntities(&data, rootWithChild(data, "Z1"), protoCatalog("TreePalm", "TownCenter"))
	if err != nil {
		t.Fatalf("parseEntities() error = %v", err)
	}

	want := []ReplayEntity{
		{Id: 0, PlayerNum: 1, Name: "TownCenter", X: 81, Y: 4, Z: 217},
		{Id: 5, PlayerNum: 0, Name: "TreePalm", X: 120.5, Y: 4, Z: 33.25},
		{Id: 7, PlayerNum: 2, Name: "unknown", X: 10, Y: 4, Z: 20},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseEntities() = %+v, want %+v", got, want)
	}
}

func TestParseEntitiesMissingNode(t *testing.T) {
	data := z1Node()
	if _, err := parseEntities(&data, rootWithChild(data, "TN"), protoCatalog()); err == nil {
		t.Error("parseEntities() error = nil, want missing J1/Z1")
	}
}
