package serialport

import (
	"errors"
	"io"
)

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
