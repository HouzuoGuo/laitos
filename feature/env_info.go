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

var ErrBadEnvInfoChoice = errors.New(`elock | estop | log | warn | runtime | stack`)

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
		return &Result{Output: "successfully triggered EmergencyStop"}
	case "runtime":
		return &Result{Output: GetRuntimeInfo()}
	case "log":
		return &Result{Output: GetLatestLog()}
	case "warn":
		return &Result{Output: GetLatestWarnings()}
	case "stack":
		return &Result{Output: GetGoroutineStacktraces()}
	default:
		return &Result{Error: ErrBadEnvInfoChoice}
	}
}

// Return runtime information (uptime, CPUs, goroutines, memory usage) in a multi-line text.
func GetRuntimeInfo() string {
	usedMem, totalMem := env.GetSystemMemoryUsageKB()
	return fmt.Sprintf(`IP: %s
Clock: %s
Sys/prog uptime: %s / %s
Total/used/prog mem: %d / %d / %d MB
Sys load: %s
Num CPU/GOMAXPROCS/goroutines: %d / %d / %d
`,
		env.GetPublicIP(),
		time.Now().String(),
		time.Duration(env.GetSystemUptimeSec()*int(time.Second)).String(), time.Now().Sub(global.StartupTime).String(),
		totalMem/1024, usedMem/1024, env.GetProgramMemoryUsageKB()/1024,
		env.GetSystemLoad(),
		runtime.NumCPU(), runtime.GOMAXPROCS(0), runtime.NumGoroutine())
}

// Return latest log entry of all kinds in a multi-line text, one log entry per line. Latest log entry comes first.
func GetLatestLog() string {
	buf := new(bytes.Buffer)
	global.LatestLogs.Iterate(func(entry string) bool {
		buf.WriteString(entry)
		buf.WriteRune('\n')
		return true
	})
	return buf.String()
}

// Return latest warning entries in a multi-line text, one log entry per line. Latest entry comes first.
func GetLatestWarnings() string {
	buf := new(bytes.Buffer)
	global.LatestWarnings.Iterate(func(entry string) bool {
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
