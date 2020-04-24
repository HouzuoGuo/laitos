package toolbox

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

/*
SubjectReportSerialisedFieldSeparator is a separator character, ASCII Unit Separator, used to compact a subject report request into a
single string.
*/
const SubjectReportSerialisedFieldSeparator = '\x1f'

/*
SubjectReportSerialisedLineSeparator is a separator character, ASCII Record Separator, used in between lines of a compacted subject report
request.
*/
const SubjectReportSerialisedLineSeparator = '\x1e'

/*
AppCommandRequest consists of an app command to run. It is embedded in subject report requests or server responses when one of them would
like the other party to run an app command.
The JSON attribute tags are deliberately kept short to save bandwidth in transit.
*/
type AppCommandRequest struct {
	// Command is a complete app command following the conventional format.
	Command string
}

/*
AppCommandResponse consists of an app command, timestamp, and execution stats. After a subject or a server has completed an app command
previously requested by the other party, this response will be embedded in the next message exchange.
The JSON attribute tags are deliberately kept short to save bandwidth in transit.
*/
type AppCommandResponse struct {
	// Command is the app command that was run.
	Command string
	// ReceivedAt is the timestamp of the command that was received.
	ReceivedAt time.Time
	// Response is the execution result of a previously executed app command.
	Result string
	// Duration is the number of seconds the app command took to run.
	RunDurationSec int
}

/*
SubjectReportRequest is a request message consisting of subject's system status and result from most recent command execution (if asked),
regularly transmitted to a server.
The JSON attribute tags are deliberately kept short to save bandwidth in transit.
*/
type SubjectReportRequest struct {
	// SubjectIP is the public IP address of the computer. This may not be identical to the client IP observed by server.
	SubjectIP string
	// SubjectHostName is the host name of the computer.
	SubjectHostName string
	// SubjectPlatform is the OS and CPU architecture of the computer (GOOS/GOARCH).
	SubjectPlatform string
	// SubjectComment is a free from text the subject chooses to include in this report.
	SubjectComment string

	// ServerTime is overwritten by server upon receiving the request, it is not supplied by a subject, and only used by the server internally.
	ServerTime time.Time `json:"-"`

	// CommandRequest is an app command that the subject would like server to run (if any).
	CommandRequest AppCommandRequest
	// CommandResponse is an response for a command that server previously asked the subject to run (if asked).
	CommandResponse AppCommandResponse
}

/*
SerialiseCompact serialises the request into a compact string.
The fields carried by the serialised string rank from most important to least important.
*/
func (req *SubjectReportRequest) SerialiseCompact() string {
	return fmt.Sprintf("%s%c%s%c%s%c%s%c%s%c%s%c%s%c%d%c%d",
		// Ordered from most important to least important
		req.SubjectHostName,
		SubjectReportSerialisedFieldSeparator,
		req.CommandRequest.Command,
		SubjectReportSerialisedFieldSeparator,
		req.CommandResponse.Command,
		SubjectReportSerialisedFieldSeparator,
		strings.ReplaceAll(req.CommandResponse.Result, "\n", fmt.Sprintf("%c", SubjectReportSerialisedLineSeparator)),
		SubjectReportSerialisedFieldSeparator,

		req.SubjectPlatform,
		SubjectReportSerialisedFieldSeparator,
		strings.ReplaceAll(req.SubjectComment, "\n", fmt.Sprintf("%c", SubjectReportSerialisedLineSeparator)),
		SubjectReportSerialisedFieldSeparator,
		req.SubjectIP,
		SubjectReportSerialisedFieldSeparator,

		req.CommandResponse.ReceivedAt.Unix(),
		SubjectReportSerialisedFieldSeparator,
		req.CommandResponse.RunDurationSec,
	)
}

var ErrSubjectReportTruncated = errors.New("the subject report request or response appears to have been truncated")

/*
DeserialiseFromCompact deserialises the report request from the compact input string.
If the input string is incomplete or truncated, the function will try to decode as much information as possible while returning ErrSubjectReportRequestTruncated.
*/
func (req *SubjectReportRequest) DeserialiseFromCompact(in string) error {
	components := strings.Split(in, fmt.Sprintf("%c", SubjectReportSerialisedFieldSeparator))
	if len(components) > 0 {
		req.SubjectHostName = components[0]
	}
	if len(components) > 1 {
		req.CommandRequest.Command = components[1]
	}
	if len(components) > 2 {
		req.CommandResponse.Command = components[2]
	}
	if len(components) > 3 {
		req.CommandResponse.Result = strings.ReplaceAll(components[3], fmt.Sprintf("%c", SubjectReportSerialisedLineSeparator), "\n")
	}
	if len(components) > 4 {
		req.SubjectPlatform = components[4]
	}
	if len(components) > 5 {
		req.SubjectComment = strings.ReplaceAll(components[5], fmt.Sprintf("%c", SubjectReportSerialisedLineSeparator), "\n")
	}
	if len(components) > 6 {
		req.SubjectIP = components[6]
	}
	if len(components) > 7 {
		unixTimeSec, _ := strconv.Atoi(components[7])
		req.CommandResponse.ReceivedAt = time.Unix(int64(unixTimeSec), 0)
	}
	if len(components) > 8 {
		durationSec, _ := strconv.Atoi(components[8])
		req.CommandResponse.RunDurationSec = durationSec
	}
	if len(components) != 9 {
		return ErrSubjectReportTruncated
	}
	if req.SubjectHostName == "" {
		return errors.New("nothing could be decoded from the input")
	}
	return nil
}

/*
SubjectReportResponse is made in reply to a report, the response consists of a pending app command for the subject to run (if any), or result
from most recent command an agent asked the server to run (if asked).
The JSON attribute tags are deliberately kept short to save bandwidth in transit.
*/
type SubjectReportResponse struct {
	// CommandRequest is an app command that the server would like a subject to run (if any).
	CommandRequest AppCommandRequest
	// CommandResponse is an response for a command that a subject previously asked the server to run (if any).
	CommandResponse AppCommandResponse
}
