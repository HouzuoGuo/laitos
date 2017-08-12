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

var ErrBadEnvInfoChoice = errors.New(`e(lock/stop/kill) | log | warn | runtime | stack | tune`)

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
		return &Result{Output: "OK - EmergencyLockDown"}
	case "estop":
		global.TriggerEmergencyStop()
		return &Result{Output: "OK - EmergencyStop"}
	case "ekill":
		global.TriggerEmergencyKill()
		return &Result{Output: "OK - EmergencyKill"}
	case "runtime":
		return &Result{Output: GetRuntimeInfo()}
	case "log":
		return &Result{Output: GetLatestLog()}
	case "warn":
		return &Result{Output: GetLatestWarnings()}
	case "stack":
		return &Result{Output: GetGoroutineStacktraces()}
	case "tune":
		return &Result{Output: TuneLinux()}
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

/*
TuneLinux tweaks Linux-specific system parameters to ensure optimal operation and
maximum utilisation of system resources. Return human-readable description of how values
have been tweaked (i.e. the differences).
*/
func TuneLinux() string {
	// The following settings have little influence on system resources
	assignment := map[string]string{
		"net.ipv4.ip_forward":                   "0",
		"net.ipv6.ip_forward":                   "0",
		"net.ipv4.conf.all.mc_forwarding":       "0",
		"net.ipv6.conf.all.mc_forwarding":       "0",
		"net.ipv4.conf.all.accept_redirects":    "0",
		"net.ipv6.conf.all.accept_redirects":    "0",
		"net.ipv4.conf.all.accept_source_route": "0",
		"net.ipv6.conf.all.accept_source_route": "0",
		"net.ipv4.conf.all.secure_redirects":    "0",
		"net.ipv6.conf.all.secure_redirects":    "0",
		"net.ipv4.conf.all.send_redirects":      "0",
		"net.ipv6.conf.all.send_redirects":      "0",

		"net.ipv4.icmp_echo_ignore_broadcasts":       "1",
		"net.ipv4.icmp_ignore_bogus_error_responses": "1",
		"net.ipv4.tcp_syncookies":                    "1",
		"net.ipv4.conf.default.rp_filter":            "1",

		"net.ipv4.tcp_mtu_probing":      "2",
		"net.ipv4.tcp_base_mss":         "1024",
		"net.ipv4.tcp_keepalive_time":   "120",
		"net.ipv4.tcp_keepalive_intvl":  "30",
		"net.ipv4.tcp_keepalive_probes": "4",

		"net.ipv4.tcp_congestion_control": "hybla",
		"net.ipv4.tcp_tw_recycle":         "0",
		"net.ipv4.tcp_tw_reuse":           "1",
		"net.ipv4.tcp_fastopen":           "3",
		"net.ipv4.ip_local_port_range":    "2048\t65535",

		"kernel.sysrq": "0",
		"kernel.panic": "10",
	}
	// The following settings have greater influence on system resources
	_, memSizeKB := env.GetSystemMemoryUsageKB()
	atLeast := map[string]int{
		"net.core.somaxconn":           memSizeKB / 1024 / 512 * 256,  // 256 per 512MB of mem
		"net.ipv4.tcp_max_syn_backlog": memSizeKB / 1024 / 512 * 512,  // 512 per 512MB of mem
		"net.core.netdev_max_backlog":  memSizeKB / 1024 / 512 * 1024, // 1024 per 512MB of mem
		"net.ipv4.tcp_max_tw_buckets":  memSizeKB / 1024 / 512 * 2048, // 2048 per 512MB of mem
	}
	// Apply the optimal and return human-readable tuning result
	var ret bytes.Buffer
	for key, val := range assignment {
		old, err := env.SetSysctl(key, val)
		if err == nil {
			if old != val {
				ret.WriteString(fmt.Sprintf("%s: %v -> %v\n", key, old, val))
			}
		} else {
			ret.WriteString(fmt.Sprintf("Failed to set %s - %v\n", key, err))
		}
	}
	for key, val := range atLeast {
		old, err := env.IncreaseSysctlInt(key, val)
		if err == nil {
			if old < val {
				ret.WriteString(fmt.Sprintf("%s: %v -> %v\n", key, old, val))
			}
		} else {
			ret.WriteString(fmt.Sprintf("Failed to set %s - %v\n", key, err))
		}
	}
	return ret.String()
}
