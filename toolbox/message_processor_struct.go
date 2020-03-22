package toolbox

import "time"

/*
AppCommandRequest consists of an app command to run. It is embedded in subject report requests or server responses when one of them would
like the other party to run an app command.
The JSON attribute tags are deliberately kept short to save bandwidth in transit.
*/
type AppCommandRequest struct {
	// Command is a complete app command following the conventional format.
	Command string `json:"cmd"`
}

/*
AppCommandResponse consists of an app command, timestamp, and execution stats. After a subject or a server has completed an app command
previously requested by the other party, this response will be embedded in the next message exchange.
The JSON attribute tags are deliberately kept short to save bandwidth in transit.
*/
type AppCommandResponse struct {
	// Command is the app command that was run.
	Command string `json:"cmd"`
	// ReceivedAt is the timestamp of the command that was received.
	ReceivedAt time.Time `json:"at"`
	// Response is the app command execution result.
	Result string `json:"resp"`
	// Duration is the number of seconds the app command took to run.
	RunDurationSec int `json:"dur"`
}

/*
SubjectReportRequest is a request message consisting of subject's system status and result from most recent command execution (if asked),
regularly transmitted to a server.
The JSON attribute tags are deliberately kept short to save bandwidth in transit.
*/
type SubjectReportRequest struct {
	// SubjectIP is the public IP address of the computer. This may not be identical to the client IP observed by server.
	SubjectIP string `json:"ip"`
	// SubjectHostName is the host name of the computer.
	SubjectHostName string `json:"host"`
	// SubjectPlatform is the OS and CPU architecture of the computer (GOOS/GOARCH).
	SubjectPlatform string `json:"plat"`
	// SubjectComment is a free from text the subject chooses to include in this report.
	SubjectComment string `json:"comment"`

	// ServerTime is overwritten by server upon receiving the request, it is not supplied by a subject, and only used by the server internally.
	ServerTime time.Time `json:"-"`

	// CommandRequest is an app command that the subject would like server to run (if any).
	CommandRequest AppCommandRequest `json:"req"`
	// CommandResponse is an response for a command that server previously asked the subject to run (if asked).
	CommandResponse AppCommandResponse `json:"resp"`
}

/*
SubjectReportResponse is made in reply to a report, the response consists of a pending app command for the subject to run (if any), or result
from most recent command an agent asked the server to run (if asked).
The JSON attribute tags are deliberately kept short to save bandwidth in transit.
*/
type SubjectReportResponse struct {
	// CommandRequest is an app command that the server would like a subject to run (if any).
	CommandRequest AppCommandRequest `json:"req"`
	// CommandResponse is an response for a command that a subject previously asked the server to run (if any).
	CommandResponse AppCommandResponse `json:"resp"`
}
