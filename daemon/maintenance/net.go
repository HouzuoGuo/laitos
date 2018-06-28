package maintenance

import (
	"bytes"
	"github.com/HouzuoGuo/laitos/misc"
	"strconv"
	"strings"
)

// MaintainsIptables blocks ports that are not listed in allowed port and throttle incoming traffic.
func (daemon *Daemon) MaintainsIptables(out *bytes.Buffer) {
	if daemon.BlockPortsExcept == nil || len(daemon.BlockPortsExcept) == 0 {
		return
	}
	if misc.HostIsWindows() {
		daemon.logPrintStage(out, "skipped on windows: maintain iptables")
		return
	}
	daemon.logPrintStage(out, "maintain iptables")
	if daemon.ThrottleIncomingPackets < 5 {
		daemon.logPrintStageStep(out, "ThrottleIncomingPackets(%d) must be greater or equal to 5", daemon.ThrottleIncomingPackets)
		return
	}
	if daemon.ThrottleIncomingPackets > 255 {
		daemon.logPrintStageStep(out, "ThrottleIncomingPackets(%d) must be less than 256, resetting it to 255.", daemon.ThrottleIncomingPackets)
		daemon.ThrottleIncomingPackets = 255
	}
	// Fail safe commands are executed if the usual commands encounter an error. The fail safe permits all traffic.
	failSafe := [][]string{
		{"-F", "OUTPUT"},
		{"-P", "OUTPUT", "ACCEPT"},
		{"-F", "INPUT"},
		{"-P", "INPUT", "ACCEPT"},
	}
	// These are the usual setup commands. Begin by clearing iptables.
	iptables := [][]string{
		{"-F", "OUTPUT"},
		{"-P", "OUTPUT", "ACCEPT"},
		{"-P", "INPUT", "DROP"},
		{"-F", "INPUT"},
	}
	// Work around a redhat kernel bug that prevented throttle counter from exceeding 20
	for _, cmd := range iptables {
		ipOut, ipErr := misc.InvokeProgram(nil, 10, "iptables", cmd...)
		if ipErr != nil {
			daemon.logPrintStageStep(out, "failed in a step that clears iptables - %v - %s", ipErr, ipOut)
		}
	}
	mOut, mErr := misc.InvokeProgram(nil, 10, "modprobe", "-r", "xt_recent")
	daemon.logPrintStageStep(out, "disable xt_recent - %v - %s", mErr, mOut)
	mOut, mErr = misc.InvokeProgram(nil, 10, "modprobe", "xt_recent", "ip_pkt_list_tot=255")
	daemon.logPrintStageStep(out, "re-enable xt_recent - %v - %s", mErr, mOut)

	// After clearing iptables, allow ICMP, established connections, and localhost to communicate
	iptables = append(iptables,
		[]string{"-A", "INPUT", "-p", "icmp", "-j", "ACCEPT"},
		[]string{"-A", "INPUT", "-m", "conntrack", "--ctstate", "INVALID", "-j", "DROP"},
		[]string{"-A", "INPUT", "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT"},
		[]string{"-A", "INPUT", "-i", "lo", "-j", "ACCEPT"},
		[]string{"-A", "INPUT", "-s", "127.0.0.0/8", "-j", "ACCEPT"},
	)
	// Throttle ports
	for _, port := range daemon.BlockPortsExcept {
		// Throttle new TCP connections
		iptables = append(iptables, []string{"-A", "INPUT", "-p", "tcp", "--dport", strconv.Itoa(port), "-m", "conntrack", "--ctstate", "NEW", "-m", "recent", "--set"})
		iptables = append(iptables, []string{"-A", "INPUT", "-p", "tcp", "--dport", strconv.Itoa(port), "-m", "conntrack", "--ctstate", "NEW", "-m", "recent", "--update", "--seconds", "1", "--hitcount", strconv.Itoa(daemon.ThrottleIncomingPackets), "-j", "DROP"})
		iptables = append(iptables, []string{"-A", "INPUT", "-p", "tcp", "--dport", strconv.Itoa(port), "-m", "conntrack", "--ctstate", "NEW,ESTABLISHED", "-j", "ACCEPT"})

		// Throttle UDP packets
		iptables = append(iptables, []string{"-A", "INPUT", "-p", "udp", "--dport", strconv.Itoa(port), "-m", "recent", "--set"})
		iptables = append(iptables, []string{"-A", "INPUT", "-p", "udp", "--dport", strconv.Itoa(port), "-m", "recent", "--update", "--seconds", "1", "--hitcount", strconv.Itoa(daemon.ThrottleIncomingPackets), "-j", "DROP"})
		iptables = append(iptables, []string{"-A", "INPUT", "-p", "udp", "--dport", strconv.Itoa(port), "-j", "ACCEPT"})
	}
	// Safety rule
	iptables = append(iptables, []string{"-A", "INPUT", "-j", "DROP"})
	// Run setup commands
	for _, args := range iptables {
		ipOut, ipErr := misc.InvokeProgram(nil, 10, "iptables", args...)
		if ipErr != nil {
			daemon.logPrintStageStep(out, "command failed for \"%s\" - %v - %s", strings.Join(args, " "), ipErr, ipOut)
			daemon.logPrintStageStep(out, "configure for fail safe that will allow ALL traffic")
			for _, failSafeCmd := range failSafe {
				failSafeOut, failSafeErr := misc.InvokeProgram(nil, 10, "iptables", failSafeCmd...)
				daemon.logPrintStageStep(out, "fail safe \"%s\" - %v - %s", strings.Join(failSafeCmd, " "), failSafeErr, failSafeOut)
			}
			return
		}
	}
	// Do not touch NAT and Forward as they might have been manipulated by docker daemon
}
