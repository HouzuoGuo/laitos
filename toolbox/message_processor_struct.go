package toolbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	// SubjectReportSerialisedFieldSeparator is a separator character, ASCII Unit Separator, used to compact a subject report request into a single string.
	SubjectReportSerialisedFieldSeparator = '\x1f'

	// SubjectReportSerialisedLineSeparator is a separator character, ASCII Record Separator, used in between lines of a compacted subject report request.
	SubjectReportSerialisedLineSeparator = '\x1e'

	// MaxSubjectCommentStringLen is the maximum length of a comment coming in from a subject report request.
	// If a comment exceeds this length, then it will be truncated to the length before it is stored in memory.
	// Should truncation occurr, the truncated comment will be stored as a string, instead of a deserialised JSON object.
	MaxSubjectCommentStringLen = 4 * 1024
)

/*
AppCommandRequest describes an app command that message processor (local) would like monitored subject (remote to run).
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
	// SubjectHostName is the self-reported host name of the subject's computer. Message processor uses the name to uniquely identify the monitored subject.
	SubjectHostName string
	// SubjectIP is the subject's self-reported public IP address. It may potentially differ from the client IP observed by the server daemon receiving this report.
	SubjectIP string
	// SubjectPlatform is the OS and CPU architecture of the subject's computer (GOOS/GOARCH).
	SubjectPlatform string
	// SubjectComment is a free from JSON object/string the subject voluntarily includes in this report.
	SubjectComment interface{}

	// ServerTime is overwritten by server upon receiving the request, it is not supplied by a subject, and only used by the server internally.
	ServerTime time.Time `json:"-"`

	// CommandRequest is an app command that the subject (remote) would like this message processor (local) to run, if any.
	CommandRequest AppCommandRequest
	// CommandResponse is result of app command execution the subject (remote) made in response to command request originated from this message processor (local).
	CommandResponse AppCommandResponse
}

// Lint truncates attributes of the request to ensure that none of the attributes are exceedingly long.
func (req *SubjectReportRequest) Lint() {
	if len(req.SubjectIP) > 64 {
		// The text representation of An IPv6 address may use up to ~40 characters
		req.SubjectIP = req.SubjectIP[:64]
	}
	if len(req.SubjectHostName) > 256 {
		// A DNS name may be up to 254 characters long
		req.SubjectHostName = req.SubjectHostName[:256]
	}
	if len(req.SubjectPlatform) > 128 {
		req.SubjectPlatform = req.SubjectPlatform[:128]
	}
	// The size of the comment attribute is not checked if it is a JSON object
	if commentStr, isStr := req.SubjectComment.(string); isStr {
		if len(commentStr) > MaxSubjectCommentStringLen {
			// Allow up to 4KB of free form text to appear in the comment
			req.SubjectComment = commentStr[:MaxSubjectCommentStringLen]
		}
	}
	if len(req.CommandRequest.Command) > MaxCmdLength {
		req.CommandRequest.Command = req.CommandRequest.Command[:MaxCmdLength]
	}
	if len(req.CommandResponse.Command) > MaxCmdLength {
		req.CommandResponse.Command = req.CommandResponse.Command[:MaxCmdLength]
	}
}

/*
SerialiseCompact serialises the request into a compact string.
The fields carried by the serialised string rank from most important to least important.
*/
func (req *SubjectReportRequest) SerialiseCompact() string {
	var serialisedComment string
	if commentStr, isStr := req.SubjectComment.(string); isStr {
		serialisedComment = commentStr
	} else {
		if commentJSON, err := json.Marshal(req.SubjectComment); err == nil {
			serialisedComment = string(commentJSON)
		}
	}
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
		strings.ReplaceAll(serialisedComment, "\n", fmt.Sprintf("%c", SubjectReportSerialisedLineSeparator)),
		SubjectReportSerialisedFieldSeparator,
		req.SubjectIP,
		SubjectReportSerialisedFieldSeparator,

		req.CommandResponse.ReceivedAt.Unix(),
		SubjectReportSerialisedFieldSeparator,
		req.CommandResponse.RunDurationSec,
	)
}

// ErrSubjectReportTruncated is returned when a subject report has been truncated during its transport, therefore not all of the fields were decoded successfully.
// See also "DeserialiseFromCompact".
var ErrSubjectReportTruncated = errors.New("the subject report request or response appears to have been truncated")

/*
DeserialiseFromCompact deserialises the report request from the compact input string. If the input string is incomplete or truncated during its transport,
the function will try to decode as much information as possible while returning ErrSubjectReportRequestTruncated.
*/
func (req *SubjectReportRequest) DeserialiseFromCompact(in string) error {
	attributes := strings.Split(in, fmt.Sprintf("%c", SubjectReportSerialisedFieldSeparator))
	if len(attributes) > 0 {
		req.SubjectHostName = attributes[0]
	}
	if len(attributes) > 1 {
		req.CommandRequest.Command = attributes[1]
	}
	if len(attributes) > 2 {
		req.CommandResponse.Command = attributes[2]
	}
	if len(attributes) > 3 {
		req.CommandResponse.Result = strings.ReplaceAll(attributes[3], fmt.Sprintf("%c", SubjectReportSerialisedLineSeparator), "\n")
	}
	if len(attributes) > 4 {
		req.SubjectPlatform = attributes[4]
	}
	if len(attributes) > 5 {
		commentAttribute := strings.ReplaceAll(attributes[5], fmt.Sprintf("%c", SubjectReportSerialisedLineSeparator), "\n")
		req.SubjectComment = commentAttribute
		if len(commentAttribute) > MaxSubjectCommentStringLen {
			// Truncate the oversize comment, do not attempt to decode it into a JSON object.
			req.SubjectComment = commentAttribute[:MaxSubjectCommentStringLen]
		} else {
			// Allow up to 4KB of free form text to be decoded into a JSON object
			var commentJSON map[string]interface{}
			if err := json.Unmarshal([]byte(commentAttribute), &commentJSON); err == nil {
				req.SubjectComment = commentJSON
			}
		}
	}
	if len(attributes) > 6 {
		req.SubjectIP = attributes[6]
	}
	if len(attributes) > 7 {
		unixTimeSec, _ := strconv.Atoi(attributes[7])
		req.CommandResponse.ReceivedAt = time.Unix(int64(unixTimeSec), 0)
	}
	if len(attributes) > 8 {
		durationSec, _ := strconv.Atoi(attributes[8])
		req.CommandResponse.RunDurationSec = durationSec
	}
	if len(attributes) != 9 {
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
