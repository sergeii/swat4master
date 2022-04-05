package binutils

// ConsumeString scans a slice of bytes for delimiter-terminated strings.
// Returns a subslice representing the first found string,
// and another subslice with the remaining bytes in it, excluding the delimiter
func ConsumeString(data []byte, delim byte) ([]byte, []byte) {
	for i := range data {
		if data[i] == delim {
			return data[:i], data[i+1:]
		}
	}
	return data, nil
}

// ConsumeCString scans a slice of bytes for null-terminated strings
// Otherwise, the behavior is the same as ConsumeString
func ConsumeCString(data []byte) ([]byte, []byte) {
	return ConsumeString(data, 0x00)
}
