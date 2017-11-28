package misc

import (
	"io"
	"sync"
)

/*
ByteLogWriter forwards verbatim bytes to destination writer, and keeps designated number of latest output bytes in
internal buffers for later retrieval. It implements io.Writer interface.
*/
type ByteLogWriter struct {
	destination io.Writer  // destination is the writer to forward verbatim output to.
	mem         []byte     // mem has the size of KeepBytes and memorises latest output bytes.
	memPos      int        // memPos is the location in buffer to write the next output at.
	everFull    bool       // everFull is true only if the internal memory has ever been filled up.
	mutex       sync.Mutex // mutex prevents simultaneous Write operations from taking place.
	KeepBytes   int        // KeepBytes is the number of bytes to keep
}

// NewByteLogWriter initialises a new ByteLogBuffer and returns it.
func NewByteLogWriter(destination io.Writer, keepBytes int) *ByteLogWriter {
	return &ByteLogWriter{
		destination: destination,
		mem:         make([]byte, keepBytes),
		memPos:      0,
		KeepBytes:   keepBytes,
	}
}

// absorb memorises the bytes written by the latest operation (protected by mutex) in internal buffers.
func (writer *ByteLogWriter) absorb(p []byte) {
	for {
		room := len(writer.mem) - writer.memPos
		if room >= len(p) {
			// There is enough room for the latest write buffer
			copy(writer.mem[writer.memPos:], p)
			writer.memPos += len(p)
			if room == len(p) {
				// Reset position so that next write operation will restart from beginning of the memory
				writer.memPos = 0
				writer.everFull = true
			}
			return
		} else {
			// There is not enough room for the latest write buffer
			copy(writer.mem[writer.memPos:], p[:room])
			p = p[room:]
			writer.memPos = 0
			writer.everFull = true
			continue
		}
	}
}

// Retrieve returns a copy of the latest bytes written.
func (writer *ByteLogWriter) Retrieve() (ret []byte) {
	if writer.everFull {
		ret = make([]byte, writer.KeepBytes)
		copy(ret, writer.mem[writer.memPos:])
		copy(ret[len(writer.mem)-writer.memPos:], writer.mem[:writer.memPos])
	} else {
		ret = make([]byte, writer.memPos)
		copy(ret, writer.mem[:writer.memPos])
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
