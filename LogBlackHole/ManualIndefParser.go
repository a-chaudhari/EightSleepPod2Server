package LogBlackHole

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// ChunkReader reads indefinite-length byte string chunks as they arrive
type ChunkReader struct {
	reader       io.Reader
	inIndefinite bool
	chunks       [][]byte
}

func NewChunkReader(r io.Reader) *ChunkReader {
	return &ChunkReader{reader: r}
}

// ReadChunk reads the next chunk from an indefinite byte string
// Returns io.EOF when the break code (0xff) is encountered
func (cr *ChunkReader) ReadChunk() ([]byte, error) {
	// Read the initial byte
	header := make([]byte, 1)
	if _, err := io.ReadFull(cr.reader, header); err != nil {
		return nil, err
	}

	majorType := header[0] >> 5
	additionalInfo := header[0] & 0x1f

	// Check for indefinite-length byte string start (0x5f)
	if header[0] == 0x5f {
		cr.inIndefinite = true
		// Recursively read the first actual chunk
		return cr.ReadChunk()
	}

	// Check for break code (0xff) - end of indefinite length
	if header[0] == 0xff {
		cr.inIndefinite = false
		return nil, io.EOF
	}

	// Must be a byte string (major type 2)
	if majorType != 2 {
		return nil, fmt.Errorf("expected byte string, got major type %d", majorType)
	}

	// Decode the length
	length, err := cr.decodeLength(additionalInfo)
	if err != nil {
		return nil, err
	}

	// Read the chunk data
	chunk := make([]byte, length)
	if _, err := io.ReadFull(cr.reader, chunk); err != nil {
		return nil, err
	}

	cr.chunks = append(cr.chunks, chunk)
	return chunk, nil
}

func (cr *ChunkReader) decodeLength(additionalInfo byte) (uint64, error) {
	if additionalInfo < 24 {
		return uint64(additionalInfo), nil
	}

	var lengthBytes int
	switch additionalInfo {
	case 24:
		lengthBytes = 1
	case 25:
		lengthBytes = 2
	case 26:
		lengthBytes = 4
	case 27:
		lengthBytes = 8
	default:
		return 0, errors.New("invalid additional info for length")
	}

	buf := make([]byte, lengthBytes)
	if _, err := io.ReadFull(cr.reader, buf); err != nil {
		return 0, err
	}

	var length uint64
	for _, b := range buf {
		length = (length << 8) | uint64(b)
	}
	return length, nil
}

// GetCombinedData returns all chunks combined into a single byte slice
func (cr *ChunkReader) GetCombinedData() []byte {
	var total int
	for _, chunk := range cr.chunks {
		total += len(chunk)
	}

	result := make([]byte, 0, total)
	for _, chunk := range cr.chunks {
		result = append(result, chunk...)
	}
	return result
}

// ReadAllChunks reads all chunks until break code and returns combined data
func (cr *ChunkReader) ReadAllChunks() ([]byte, error) {
	for {
		chunk, err := cr.ReadChunk()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		fmt.Printf("Received chunk: %x (%d bytes)\n", chunk, len(chunk))
	}
	return cr.GetCombinedData(), nil
}

func main() {
	// Example: indefinite byte string with 3 chunks
	// 5f = start indefinite byte string
	// 44 01020304 = chunk 1 (4 bytes)
	// 43 050607 = chunk 2 (3 bytes)
	// 42 0809 = chunk 3 (2 bytes)
	// ff = break (end)
	data, _ := hex.DecodeString("5f440102030443050607420809ff")

	reader := NewChunkReader(bytes.NewReader(data))
	combined, err := reader.ReadAllChunks()
	if err != nil {
		panic(err)
	}

	fmt.Printf("Combined data: %x\n", combined)
	// Output: Combined data: 010203040506070809
}
