package lalog

import (
	"bytes"
	"fmt"
	"log"
	"time"
)

const (
	NumLatestLogEntries = 512 // Keep this number of latest log entries in memory
)

var LatestLogEntries = NewRingBuffer(NumLatestLogEntries) // Keep latest log entries in the buffer
var StartupTime = time.Now()                              // Timestamp when this program started

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
	return msg.String()
}

func (logger *Logger) Printf(functionName, actorName string, err error, template string, values ...interface{}) {
	msg := logger.Format(functionName, actorName, err, template, values...)
	LatestLogEntries.Push(time.Now().Format("2006-01-02 15:04:05 ") + msg)
	log.Print(msg)
}

func (logger *Logger) Fatalf(functionName, actorName string, err error, template string, values ...interface{}) {
	log.Fatal(logger.Format(functionName, actorName, err, template, values...))
}

func (logger *Logger) Panicf(functionName, actorName string, err error, template string, values ...interface{}) {
	log.Panic(logger.Format(functionName, actorName, err, template, values...))
}
