package LogServer

// CircularBuffer is a fixed-size circular buffer for bytes.
type CircularBuffer struct {
	buf        [8192]byte
	head, tail int // head: read, tail: write
	full       bool
}

// Insert adds data to the buffer. If data exceeds available space, it overwrites old data.
func (cb *CircularBuffer) Insert(data []byte) {
	for _, b := range data {
		cb.buf[cb.tail] = b
		if cb.full {
			cb.head = (cb.head + 1) % len(cb.buf)
		}
		cb.tail = (cb.tail + 1) % len(cb.buf)
		cb.full = cb.head == cb.tail
	}
}

// CBORExtractResult holds the result of extracting CBOR byte strings from the buffer.
type CBORExtractResult struct {
	Data       [][]byte // extracted data portions
	ResetFound bool     // true if a reset code (0xFF) was encountered
}

// ExtractCBORByteStrings extracts as many CBOR byte strings as possible from the buffer.
// Stops if a reset code (0xFF) is encountered or incomplete data is found.
// Returns a CBORExtractResult with the extracted data and reset status.
func (cb *CircularBuffer) ExtractCBORByteStrings() CBORExtractResult {
	var results [][]byte
	resetFound := false
	for {
		if cb.isEmpty() {
			break
		}
		b := cb.peek(0)
		if b == 0xFF {
			// Reset code encountered, clear buffer
			cb.reset()
			resetFound = true
			break
		}
		length, headerLen, ok := parseCBORByteStringHeader(cb)
		if !ok {
			break // incomplete header
		}
		totalLen := headerLen + length
		if cb.size() < totalLen {
			break // incomplete data
		}
		// Extract only the data portion (skip header)
		cb.readN(headerLen) // discard header
		bs := cb.readN(length)
		results = append(results, bs)
	}
	return CBORExtractResult{Data: results, ResetFound: resetFound}
}

// Helper: Check if buffer is empty
func (cb *CircularBuffer) isEmpty() bool {
	return !cb.full && cb.head == cb.tail
}

// Helper: Buffer size
func (cb *CircularBuffer) size() int {
	if cb.full {
		return len(cb.buf)
	}
	if cb.tail >= cb.head {
		return cb.tail - cb.head
	}
	return len(cb.buf) - cb.head + cb.tail
}

// Helper: Peek at offset from head
func (cb *CircularBuffer) peek(offset int) byte {
	return cb.buf[(cb.head+offset)%len(cb.buf)]
}

// Helper: Read N bytes from head
func (cb *CircularBuffer) readN(n int) []byte {
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		out[i] = cb.buf[cb.head]
		cb.head = (cb.head + 1) % len(cb.buf)
		cb.full = false
	}
	return out
}

// Helper: Reset buffer
func (cb *CircularBuffer) reset() {
	cb.head = 0
	cb.tail = 0
	cb.full = false
}

// parseCBORByteStringHeader parses the CBOR byte string header at the buffer head.
// Returns (length, headerLen, ok). If not enough data, ok=false.
func parseCBORByteStringHeader(cb *CircularBuffer) (int, int, bool) {
	if cb.size() < 1 {
		return 0, 0, false
	}
	first := cb.peek(0)
	if (first & 0xE0) != 0x40 {
		return 0, 0, false // not a byte string
	}
	ai := first & 0x1F
	switch {
	case ai < 24:
		return int(ai), 1, true
	case ai == 24:
		if cb.size() < 2 {
			return 0, 0, false
		}
		return int(cb.peek(1)), 2, true
	case ai == 25:
		if cb.size() < 3 {
			return 0, 0, false
		}
		return int(cb.peek(1))<<8 | int(cb.peek(2)), 3, true
	case ai == 26:
		if cb.size() < 5 {
			return 0, 0, false
		}
		return int(cb.peek(1))<<24 | int(cb.peek(2))<<16 | int(cb.peek(3))<<8 | int(cb.peek(4)), 5, true
	default:
		return 0, 0, false // indefinite or unsupported
	}
}
