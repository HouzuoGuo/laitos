package toolbox

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

var (
	// RegexTextSearch finds a shortcut name and a search string.
	RegexTextSearch       = regexp.MustCompile(`(\w+)[^\w]+(.*)`)
	ErrBadTextSearchParam = errors.New(`example: shortcut text_to_search`)
)

const TextSearchTrigger = ".g" // TextSearchTrigger the trigger prefix string of TextSearch feature.

// TextSearch locates a string among lines in a text file.
type TextSearch struct {
	FilePaths map[string]string `json:"FilePaths"` // FilePaths contains shortcut name VS text file path
}

func (txt *TextSearch) IsConfigured() bool {
	return txt.FilePaths != nil && len(txt.FilePaths) > 0
}

func (txt *TextSearch) SelfTest() error {
	if !txt.IsConfigured() {
		return ErrIncompleteConfig
	}
	for _, filePath := range txt.FilePaths {
		if _, err := os.Stat(filePath); err != nil {
			return fmt.Errorf("TextSearch.SelfTest: file \"%s\" is no longer readable - %v", filePath, err)
		}
	}
	return nil
}

func (txt *TextSearch) Initialise() error {
	// Run a round of self test to discover configuration error during initialisation
	return txt.SelfTest()
}

func (txt *TextSearch) Trigger() Trigger {
	return TextSearchTrigger
}

func (txt *TextSearch) Execute(cmd Command) (ret *Result) {
	if errResult := cmd.Trim(); errResult != nil {
		return errResult
	}

	// Break down parameter shortcut name and text string to search
	params := RegexTextSearch.FindStringSubmatch(cmd.Content)
	if len(params) != 3 {
		return &Result{Error: ErrBadTextSearchParam}
	}
	shortcutName := params[1]
	searchString := strings.ToLower(params[2])
	filePath, found := txt.FilePaths[shortcutName]
	if !found {
		return &Result{Error: errors.New("cannot find " + shortcutName)}
	}

	// Open the text file to read its fresh data
	file, err := os.Open(filePath)
	if err != nil {
		return &Result{Error: fmt.Errorf("failed to read text file %s - %v", filePath, err)}
	}
	defer file.Close()

	// Collect matching lines
	var numMatch int
	var match bytes.Buffer
	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if strings.Contains(strings.ToLower(line), searchString) {
			match.WriteString(line)
			numMatch++
		}
		if err == io.EOF {
			break
		} else if err != nil {
			return &Result{Error: fmt.Errorf("failed to read text file %s - %v", filePath, err)}
		}
	}
	// Output is number of matched lines followed by their content
	return &Result{Output: fmt.Sprintf("%d %s", numMatch, match.String())}
}
