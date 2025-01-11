package parser

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"unicode/utf16"
)

type String struct {
	value     string
	endOffset int
}

func readUint16(data *[]byte, offset int) uint16 {
	return binary.LittleEndian.Uint16((*data)[offset : offset+2])
}

func readInt16(data *[]byte, offset int) int16 {
	return int16(readUint16(data, offset))
}

func readUint32(data *[]byte, offset int) uint32 {
	return binary.LittleEndian.Uint32((*data)[offset : offset+4])
}

func readInt32(data *[]byte, offset int) int32 {
	return int32(readUint32(data, offset))
}

func readBool(data *[]byte, offset int) bool {
	return (*data)[offset] != 0
}

func readString(data *[]byte, offset int) String {
	/*
	   Reads the utf-16 little endian encoded at the given offset. Strings are enocde such that the first 2 bytes
	   are an unsigned integer indicating the number of characters in the string. The next 2 bytes are null padding.
	   Then the string follows. Since the strings are unicode enocode each character takes up 2 bytes.
	   For example a string might look like:
	   \x02\x00\x00\x00H\x00e\x00l\x00l\x00o\x00

	   Returns the string and the index directly after the string.
	*/
	numChars := readUint16(data, offset)
	startOfString := offset + 4
	endOfString := startOfString + int(numChars)*2

	// Converts the bytes into uint16, which are used for utf-16 encoding
	u16s := make([]uint16, numChars)
	for i := uint16(0); i < numChars; i++ {
		u16s[i] = readUint16(data, startOfString+int(i)*2)
	}

	return String{
		// Converts the UTF-16 encoded string into the native utf-8 encoded string by Go, might need to change this
		string(utf16.Decode(u16s)),
		endOfString,
	}
}

type NotL33t string

func (err NotL33t) Error() string {
	return string(err)
}

func Decompressl33t(compressed_array *[]byte) ([]byte, error) {
	header := string((*compressed_array)[:4])
	if header != "l33t" {
		return nil, NotL33t(fmt.Sprintf("Data is no l33t compressed, incorrect header: \"%s\"", header))
	}

	reader, err := zlib.NewReader(bytes.NewReader((*compressed_array)[8:]))
	defer reader.Close()
	if err != nil {
		return nil, err
	}

	return io.ReadAll(reader)
}
