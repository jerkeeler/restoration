package parser

import (
	"encoding/binary"
	"math"
	"unicode/utf16"
)

// byteWriter builds little-endian replay bytes for tests.
type byteWriter struct {
	buf []byte
}

func (w *byteWriter) u8(v uint8) *byteWriter {
	w.buf = append(w.buf, v)
	return w
}

func (w *byteWriter) u16(v uint16) *byteWriter {
	w.buf = binary.LittleEndian.AppendUint16(w.buf, v)
	return w
}

func (w *byteWriter) u32(v uint32) *byteWriter {
	w.buf = binary.LittleEndian.AppendUint32(w.buf, v)
	return w
}

func (w *byteWriter) i32(v int32) *byteWriter {
	return w.u32(uint32(v))
}

func (w *byteWriter) f32(v float32) *byteWriter {
	return w.u32(math.Float32bits(v))
}

func (w *byteWriter) vec(x, y, z float32) *byteWriter {
	return w.f32(x).f32(y).f32(z)
}

// str writes a replay string: uint16 char count, 2 padding bytes, UTF-16 LE chars.
func (w *byteWriter) str(s string) *byteWriter {
	chars := utf16.Encode([]rune(s))
	w.u16(uint16(len(chars))).u16(0)
	for _, c := range chars {
		w.u16(c)
	}
	return w
}

func (w *byteWriter) raw(b ...byte) *byteWriter {
	w.buf = append(w.buf, b...)
	return w
}

// node writes a two-letter token, uint32 body size, then the body.
func (w *byteWriter) node(token string, body []byte) *byteWriter {
	w.raw([]byte(token)...).u32(uint32(len(body)))
	return w.raw(body...)
}

// rootWithChild builds a BG root holding J1/<token> backed by a node at offset 0 of data.
func rootWithChild(data []byte, token string) Node {
	child := &Node{token: token, offset: 0, size: uint32(len(data) - DATA_OFFSET)}
	j1 := &Node{token: "J1", children: []*Node{child}}
	return Node{token: "BG", children: []*Node{j1}}
}
