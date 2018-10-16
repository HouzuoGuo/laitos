package lalog

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"time"
)

const (
	NumLatestLogEntries = 256  // Keep this number of latest log entries in memory
	MaxLogMessageLen    = 2048 // Truncate long log messages to this length
)

var LatestLogs = NewRingBuffer(NumLatestLogEntries)     // Keep latest log entry of all kinds in the buffer
var LatestWarnings = NewRingBuffer(NumLatestLogEntries) // Keep latest warnings and log entries that come with error in the buffer

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

// Format a log message and return, but do not print it.
func (logger *Logger) Format(functionName, actorName string, err error, template string, values ...interface{}) string {
	// Message is going to look like this:
	// ComponentName[IDKey1-IDVal1;IDKey2-IDVal2].FunctionName(actorName): Error "no such file" - failed to start component
	var msg bytes.Buffer
	if logger.ComponentName != "" {
		msg.WriteString(logger.ComponentName)
	}
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
	if msg.Len() > 0 {
		msg.WriteRune('.')
	}
	if functionName != "" {
		msg.WriteString(fmt.Sprintf("%s", functionName))
	}
	if actorName != "" {
		msg.WriteString(fmt.Sprintf("(%s)", actorName))
	}
	if msg.Len() > 0 {
		msg.WriteString(": ")
	}
	if err != nil {
		msg.WriteString(fmt.Sprintf("Error \"%v\" - ", err))
	}
	msg.WriteString(fmt.Sprintf(template, values...))
	if msg.Len() > MaxLogMessageLen {
		// Grab the beginning and end of the message
		truncatedLabel := "...(truncated)..."
		firstHalfEnd := MaxLogMessageLen/2 - len(truncatedLabel)/2
		secondHalfBegin := msg.Len() - (MaxLogMessageLen / 2) + len(truncatedLabel)/2 + 1
		var truncatedMsg bytes.Buffer
		truncatedMsg.Write(msg.Bytes()[:firstHalfEnd])
		truncatedMsg.WriteString(truncatedLabel)
		truncatedMsg.Write(msg.Bytes()[secondHalfBegin:])
		msg = truncatedMsg
	}
	return msg.String()
}

// Print a log message and keep the message in warnings buffer.
func (logger *Logger) Warning(functionName, actorName string, err error, template string, values ...interface{}) {
	msg := logger.Format(functionName, actorName, err, template, values...)
	msgWithTime := time.Now().Format("2006-01-02 15:04:05 ") + msg
	LatestLogs.Push(msgWithTime)
	LatestWarnings.Push(msgWithTime)
	log.Print(msg)
}

// Print a log message and keep the message in latest log buffer. If there is an error, also keep the message in warnings buffer.
func (logger *Logger) Info(functionName, actorName string, err error, template string, values ...interface{}) {
	msg := logger.Format(functionName, actorName, err, template, values...)
	msgWithTime := time.Now().Format("2006-01-02 15:04:05 ") + msg
	LatestLogs.Push(msgWithTime)
	if err != nil {
		// If the log message comes with an error, upgrade the severity level to warning, so place it into recent warnings.
		LatestWarnings.Push(msgWithTime)
	}
	log.Print(msg)
}

func (logger *Logger) Abort(functionName, actorName string, err error, template string, values ...interface{}) {
	log.Fatal(logger.Format(functionName, actorName, err, template, values...))
}

func (logger *Logger) Panic(functionName, actorName string, err error, template string, values ...interface{}) {
	log.Panic(logger.Format(functionName, actorName, err, template, values...))
}

// DefaultLogger must be used when it is not possible to acquire a reference to a more dedicated logger.
var DefaultLogger = &Logger{ComponentName: "default", ComponentID: []LoggerIDField{{"PID", os.Getpid()}}}
