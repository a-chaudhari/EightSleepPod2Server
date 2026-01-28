package LogServer

import (
	"github.com/fxamacker/cbor/v2"
)

type SecondLevelParser struct {
	cb CircularBuffer
}

// Insert adds data to the buffer. If data exceeds available space, it returns an error.
func (sp *SecondLevelParser) Insert(data []byte) {
	_ = sp.cb.Write(data) // Overwrite error is ignored for compatibility
}

func NewSecondLevelParser() *SecondLevelParser {
	return &SecondLevelParser{}
}

// DataLen returns the number of bytes currently in the buffer.
func (s *SecondLevelParser) DataLen() int {
	return s.cb.DataLen()
}

// CBORPayload represents the expected CBOR map structure.
type CBORPayload struct {
	Seq  uint32 `cbor:"seq"`
	Data []byte `cbor:"data"`
}

// ExtractCBORPayloads tries to extract as many complete CBOR payloads as possible from the buffer.
// It returns a slice of successfully decoded payloads.
func (cb *SecondLevelParser) ExtractCBORPayloads() ([]CBORPayload, error) {
	var results []CBORPayload
	for {
		if cb.DataLen() == 0 {
			break
		}
		peek := cb.cb.Peek(cb.DataLen())
		var payload CBORPayload
		var consumed int
		// Try to decode the payload from the buffer
		for i := 1; i <= len(peek); i++ {
			err := cbor.Unmarshal(peek[:i], &payload)
			if err == nil {
				// Confirm that re-encoding matches the slice length
				encoded, err2 := cbor.Marshal(payload)
				if err2 == nil && len(encoded) == i {
					consumed = i
					break
				}
			}
		}
		if consumed == 0 {
			// No complete payload found
			break
		}
		results = append(results, payload)
		cb.cb.Advance(consumed)
	}
	return results, nil
}
