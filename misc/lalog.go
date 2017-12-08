package misc

import (
	"bytes"
	"fmt"
	"log"
	"time"
)

const (
	NumLatestLogEntries = 256  // Keep this number of latest log entries in memory
	MaxLogMessageLen    = 2048 // Truncate long log messages to this length
)

var LatestLogs = NewRingBuffer(NumLatestLogEntries)     // Keep latest log entry of all kinds in the buffer
var LatestWarnings = NewRingBuffer(NumLatestLogEntries) // Keep latest warnings and log entries that come with error in the buffer

// Help to write log messages in a regular format.
type Logger struct {
	ComponentName string // Component name, such as HTTPD, SMTPD.
	ComponentID   string // Clue of which component instance the message comes from, e.g. "0.0.0.0:25"
}

// Format a log message and return, but do not print it.
func (logger *Logger) Format(functionName, actorName string, err error, template string, values ...interface{}) string {
	// Message is going to look like this:
	// ComponentName[ID].FunctionName(actorName): Error "no such file" - failed to start component
	var msg bytes.Buffer
	if logger.ComponentName != "" {
		msg.WriteString(logger.ComponentName)
	}
	if logger.ComponentID != "" {
		msg.WriteString(fmt.Sprintf("[%s]", logger.ComponentID))
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
		msg.Truncate(MaxLogMessageLen)
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
var DefaultLogger = &Logger{ComponentName: "default", ComponentID: ""}
