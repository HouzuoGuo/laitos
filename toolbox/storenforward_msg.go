package toolbox

import "time"

/*
AppCommandRequest consists of an app command to run. It is embedded in subject report requests or server responses when one of them would
like the other party to run an app command.
*/
type AppCommandRequest struct {
	Command string // Command is a complete app command following the conventional format.
}

/*
AppCommandResponse consists of an app command, timestamp, and execution stats. After a subject or a server has completed an app command
previously requested by the other party, this response will be embedded in the next message exchange.
*/
type AppCommandResponse struct {
	Command    string        // Command is the app command that was run.
	ReceivedAt time.Time     // ReceivedAt is the timestamp of the command that was received.
	Response   string        // Response is the app command execution result.
	Duration   time.Duration // Duration is how long it took to run the app command.
}

/*
SubjectReportRequest is a request message consisting of subject's system status and result from most recent command execution (if asked),
regularly transmitted to a server.
*/
type SubjectReportRequest struct {
	SubjectClock    time.Time // SubjectClock is the computer's system clock time.
	SubjectIP       string    // SubjectIP is the public IP address of the computer.
	SubjectHostName string    // SubjectHostName is the host name of the computer.
	SubjectPlatform string    // SubjectPlatform is the OS and CPU architecture of the computer (GOOS/GOARCH).

	ServerAddress string // ServerAddress is the server host name/IP address intended for handling this request.
	ServerDaemon  string // ServerDaemon is the server daemon intended for handling this request.
	ServerPort    int    // ServerPort is the port number (if applicable) of the server daemon intended for handling this request.

	// CommandRequest is an app command that the subject would like server to run (if any).
	CommandRequest AppCommandRequest
	// CommandResponse is an response for a command that server previously asked the subject to run (if asked).
	CommandResponse AppCommandResponse
}

/*
SubjectReportResponse is made in reply to a report, the response consists of a pending app command for the subject to run (if any), or result
from most recent command an agent asked the server to run (if asked).
*/
type SubjectReportResponse struct {
	// CommandRequest is an app command that the server would like a subject to run (if any).
	CommandRequest AppCommandRequest
	// CommandResponse is an response for a command that a subject previously asked the server to run (if any).
	CommandResponse AppCommandResponse
}
