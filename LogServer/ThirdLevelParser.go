package LogServer

import (
	"github.com/fxamacker/cbor/v2"
)

// LogEntry represents a single log entry with 4 fields.
type LogEntry struct {
	Ts    uint32 `cbor:"ts"`
	Msg   string `cbor:"msg"`
	Level string `cbor:"level"`
	Type  string `cbor:"type"`
}

// ThirdLevelParser parses CBOR log entries from a CircularBuffer.
type ThirdLevelParser struct {
	cb CircularBuffer
}

// Insert adds data to the buffer. If data exceeds available space, it returns an error.
func (tp *ThirdLevelParser) Insert(data []byte) {
	_ = tp.cb.Write(data)
}

// ExtractLogEntries tries to extract as many complete log entries as possible from the buffer.
// It returns a slice of successfully decoded LogEntry structs.
func (tp *ThirdLevelParser) ExtractLogEntries() ([]LogEntry, error) {
	var results []LogEntry
	for {
		if tp.cb.DataLen() == 0 {
			break
		}
		peek := tp.cb.Peek(tp.cb.DataLen())
		var entry LogEntry
		var consumed int
		for i := 1; i <= len(peek); i++ {
			err := cbor.Unmarshal(peek[:i], &entry)
			if err == nil {
				encoded, err2 := cbor.Marshal(entry)
				if err2 == nil && len(encoded) == i {
					consumed = i
					break
				}
			}
		}
		if consumed == 0 {
			break
		}
		results = append(results, entry)
		tp.cb.Advance(consumed)
	}
	return results, nil
}
