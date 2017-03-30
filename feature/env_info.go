package feature

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/env"
	"github.com/HouzuoGuo/laitos/global"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"
)

var ErrBadEnvInfoChoice = errors.New(`elock | estop | log | runtime | stack`)

// Retrieve environment information and trigger emergency stop upon request.
type EnvControl struct {
}

func (info *EnvControl) IsConfigured() bool {
	return true
}

func (info *EnvControl) SelfTest() error {
	return nil
}

func (info *EnvControl) Initialise() error {
	return nil
}

func (info *EnvControl) Trigger() Trigger {
	return ".e"
}

func (info *EnvControl) Execute(cmd Command) *Result {
	if errResult := cmd.Trim(); errResult != nil {
		return errResult
	}
	switch strings.ToLower(cmd.Content) {
	case "elock":
		global.TriggerEmergencyLockDown()
		return &Result{Output: "successfully triggered EmergencyLockDown"}
	case "estop":
		global.TriggerEmergencyStop()
		// Not reachable
		return &Result{Output: "successfully triggered EmergencyStop"}
	case "runtime":
		return &Result{Output: GetRuntimeInfo()}
	case "log":
		return &Result{Output: GetLatestGlobalLog()}
	case "stack":
		return &Result{Output: GetGoroutineStacktraces()}
	default:
		return &Result{Error: ErrBadEnvInfoChoice}
	}
}

// Return runtime information (uptime, CPUs, goroutines, memory usage) in a multi-line text.
func GetRuntimeInfo() string {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	return fmt.Sprintf(`Public IP: %s
Uptime: %s
Number of CPUs: %d
Number of Goroutines: %d
GOMAXPROCS: %d
System memory usage: %d MBytes
`,
		env.GetPublicIP(),
		time.Now().Sub(global.StartupTime).String(),
		runtime.NumCPU(),
		runtime.NumGoroutine(),
		runtime.GOMAXPROCS(0),
		memStats.Sys/1024/1024)
}

// Return latest log entries in a multi-line text, one log entry per line. Latest log entry comes first.
func GetLatestGlobalLog() string {
	buf := new(bytes.Buffer)
	global.LatestLogEntries.Iterate(func(entry string) bool {
		buf.WriteString(entry)
		buf.WriteRune('\n')
		return true
	})
	return buf.String()
}

// Return stack traces of all currently running goroutines.
func GetGoroutineStacktraces() string {
	buf := new(bytes.Buffer)
	pprof.Lookup("goroutine").WriteTo(buf, 1)
	return buf.String()
}
