package lalog

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/HouzuoGuo/laitos/datastruct"
)

const (
	/*
		NumLatestLogEntries is the number of latest log entries to memorise. They are presented in information HTTP
		endpoint as well as system maintenance report for a glance.
	*/
	NumLatestLogEntries = 128
	// MaxLogMessageLen is the maximum length memorised for each of the latest log entries.
	MaxLogMessageLen = 2048
	truncatedLabel   = "...(truncated)..."
)

type LogWarningCallbackFunc func(componentName, componentID, funcName, actorName string, err error, msg string)

var (
	// LatestWarnings are a small number of the most recent log messages (warnings and info messages ) kept in memory for retrieval and inspection.
	LatestLogs = datastruct.NewRingBuffer(NumLatestLogEntries)

	// LatestWarnings are a small number of the most recent warning log messages kept in memory for retrieval and inspection.
	LatestWarnings = datastruct.NewRingBuffer(NumLatestLogEntries)

	// LatestWarningActors are a small number of actors that have recently generated warning messages.
	// The LRU buffer helps to gain a more comprehensive picture of the actors that have resulted in warning log messages by
	// working as a conditional filter, so that only the first instance of warning from an actor (identified by component name + function name +
	// actor name) will be added to the in-memory warning log buffer. The subsequent warning messages of that actor will be excluded from the
	// warning buffer. The actor will get another chance to show up in the warning buffer when it eventually becomes stale and is subsequently
	// evicted by this LRU buffer.
	LatestWarningActors = datastruct.NewLeastRecentlyUsedBuffer(NumLatestLogEntries / 4)

	// LogWarningCallback is invoked in a separate goroutine after any logger has processed a warning message.
	// The function must avoid generating a warning log message of itself, to avoid an infinite recursion.
	GlobalLogWarningCallback LogWarningCallbackFunc = nil
)

/*
LoggerIDField is a field of Logger's ComponentID, all fields that make up a ComponentID offer log entry a clue as to
which component instance generated the log message.
*/
type LoggerIDField struct {
	Key   string      // Key is an arbitrary string key
	Value interface{} // Value is an arbitrary value that will be converted to string upon printing a log entry.
}

// Help to write log messages in a regular format.
type Logger struct {
	ComponentName string          // ComponentName is similar to a class name, or a category name.
	ComponentID   []LoggerIDField // ComponentID comprises key-value pairs that give log entry a clue as to its origin.
}

// getComponentIDs returns a string consisting of the logger's component ID fields. If there are none, it returns an empty string.
func (logger *Logger) getComponentIDs() string {
	var msg bytes.Buffer
	if logger.ComponentID != nil && len(logger.ComponentID) > 0 {
		msg.WriteRune('[')
		for i, field := range logger.ComponentID {
			msg.WriteString(fmt.Sprintf("%s=%v", field.Key, field.Value))
			if i < len(logger.ComponentID)-1 {
				msg.WriteRune(';')
			}
		}
		msg.WriteRune(']')
	}
	return msg.String()
}

// Format a log message and return, but do not print it.
func (logger *Logger) Format(functionName, actorName string, err error, template string, values ...interface{}) string {
	// Message is going to look like this:
	// ComponentName[IDKey1-IDVal1;IDKey2-IDVal2].FunctionName(actorName): Error "no such file" - failed to start component
	var msg bytes.Buffer
	if logger.ComponentName != "" {
		msg.WriteString(logger.ComponentName)
	}
	msg.WriteString(logger.getComponentIDs())
	if functionName != "" {
		if msg.Len() > 0 {
			msg.WriteRune('.')
		}
		msg.WriteString(functionName)
	}
	if actorName != "" {
		msg.WriteString(fmt.Sprintf("(%s)", actorName))
	}
	if msg.Len() > 0 {
		msg.WriteString(": ")
	}
	if err != nil {
		msg.WriteString(fmt.Sprintf("Error \"%v\"", err))
		if template != "" {
			msg.WriteString(" - ")
		}
	}
	msg.WriteString(fmt.Sprintf(template, values...))
	return LintString(TruncateString(msg.String(), MaxLogMessageLen), MaxLogMessageLen)
}

// Print a log message and keep the message in warnings buffer.
func (logger *Logger) Warning(functionName, actorName string, err error, template string, values ...interface{}) {
	msg := logger.Format(functionName, actorName, err, template, values...)
	msgWithTime := time.Now().Format("2006-01-02 15:04:05 ") + msg
	log.Print(msg)
	// All warning messages to to the latest logs buffer
	LatestLogs.Push(msgWithTime)
	// As determined by the LRU buffer, only the first instance of warning from this actor (identified by component name + function name +
	// actor name) will be added to the in-memory warning log buffer, this helps to gain a more comprehensive picture of actors behind latest
	// warning messages by suppressing the noisest actors.
	if alreadyPresent, _ := LatestWarningActors.Add(functionName + actorName); !alreadyPresent {
		LatestWarnings.Push(msgWithTime)
		if GlobalLogWarningCallback != nil {
			go GlobalLogWarningCallback(logger.ComponentName, logger.getComponentIDs(), functionName, actorName, err, fmt.Sprintf(template, values...))
		}
	}
}

// Print a log message and keep the message in latest log buffer. If there is an error, also keep the message in warnings buffer.
func (logger *Logger) Info(functionName, actorName string, err error, template string, values ...interface{}) {
	// If the log message comes with an error, upgrade it to a warning.
	if err != nil {
		logger.Warning(functionName, actorName, err, template, values...)
		return
	}
	msg := logger.Format(functionName, actorName, err, template, values...)
	msgWithTime := time.Now().Format("2006-01-02 15:04:05 ") + msg
	LatestLogs.Push(msgWithTime)
	log.Print(msg)
}

func (logger *Logger) Abort(functionName, actorName string, err error, template string, values ...interface{}) {
	log.Fatal(logger.Format(functionName, actorName, err, template, values...))
}

func (logger *Logger) Panic(functionName, actorName string, err error, template string, values ...interface{}) {
	log.Panic(logger.Format(functionName, actorName, err, template, values...))
}

// MaybeMinorError logs the input error, which by convention is minor in nature, in an info log message.
// As a special case, if the error indicates the closure of a network connection, or includes the keyword "broken",
// then no log message will be written.
func (logger *Logger) MaybeMinorError(err error) {
	if err != nil && !strings.Contains(err.Error(), "closed") && !strings.Contains(err.Error(), "broken") {
		logger.Info("", "", nil, "minor error - %s", err.Error())
	}
}

// DefaultLogger must be used when it is not possible to acquire a reference to a more dedicated logger.
var DefaultLogger = &Logger{ComponentName: "default", ComponentID: []LoggerIDField{{"PID", os.Getpid()}}}

/*
TruncateString returns the input string as-is if it is less or equal to the desired length. Otherwise, it removes text
from the middle of string to fit to the desired length, and substitutes the removed portion with text
"...(truncated)..." and then returns.
*/
func TruncateString(in string, maxLength int) string {
	if maxLength < 0 {
		maxLength = 0
	}
	if len(in) > maxLength {
		if maxLength <= len(truncatedLabel) {
			return in[:maxLength]
		}
		// Grab the beginning and end of the string
		firstHalfEnd := maxLength/2 - len(truncatedLabel)/2
		secondHalfBegin := len(in) - (maxLength / 2) + len(truncatedLabel)/2
		if maxLength%2 == 0 {
			secondHalfBegin++
		}
		var truncatedMsg bytes.Buffer
		truncatedMsg.WriteString(in[:firstHalfEnd])
		truncatedMsg.WriteString(truncatedLabel)
		truncatedMsg.WriteString(in[secondHalfBegin:])
		return truncatedMsg.String()
	}
	return in
}

/*
LintString returns a copy of the input string with unusual characters (such as non-printable characters and record
separators) replaced by an underscore. Consequently, printable characters such as CJK languages are also replaced.
Additionally the string return value is capped to the maximum specified length.
*/
func LintString(in string, maxLength int) string {
	if maxLength < 0 {
		maxLength = 0
	}
	var cleanedResult bytes.Buffer
	for i, r := range in {
		if i >= maxLength {
			break
		}
		if (r >= 0 && r <= 8) || // Skip NUL...Backspace
			(r >= 14 && r <= 31) || // Skip ShiftOut..UnitSeparator
			(r >= 127) || // Skip those beyond ASCII table
			(!unicode.IsPrint(r) && !unicode.IsSpace(r)) { // Skip non-printable
			cleanedResult.WriteRune('_')
		} else {
			cleanedResult.WriteRune(r)
		}
	}
	return cleanedResult.String()
}

// ByteArrayLogString returns a human-readable string for the input byte array.
// The returned string is only suitable for log messages.
func ByteArrayLogString(data []byte) string {
	var countBinaryBytes int
	for _, b := range data {
		if (b <= 8) || // NUL...Backspace
			(b >= 14 && b <= 31) || // ShiftOut..UnitSeparator
			(b >= 127) || // Past the basic ASCII table
			(!unicode.IsPrint(rune(b)) && !unicode.IsSpace(rune(b))) { // Non-printable
			countBinaryBytes++
		}
	}
	if float32(countBinaryBytes)/float32(len(data)) > 0.5 {
		return fmt.Sprintf("%#v", data)
	}
	return LintString(string(data), 1000)
}
