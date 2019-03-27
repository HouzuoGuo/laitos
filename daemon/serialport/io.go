package serialport

import (
	"errors"
	"io"
	"os"
	"time"
)

/*
WriteSlowlyIntervalMS is the interval at which writeSlowly function writes a byte to its destination.
8 MS interval corresponds to ~125 bytes/second, which is about 1000 bauds/second.
*/
const WriteSlowlyIntervalMS = 8

/*
readUntilDelimiter reads an input byte sequence (without suffix delimiter) until any of the acceptable delimiter is seen.
Empty byte sequences (i.e. zero length input solely consisting of a delimiter) are discarded.
*/
func readUntilDelimiter(src io.Reader, delimiters ...byte) ([]byte, error) {
	ret := make([]byte, 0, 64)
readNextByte:
	for {
		b := []byte{0}
		n, err := src.Read(b)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			// should not happen
			return nil, errors.New("readUntilDelimiterOrTimeout: read nothing yet no error")
		}
		// Return non-empty byte sequences
		for _, delimiter := range delimiters {
			if b[0] == delimiter {
				if len(ret) > 0 {
					return ret, nil
				} else {
					continue readNextByte
				}
			}
		}
		ret = append(ret, b[0])
	}
}

/*
writeSlowly writes the data buffer into the file at approximately 125 bytes/second (less than 1200 bauds/second).
Serial communication uses "baud rate" (1 baud/second = 1 bit/second) to determine speed and throughput of transmission, nowadays serial
ports can operate at up to 115200 BPS and even the slowest among modems (e.g. satellite connection) can handle 2400 BPS.
Hence, 125 bytes/second should be compatible with nearly all serial devices.
*/
func writeSlowly(dst *os.File, data []byte) (err error) {
	for _, b := range data {
		if _, err := dst.Write([]byte{b}); err != nil {
			return err
		} else if err := dst.Sync(); err != nil {
			return err
		}
		// 1 second = 1000 milliseconds, 1000 MS / (8 MS/byte) = 125 bytes
		time.Sleep(WriteSlowlyIntervalMS * time.Millisecond)
	}
	return nil
}
