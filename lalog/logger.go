package lalog

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/HouzuoGuo/laitos/datastruct"
)

const (
	// MaxLogMessageLen is the maximum length memorised for each of the latest log entries.
	MaxLogMessageLen = 4096
	truncatedLabel   = "...(truncated)..."
)

type LogWarningCallbackFunc func(componentName, componentID, funcName string, actorName interface{}, err error, msg string)

var (
	// MaxLogMessagePerSec is the maximum number of messages each logger will be able to print out.
	// Any additional log messages will be dropped.
	MaxLogMessagePerSec = runtime.NumCPU() * 300

	// LatestWarnings are a small number of the most recent log messages
	// (warnings and info messages) kept in memory for retrieval and inspection.
	LatestLogs = datastruct.NewRingBuffer(1 * 1048576 / MaxLogMessageLen)

	// LatestWarnings are a small number of the most recent warning log messages kept in memory for retrieval and inspection.
	LatestWarnings = datastruct.NewRingBuffer(1 * 1048576 / MaxLogMessageLen)

	// LatestWarningActors is a small number of identifiers (actors) from recent
	// warning messages, they are used to de-duplicate these messages at regular
	// intervals to reduce spamming.
	LatestWarningActors = datastruct.NewLeastRecentlyUsedBuffer(1 * 1048576 / MaxLogMessageLen)

	// LatestWarningActors is a small number of message contents from recent log
	// messages of all types , they are used to de-duplicate these messages at
	// regular intervals to reduce spamming.
	LatestLogMessageContent = datastruct.NewLeastRecentlyUsedBuffer(1 * 1048576 / MaxLogMessageLen)

	// LogWarningCallback is invoked in a separate goroutine after any logger has processed a warning message.
	// The function must avoid generating a warning log message of itself, to avoid an infinite recursion.
	GlobalLogWarningCallback LogWarningCallbackFunc = nil

	// NumDropped is the number of de-duplicated log messages that are not
	// printed to stderr.
	NumDropped = new(atomic.Int64)
)

// Clear the global LRU buffers used for de-duplicating log messages.
func ClearDedupBuffers() {
	LatestWarningActors.Clear()
	LatestLogMessageContent.Clear()
}

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

	// initOnce is used to synchronise the initialisation of the logger upon first use.
	initOnce sync.Once
	// rateLimit throttles the logger to avoid inadvertently spamming stderr.
	rateLimit *RateLimit
}

func (logger *Logger) initialiseOnce() {
	logger.initOnce.Do(func() {
		logger.rateLimit = NewRateLimit(1, MaxLogMessagePerSec, logger)
	})
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
func (logger *Logger) Format(functionName string, actorName interface{}, err error, template string, values ...interface{}) string {
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
		msg.WriteString(fmt.Sprint(functionName))
	}
	if actorName != "" {
		msg.WriteString(fmt.Sprintf("(%v)", actorName))
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

func callerName(skip int) string {
	pc, file, _, ok := runtime.Caller(skip)
	if !ok {
		file = "?"
	}
	fun := runtime.FuncForPC(pc)
	var funName string
	if fun == nil {
		funName = "?"
	} else {
		funName = strings.TrimLeft(filepath.Ext(fun.Name()), ".")
	}
	return filepath.Base(file) + ":" + funName
}

func (logger *Logger) warning(funcName string, actorName interface{}, err error, template string, values ...interface{}) {
	// De-duplicate recent warnings from the same actor, and honour the logger instance's rate limit.
	if alreadyPresent, _ := LatestWarningActors.Add(funcName + fmt.Sprint(actorName)); alreadyPresent || !logger.rateLimit.Add("", false) {
		NumDropped.Add(1)
		return
	}
	msg := logger.Format(funcName, actorName, err, template, values...)
	log.Print(msg)

	msgWithTime := time.Now().Format("2006-01-02 15:04:05 ") + msg
	LatestLogs.Push(msgWithTime)
	LatestWarnings.Push(msgWithTime)

	if GlobalLogWarningCallback != nil {
		go GlobalLogWarningCallback(logger.ComponentName, logger.getComponentIDs(), funcName, actorName, err, fmt.Sprintf(template, values...))
	}
}

// Print a log message and keep the message in warnings buffer.
func (logger *Logger) Warning(actorName interface{}, err error, template string, values ...interface{}) {
	logger.initialiseOnce()
	funcName := callerName(2)
	logger.warning(funcName, actorName, err, template, values...)
}

func (logger *Logger) info(funcName string, actorName interface{}, err error, template string, values ...interface{}) {
	if err != nil {
		// If the log message comes with an error, treat it as a warning.
		logger.warning(funcName, actorName, err, template, values...)
		return
	}
	msg := logger.Format(funcName, actorName, err, template, values...)
	// De-duplicate recent log messages, and honour the logger instance's rate limit.
	if alreadyPresent, _ := LatestLogMessageContent.Add(msg); alreadyPresent || !logger.rateLimit.Add("", false) {
		NumDropped.Add(1)
		return
	}
	msgWithTime := time.Now().Format("2006-01-02 15:04:05 ") + msg
	log.Print(msg)
	LatestLogs.Push(msgWithTime)
}

// Print a log message and keep the message in latest log buffer. If there is an error, also keep the message in warnings buffer.
func (logger *Logger) Info(actorName interface{}, err error, template string, values ...interface{}) {
	logger.initialiseOnce()
	funcName := callerName(2)
	logger.info(funcName, actorName, err, template, values...)
}

func (logger *Logger) Abort(actorName interface{}, err error, template string, values ...interface{}) {
	logger.initialiseOnce()
	functionName := callerName(2)
	log.Fatal(logger.Format(functionName, actorName, err, template, values...))
}

func (logger *Logger) Panic(actorName interface{}, err error, template string, values ...interface{}) {
	logger.initialiseOnce()
	functionName := callerName(2)
	log.Panic(logger.Format(functionName, actorName, err, template, values...))
}

// MaybeMinorError logs the input error, which by convention is minor in nature, in an info log message.
// As a special case, if the error indicates the closure of a network connection, or includes the keyword "broken",
// then no log message will be written.
func (logger *Logger) MaybeMinorError(err error) {
	logger.initialiseOnce()
	funcName := callerName(2)
	if err != nil && !strings.Contains(err.Error(), "closed") && !strings.Contains(err.Error(), "broken") {
		logger.info(funcName, "", err, "minor error")
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
