package toolbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/platform"
)

var ErrBadEnvInfoChoice = errors.New(`lock | stop | kill | log | warn | runtime | stack | tune`)

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

func (info *EnvControl) Execute(ctx context.Context, cmd Command) *Result {
	if errResult := cmd.Trim(); errResult != nil {
		return errResult
	}
	switch strings.ToLower(cmd.Content) {
	case "lock":
		misc.TriggerEmergencyLockDown()
		return &Result{Output: "OK - EmergencyLockDown"}
	case "stop":
		misc.TriggerEmergencyStop()
		return &Result{Output: "OK - EmergencyStop"}
	case "kill":
		misc.TriggerEmergencyKill()
		return &Result{Output: "OK - EmergencyKill"}
	case "info":
		summary := platform.GetProgramStatusSummary(true)
		return &Result{Output: summary.String()}
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

// Return latest log entry of all kinds in a multi-line text, one log entry per line. Latest log entry comes first.
func GetLatestLog() string {
	buf := new(bytes.Buffer)
	lalog.LatestLogs.IterateReverse(func(entry string) bool {
		buf.WriteString(entry)
		buf.WriteRune('\n')
		return true
	})
	return buf.String()
}

// Return latest warning entries in a multi-line text, one log entry per line. Latest entry comes first.
func GetLatestWarnings() string {
	buf := new(bytes.Buffer)
	lalog.LatestWarnings.IterateReverse(func(entry string) bool {
		buf.WriteString(entry)
		buf.WriteRune('\n')
		return true
	})
	return buf.String()
}

// Return stack traces of all currently running goroutines.
func GetGoroutineStacktraces() string {
	buf := new(bytes.Buffer)
	_ = pprof.Lookup("goroutine").WriteTo(buf, 1)
	return buf.String()
}

/*
TuneLinux tweaks Linux-specific system parameters to ensure optimal operation and maximum utilisation of system
resources. Returns human-readable description of how values have been tweaked (i.e. the differences).
*/
func TuneLinux() string {
	if runtime.GOOS != "linux" {
		return "TuneLinux has nothing to do, system is not Linux."
	}
	_, memSizeKB := platform.GetSystemMemoryUsageKB()
	// The following settings have little influence on system resources
	assignment := map[string]string{
		// Optimise system security
		"fs.protected_fifos":        "1",
		"fs.protected_hardlinks":    "1",
		"fs.protected_regular":      "2",
		"fs.protected_symlinks":     "1",
		"kernel.core_uses_pid":      "1",
		"kernel.dmesg_restrict":     "1",
		"kernel.kptr_restrict":      "1",
		"kernel.panic":              "10",
		"kernel.pid_max":            "4194304",
		"kernel.randomize_va_space": "2",
		"kernel.sysrq":              "0",
		"kernel.yama.ptrace_scope":  "1",

		// Optimise system stability in low memory situation
		"vm.min_free_kbytes":   strconv.FormatInt(memSizeKB/32, 10), // reserve 1MB for every 32MB of system memory
		"vm.zone_reclaim_mode": "1",

		/*
			In earlier versions of laitos (< 1.3) IP forwarding used to be disabled right here, however, docker
			containers do not have Internet connectivity without IP forwarding, and remote browser control based on
			SlimerJS depends on docker container. Hence, IP forwarding is no longer disabled.
		*/
		// Optimise network security
		"net.ipv4.conf.all.accept_redirects":         "0",
		"net.ipv4.conf.all.accept_source_route":      "0",
		"net.ipv4.conf.all.log_martians":             "1",
		"net.ipv4.conf.all.mc_forwarding":            "0",
		"net.ipv4.conf.all.rp_filter":                "1",
		"net.ipv4.conf.all.secure_redirects":         "0",
		"net.ipv4.conf.all.send_redirects":           "0",
		"net.ipv4.conf.default.accept_redirects":     "0",
		"net.ipv4.conf.default.accept_source_route":  "0",
		"net.ipv4.conf.default.log_martians":         "1",
		"net.ipv4.conf.default.rp_filter":            "1",
		"net.ipv4.conf.default.secure_redirects":     "0",
		"net.ipv4.conf.default.send_redirects":       "0",
		"net.ipv4.icmp_echo_ignore_broadcasts":       "1",
		"net.ipv4.icmp_ignore_bogus_error_responses": "1",
		"net.ipv4.ip_local_port_range":               "2048\t65535",
		"net.ipv4.tcp_syncookies":                    "1",
		"net.ipv6.conf.all.accept_redirects":         "0",
		"net.ipv6.conf.all.accept_source_route":      "0",
		"net.ipv6.conf.all.mc_forwarding":            "0",
		"net.ipv6.conf.all.secure_redirects":         "0",
		"net.ipv6.conf.all.send_redirects":           "0",

		// Optimise network performance
		"net.core.default_qdisc":          "fq_codel",
		"net.core.netdev_max_backlog":     "16384",
		"net.core.somaxconn":              "4096",
		"net.ipv4.tcp_congestion_control": "bbr",
		"net.ipv4.tcp_fastopen":           "3",
		"net.ipv4.tcp_max_syn_backlog":    "2048",
		"net.ipv4.tcp_max_tw_buckets":     "262144",
		"net.ipv4.tcp_sack":               "1",

		// Optimise network compatibility
		"net.ipv4.conf.all.promote_secondaries":     "1",
		"net.ipv4.conf.default.promote_secondaries": "1",
		"net.ipv4.tcp_base_mss":                     "1024",
		"net.ipv4.tcp_mtu_probing":                  "2",
		"net.ipv4.tcp_rfc1337":                      "1",
		"net.ipv6.conf.all.disable_ipv6":            "0",
		"net.ipv6.conf.default.disable_ipv6":        "0",

		// Optimise network behaviour
		"net.ipv4.tcp_fin_timeout":      "10",
		"net.ipv4.tcp_keepalive_intvl":  "30",
		"net.ipv4.tcp_keepalive_probes": "4",
		"net.ipv4.tcp_keepalive_time":   "120",
		"net.ipv4.tcp_synack_retries":   "3",
		"net.ipv4.tcp_tw_recycle":       "0",
		"net.ipv4.tcp_tw_reuse":         "1",
	}
	// Apply the optimal and return human-readable tuning result
	var ret bytes.Buffer
	for key, val := range assignment {
		old, err := platform.SetSysctl(key, val)
		if err == nil {
			if old != val {
				ret.WriteString(fmt.Sprintf("%s: %v -> %v\n", key, old, val))
			}
		} else {
			ret.WriteString(fmt.Sprintf("Failed to set %s - %v\n", key, err))
		}
	}
	return ret.String()
}
