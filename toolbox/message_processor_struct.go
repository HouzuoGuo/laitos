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
AppCommandRequest is an app command that a subject (remote) would like this message processor (local) to run.
The app command follows the conventional format, e.g. "PasswordPIN.e info".
*/
type AppCommandRequest struct {
	Command string // Command is a complete app command following the conventional format.
}

/*
AppCommandResponse is the result of app command executed by this message processor (local), as a result of request made by a subject (remote).
The result field is updated upon completion of the command execution.
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
SubjectReportRequest is a subject's (remote's) system description and status report transmitted at regular interval to this message
processor (local).
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

	// CommandRequest is an app command that the subject (remote) would like this message processor (local) to run, if any.
	CommandRequest AppCommandRequest
	// CommandResponse is result of app command execution the subject (remote) made in response to command request originated from this message processor (local).
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
SubjectReportResponse is a reply made in response to a subject (remote) report. It may embed a command request that this
message processor (local) would like the subject (remote) to execute, and/or result from app command execution this
message processor (local) made in response to a previous command request from subject (remote).
*/
type SubjectReportResponse struct {
	// CommandRequest is an app command that this message processor (local) would like subject (remote) to run.
	CommandRequest AppCommandRequest
	// CommandResponse is the result from app command that subject (remote) previously asked local to execute.
	CommandResponse AppCommandResponse
}
