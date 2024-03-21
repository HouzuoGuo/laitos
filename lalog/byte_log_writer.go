package lalog

import (
	"bytes"
	"io"
	"sync"
	"unicode"
)

/*
ByteLogWriter forwards verbatim bytes to destination writer, and keeps designated number of latest output bytes in
internal buffers for later retrieval. It implements io.Writer interface.
*/
type ByteLogWriter struct {
	io.WriteCloser
	MaxBytes    int        // MaxBytes is the number of latest data bytes to keep.
	destination io.Writer  // destination is the writer to forward verbatim output to.
	mutex       sync.Mutex // mutex prevents simultaneous Write operations from taking place.
	latestBytes []byte     // latestBytes is an internal buffer memorising the latest input bytes.
	latestPos   int        // latestPos is the location of internal buffer to write next at.
	everFull    bool       // everFull is true only if the internal buffer has ever been filled completely.
	currentSize int        // currentSize is the amount of meaningful data currently residing in the internal buffer.
}

// NewByteLogWriter initialises a new ByteLogBuffer and returns it.
func NewByteLogWriter(destination io.Writer, maxBytes int) *ByteLogWriter {
	return &ByteLogWriter{
		destination: destination,
		latestBytes: make([]byte, 0),
		latestPos:   0,
		currentSize: 0,
		MaxBytes:    maxBytes,
	}
}

// ensureSize enlarges internal buffer to at least the specified size.
func (writer *ByteLogWriter) ensureSize(expectedSize int) {
	if writer.currentSize < expectedSize {
		writer.currentSize = expectedSize
	}
	if writer.currentSize > writer.MaxBytes {
		writer.currentSize = writer.MaxBytes
	}
	// Double the buffer size until it is larger than the expected size
	for {
		if len(writer.latestBytes) < expectedSize {
			newSize := len(writer.latestBytes) * 2
			if newSize == 0 {
				newSize = 2
			}
			if newSize > writer.MaxBytes {
				newSize = writer.MaxBytes
			}
			if newSize == len(writer.latestBytes) {
				return
			}
			newBuf := make([]byte, newSize)
			copy(newBuf, writer.latestBytes)
			writer.latestBytes = newBuf
			if newSize >= expectedSize {
				return
			}
			// Keep enlarging the buffer until it is large enough
			continue
		}
		return
	}
}

// absorb memorises the bytes written by the latest operation (protected by mutex) in internal buffers.
func (writer *ByteLogWriter) absorb(in []byte) {
	writer.ensureSize(len(in) + writer.latestPos)
	if len(in) >= writer.MaxBytes {
		// If input is larger then copy the last several bytes into internal buffer
		copy(writer.latestBytes, in[len(in)-writer.MaxBytes:])
		writer.everFull = true
		writer.latestPos = 0
	} else {
		room := writer.currentSize - writer.latestPos
		if room >= len(in) {
			copy(writer.latestBytes[writer.latestPos:], in)
			writer.latestPos += len(in)
			if writer.latestPos == writer.MaxBytes {
				writer.everFull = true
				writer.latestPos = 0
			}
		} else {
			copy(writer.latestBytes[writer.latestPos:], in)
			copy(writer.latestBytes[:room], in[room:])
			writer.latestPos = room
			writer.everFull = true
		}
	}
}

// Retrieve returns a copy of the latest bytes written.
func (writer *ByteLogWriter) Retrieve(asciiOnly bool) (ret []byte) {
	writer.mutex.Lock()
	defer writer.mutex.Unlock()
	var bufCopy []byte
	if writer.everFull {
		bufCopy = make([]byte, writer.currentSize)
		copy(bufCopy, writer.latestBytes[writer.latestPos:writer.currentSize])
		copy(bufCopy[len(writer.latestBytes)-writer.latestPos:writer.currentSize], writer.latestBytes[:writer.latestPos])
	} else {
		bufCopy = make([]byte, writer.latestPos)
		copy(bufCopy, writer.latestBytes[:writer.latestPos])
	}

	ret = bufCopy
	if asciiOnly {
		var out bytes.Buffer
		for _, r := range bufCopy {
			if r < 128 && (unicode.IsPrint(rune(r)) || unicode.IsSpace(rune(r))) {
				out.WriteByte(r)
			} else {
				out.WriteRune('?')
			}
		}
		ret = out.Bytes()
	}

	return
}

// Write implements io.Writer to forward the data to destination writer.
func (writer *ByteLogWriter) Write(p []byte) (n int, err error) {
	writer.mutex.Lock()
	n, err = writer.destination.Write(p)
	writer.absorb(p)
	writer.mutex.Unlock()
	return
}

// Close does nothing and always returns nil.
func (writer *ByteLogWriter) Close() error {
	return nil
}

// DiscardCloser implements io.WriteCloser.
var DiscardCloser = discardCloser{}

type discardCloser struct {
	io.WriteCloser
}

func (discardCloser) Write(p []byte) (int, error) {
	return len(p), nil
}

func (discardCloser) Close() error {
	return nil
}
