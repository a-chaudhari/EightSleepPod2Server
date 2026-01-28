package LogServer

// StreamParser wraps CircularBuffer and provides CBOR extraction utilities.
type StreamParser struct {
	cb CircularBuffer
}

// Insert adds data to the buffer. If data exceeds available space, it returns an error.
func (sp *StreamParser) Insert(data []byte) {
	_ = sp.cb.Write(data) // Overwrite error is ignored for compatibility
}

type CBORExtractResult struct {
	Data       [][]byte // extracted data portions
	ResetFound bool     // true if a reset code (0xFF) was encountered
}

// ExtractCBORByteStrings extracts as many CBOR byte strings as possible from the buffer.
// Stops if a reset code (0xFF) is encountered or incomplete data is found.
// Returns a CBORExtractResult with the extracted data and reset status.
func (sp *StreamParser) ExtractCBORByteStrings() CBORExtractResult {
	var results [][]byte
	resetFound := false
	for {
		if sp.cb.DataLen() == 0 {
			break
		}
		peek := sp.cb.Peek(1)
		b := peek[0]
		if b == 0xFF {
			// Reset code encountered, clear buffer
			sp.cb = CircularBuffer{}
			resetFound = true
			break
		}
		length, headerLen, ok := parseCBORByteStringHeaderCircular(&sp.cb)
		if !ok {
			break // incomplete header
		}
		totalLen := headerLen + length
		if sp.cb.DataLen() < totalLen {
			break // incomplete data
		}
		sp.cb.Advance(headerLen) // discard header
		bs := sp.cb.Peek(length)
		results = append(results, bs)
		sp.cb.Advance(length)
	}
	return CBORExtractResult{Data: results, ResetFound: resetFound}
}

// parseCBORByteStringHeaderCircular parses the CBOR byte string header at the buffer head.
// Returns (length, headerLen, ok). If not enough data, ok=false.
func parseCBORByteStringHeaderCircular(cb *CircularBuffer) (int, int, bool) {
	if cb.DataLen() < 1 {
		return 0, 0, false
	}
	first := cb.Peek(1)[0]
	if (first & 0xE0) != 0x40 {
		return 0, 0, false // not a byte string
	}
	ai := first & 0x1F
	switch {
	case ai < 24:
		return int(ai), 1, true
	case ai == 24:
		if cb.DataLen() < 2 {
			return 0, 0, false
		}
		return int(cb.Peek(2)[1]), 2, true
	case ai == 25:
		if cb.DataLen() < 3 {
			return 0, 0, false
		}
		peek := cb.Peek(3)
		return int(peek[1])<<8 | int(peek[2]), 3, true
	case ai == 26:
		if cb.DataLen() < 5 {
			return 0, 0, false
		}
		peek := cb.Peek(5)
		return int(peek[1])<<24 | int(peek[2])<<16 | int(peek[3])<<8 | int(peek[4]), 5, true
	default:
		return 0, 0, false // indefinite or unsupported
	}
}
