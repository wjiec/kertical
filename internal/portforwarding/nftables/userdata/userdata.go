package userdata

import "encoding/binary"

// Marshal prepares a comment string for inclusion in a nftables rule.
func Marshal(comment string) []byte {
	if len(comment) == 0 {
		return nil
	}

	buf := make([]byte, len(comment)+3) // the length of the comment + '\x00'
	binary.BigEndian.PutUint16(buf, uint16(len(comment)+1))
	copy(buf[2:], comment)
	return buf
}

// Unmarshal extracts a comment string from nftables rule user data.
//
// Returns an empty string if the data is invalid or too short.
func Unmarshal(data []byte) string {
	if len(data) < 2 {
		return ""
	}

	sz := binary.BigEndian.Uint16(data[:2])
	if len(data) < int(sz)+2 { // header + (comment + '\x00')
		return ""
	}
	return string(data[2 : 1+sz])
}
