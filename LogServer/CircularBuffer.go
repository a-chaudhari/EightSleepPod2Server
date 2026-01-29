package LogServer

import "errors"

const bufferSize = 8 * 1024 // 8KB

type CircularBuffer struct {
	buf        [bufferSize]byte
	readPos    int
	writePos   int
	dataLength int
}

// Write appends data to the buffer. Returns error if not enough space.
func (cb *CircularBuffer) Write(data []byte) error {
	if len(data) > bufferSize-cb.dataLength {
		return errors.New("not enough space in buffer")
	}
	for _, b := range data {
		cb.buf[cb.writePos] = b
		cb.writePos = (cb.writePos + 1) % bufferSize
	}
	cb.dataLength += len(data)
	return nil
}

// Peek returns up to n bytes from the buffer without advancing readPos.
func (cb *CircularBuffer) Peek(n int) []byte {
	if n > cb.dataLength {
		n = cb.dataLength
	}
	result := make([]byte, n)
	for i := 0; i < n; i++ {
		result[i] = cb.buf[(cb.readPos+i)%bufferSize]
	}
	return result
}

// Advance moves the readPos forward by n bytes.
func (cb *CircularBuffer) Advance(n int) {
	if n > cb.dataLength {
		n = cb.dataLength
	}
	cb.readPos = (cb.readPos + n) % bufferSize
	cb.dataLength -= n
}

// DataLen returns the number of bytes currently in the buffer.
func (cb *CircularBuffer) DataLen() int {
	return cb.dataLength
}
