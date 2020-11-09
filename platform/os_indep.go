package platform

import (
	"os"

	"github.com/HouzuoGuo/laitos/lalog"
)

const (
	/*
		MaxExternalProgramOutputBytes is the maximum number of bytes (combined stdout and stderr) to keep for an
		external program for caller to retrieve.
	*/
	MaxExternalProgramOutputBytes = 1024 * 1024
)

var (
	// logger is used by some of the OS platform specific actions that affect laitos process globally.
	logger = lalog.Logger{ComponentName: "platform", ComponentID: []lalog.LoggerIDField{{Key: "PID", Value: os.Getpid()}}}
)

/*
InvokeShell launches an external shell process with time constraints to run a piece of shell code. The code is fed into
shell command parameter "-c", which happens to be universally accepted by Unix shells and Windows Powershell.
Returns shell stdout+stderr output combined and error if there is any. The maximum acount of output is capped to
MaxExternalProgramOutputBytes.
*/
func InvokeShell(timeoutSec int, interpreter string, content string) (out string, err error) {
	return InvokeProgram(nil, timeoutSec, interpreter, "-c", content)
}
