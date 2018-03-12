package misc

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
)

// EditKeyValue modifies or inserts a key=value pair into the specified file.
func EditKeyValue(filePath, key, value string) error {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}
	originalLines := strings.Split(string(content), "\n")
	newLines := make([]string, 0, len(originalLines)+1)
	var foundKey bool
	// Look for all instances of the key appearing as line prefix
	for _, line := range originalLines {
		if trimmedLine := strings.TrimSpace(line); strings.HasPrefix(trimmedLine, key+"=") || strings.HasPrefix(trimmedLine, key+" ") {
			// Successfully matched "key = value" or "key=value"
			foundKey = true
			newLines = append(newLines, fmt.Sprintf("%s=%s", key, value))
		} else {
			// Preserve prefix and suffix spaces
			newLines = append(newLines, line)
		}
	}
	if !foundKey {
		newLines = append(newLines, fmt.Sprintf("%s=%s", key, value))
	}
	return ioutil.WriteFile(filePath, []byte(strings.Join(newLines, "\n")), 0600)
}

var (
	ErrInputReaderNil       = errors.New("input reader is nil")
	ErrInputCapacityInvalid = errors.New("input capacity is invalid")
)

// ReadAllUpTo reads data from input reader until the limited capacity is reached or reader is exhausted (EOF).
func ReadAllUpTo(r io.Reader, upTo int) (ret []byte, err error) {
	ret = []byte{}
	if r == nil {
		err = ErrInputReaderNil
		return
	}
	if upTo < 0 {
		err = ErrInputCapacityInvalid
		return
	}

	return ioutil.ReadAll(io.LimitReader(r, int64(upTo)))
}
