package platform

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/testingstub"
)

const (
	/*
	   UtilityDir is an element of PATH that points to a directory where laitos bundled utility programs are stored. The
	   utility programs are not essential to most of laitos operations, however they come in handy in certain scenarios:
	   - statically linked "busybox" (maintenance daemon uses it to synchronise system clock)
	   - statically linked "toybox" (its rich set of utilities help with shell usage)
	*/
	UtilityDir = "/tmp/laitos-util"

	/*
	   CommonPATH is a PATH environment variable value that includes most common executable locations across Unix and Linux.
	   Be aware that, when laitos launches external programs they usually should inherit all of the environment variables from
	   parent process, which may include PATH. However, as an exception, AWS ElasticBeanstalk launches programs via a
	   "supervisord" that resets PATH variable to deliberately exclude sbin directories, therefore, it is often useful to use
	   this hard coded PATH value to launch programs.
	*/
	CommonPATH = UtilityDir + ":/opt/bin:/opt/sbin:/usr/local/bin:/usr/local/sbin:/usr/libexec:/usr/bin:/usr/sbin:/bin:/sbin"

	// CommonOSCmdTimeoutSec is the number of seconds to tolerate for running a wide range of system management utilities.
	CommonOSCmdTimeoutSec = 120
)

var (
	RegexVMRss          = regexp.MustCompile(`VmRSS:\s*(\d+)\s*kB`)        // Parse VmRss value from /proc/*/status line
	RegexMemAvailable   = regexp.MustCompile(`MemAvailable:\s*(\d+)\s*kB`) // Parse MemAvailable value from /proc/meminfo
	RegexMemTotal       = regexp.MustCompile(`MemTotal:\s*(\d+)\s*kB`)     // Parse MemTotal value from /proc/meminfo
	RegexMemFree        = regexp.MustCompile(`MemFree:\s*(\d+)\s*kB`)      // Parse MemFree value from /proc/meminfo
	RegexTotalUptimeSec = regexp.MustCompile(`(\d+).*`)                    // Parse uptime seconds from /proc/meminfo
)

// ProgramStatusSummary describes the system resource usage and process environment of this instance of laitos program running live.
type ProgramStatusSummary struct {
	PublicIP, HostName                         string
	ClockTime                                  time.Time
	SysUptime, ProgramUptime                   time.Duration
	SysTotalMemMB, SysUsedMemMB, ProgUsedMemMB int64
	DiskUsedMB, DiskFreeMB, DiskCapMB          int64
	SysLoad                                    string
	NumCPU, NumGoMaxProcs, NumGoroutines       int
	PID, PPID, UID, EUID, GID, EGID            int
	ExePath                                    string
	CLIFlags                                   []string
	WorkingDirPath                             string
	WorkingDirContent                          []string
	EnvironmentVars                            []string
}

// DeserialiseFromJSON deserialises JSON properties from the input JSON object into this summary item.
// The primary use of this function is in test cases.
func (summary *ProgramStatusSummary) DeserialiseFromJSON(jsonObj interface{}) error {
	jsonDoc, err := json.Marshal(jsonObj)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonDoc, summary)
}

func (summary ProgramStatusSummary) String() string {
	ret := fmt.Sprintf(`Host name: %s
Clock: %s
Sys/prog uptime: %s / %s
Total/used/prog mem: %d / %d / %d MB
Total/used/free rootfs: %d / %d / %d MB
Sys load: %s
Num CPU/GOMAXPROCS/goroutines: %d / %d / %d

Program PID/PPID: %d / %d
Program UID/EUID/GID/EGID: %d / %d / %d / %d
Program executable path: %s
Program CLI flags: %v
Program working directory: %s
Working directory content (max. 100 names): %v
Program environment (max. 100 entries): %v
`,
		summary.HostName,
		summary.ClockTime,
		summary.SysUptime, summary.ProgramUptime,
		summary.SysTotalMemMB, summary.SysUsedMemMB, summary.ProgUsedMemMB,
		summary.DiskCapMB, summary.DiskUsedMB, summary.DiskFreeMB,
		summary.SysLoad,
		summary.NumCPU, summary.NumGoMaxProcs, summary.NumGoroutines,

		summary.PID, summary.PPID,
		summary.UID, summary.EUID, summary.GID, summary.EGID,
		summary.ExePath,
		summary.CLIFlags,
		summary.WorkingDirPath,
		summary.WorkingDirContent,
		summary.EnvironmentVars)
	if summary.PublicIP != "" {
		return "IP: " + summary.PublicIP + "\n" + ret
	}
	return ret
}

// FindNumInRegexGroup uses the input regex to parse the string and then parses the decimal integer (up to 64-bit in size) specified in the
// group number. A gentle reminder - the entire match is at group number 0, and the first captured regex group is at number 1.
func FindNumInRegexGroup(numRegex *regexp.Regexp, input string, groupNum int) int64 {
	match := numRegex.FindStringSubmatch(input)
	if match == nil || len(match) <= groupNum {
		return 0
	}
	val, err := strconv.ParseInt(match[groupNum], 10, 64)
	if err == nil {
		return val
	}
	return 0
}

// Return RSS memory usage of this process. Return 0 if the memory usage cannot be determined.
func GetProgramMemoryUsageKB() int64 {
	statusContent, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	return FindNumInRegexGroup(RegexVMRss, string(statusContent), 1)
}

// Return operating system memory usage. Return 0 if the memory usage cannot be determined.
func GetSystemMemoryUsageKB() (usedKB, totalKB int64) {
	infoContent, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	totalKB = FindNumInRegexGroup(RegexMemTotal, string(infoContent), 1)
	available := FindNumInRegexGroup(RegexMemAvailable, string(infoContent), 1)
	if available == 0 {
		usedKB = totalKB - FindNumInRegexGroup(RegexMemFree, string(infoContent), 1)
	} else {
		usedKB = totalKB - available
	}
	return
}

// Return system load information and number of processes from /proc/loadavg. Return empty string if IO error occurs.
func GetSystemLoad() string {
	content, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}

// Get system uptime in seconds. Return 0 if it cannot be determined.
func GetSystemUptimeSec() int64 {
	content, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	return FindNumInRegexGroup(RegexTotalUptimeSec, string(content), 1)
}

/*
CopyNonEssentialUtilities sets program environment PATH to a comprehensive list of common executable directories on popular OSes.

Then it copies non-essential utility programs (busybox, toybox, etc) from CWD into a temporary directory, the temporary
directory is already among environment PATH.

This function may take couple of seconds to complete. Be aware that certain Linux distributions (e.g. that used by AWS ElasticBeanstalk)
aggresively clears /tmp at regular interval, caller should consider invoking this function at a slow and regular interval.
*/
func CopyNonEssentialUtilities(logger *lalog.Logger) {
	if HostIsWindows() {
		logger.Info("", nil, "will not do anything on Windows")
		return
	}
	logger.Info("", nil, "going to reset program environment PATH and copy non-essential utility programs to "+UtilityDir)
	_ = os.Setenv("PATH", CommonPATH)
	if err := os.MkdirAll(UtilityDir, 0755); err != nil {
		logger.Warning("", err, "failed to create directory %s", UtilityDir)
		return
	}
	srcDestName := []string{
		"busybox-1.31.0-x86_64", "busybox",
		"busybox-x86_64", "busybox",
		"busybox", "busybox",
		"toybox-0.8.6-x86_64", "toybox",
		"toybox-x86_64", "toybox",
		"toybox", "toybox",
	}
	findInPaths := []string{
		// For developing using go module
		"../extra/linux",
		// For developing using GOPATH, assuming that laitos is the only entry underneath GOPATH.
		filepath.Join(os.Getenv("GOPATH"), "/src/github.com/HouzuoGuo/laitos/extra/linux"),
		// For running laitos in directory where config files, data files, and these supplementary programs reside.
		"./",
	}
	for i := 0; i < len(srcDestName); i += 2 {
		srcName := srcDestName[i]
		destName := srcDestName[i+1]
		for _, aPath := range findInPaths {
			srcPath := filepath.Join(aPath, srcName)
			//progress.Info("PrepareUtilities", destName, nil, "looking for %s", srcPath)
			if _, err := os.Stat(srcPath); err != nil {
				//progress.Info("PrepareUtilities", destName, err, "failed to stat srcPath %s", srcPath)
				continue
			}
			from, err := os.Open(srcPath)
			if err != nil {
				//progress.Info("PrepareUtilities", destName, err, "failed to open srcPath %s", srcPath)
				continue
			}
			defer from.Close()
			destPath := filepath.Join(UtilityDir, destName)
			to, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
			if err != nil {
				//progress.Info("PrepareUtilities", destName, err, "failed to open destPath %s ", destPath)
				continue
			}
			defer to.Close()
			if err := os.Chmod(destPath, 0755); err != nil {
				//progress.Info("PrepareUtilities", destName, err, "failed to chmod %s", destPath)
				continue
			}
			if _, err = io.Copy(to, from); err == nil {
				logger.Info(destName, err, "successfully copied from %s to %s", srcPath, destPath)
			}
		}
	}
}

// PowerShellInterpreterPath is the absolute path to PowerShell interpreter executable.
const PowerShellInterpreterPath = `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`

// GetDefaultShellInterpreter returns absolute path to the default system shell interpreter. Returns "" if one cannot be found.
func GetDefaultShellInterpreter() string {
	if HostIsWindows() {
		return PowerShellInterpreterPath
	}
	// Find a Unix-style shell interpreter with a preference to use bash
	for _, shellName := range []string{"bash", "dash", "zsh", "ksh", "ash", "tcsh", "csh", "sh"} {
		for _, pathPrefix := range []string{"/bin", "/usr/bin", "/usr/local/bin", "/opt/bin"} {
			shellPath := filepath.Join(pathPrefix, shellName)
			if _, err := os.Stat(shellPath); err == nil {
				return shellPath
			}
		}
	}
	return ""
}

// GetSysctlStr returns string value of a sysctl parameter corresponding to the input key.
func GetSysctlStr(key string) (string, error) {
	content, err := os.ReadFile(filepath.Join("/proc/sys/", strings.Replace(key, ".", "/", -1)))
	return strings.TrimSpace(string(content)), err
}

// GetSysctlInt return integer value of a sysctl parameter corresponding to the input key.
func GetSysctlInt(key string) (int64, error) {
	val, err := GetSysctlStr(key)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(val, 10, 64)
}

// SetSysctl writes a new value into sysctl parameter.
func SetSysctl(key, value string) (old string, err error) {
	filePath := filepath.Join("/proc/sys/", strings.Replace(key, ".", "/", -1))
	if old, err = GetSysctlStr(key); err != nil {
		return
	}
	err = os.WriteFile(filePath, []byte(strings.TrimSpace(value)), 0644)
	return
}

// IncreaseSysctlInt increases sysctl parameter to the specified value. If value already exceeds, it is left untouched.
func IncreaseSysctlInt(key string, atLeast int64) (old int64, err error) {
	old, err = GetSysctlInt(key)
	if err != nil {
		return
	}
	if old < atLeast {
		_, err = SetSysctl(key, strconv.FormatInt(atLeast, 10))
	}
	return
}

// HostIsCircleCI returns true only if the host environment is on Circle CI.
func HostIsCircleCI() bool {
	return os.Getenv("CIRCLECI") != ""
}

// SkipTestIfCI asks a test to be skipped if it is being run on Circle CI.
func SkipTestIfCI(t testingstub.T) {
	if os.Getenv("CIRCLECI") != "" {
		t.Skip("this test is skipped on CircleCI")
	}
}

// HostIsWSL returns true only if the runtime is Windows subsystem for Linux.
func HostIsWSL() bool {
	cmd := exec.Command("uname", "-a")
	out, err := cmd.CombinedOutput()
	return err == nil && strings.Contains(string(out), "Microsoft")
}

// IsMacOS returns true iff the host OS is running MacOS (darwin).
func IsMacOS() bool {
	return runtime.GOOS == "darwin"
}

// SkipIfWSL asks a test to be skipped if it is being run on Windows Subsystem For Linux.
func SkipIfWSL(t testingstub.T) {
	if HostIsWSL() {
		t.Skip("this test is skipped on Windows Subsystem For Linux")
	}
}

// HostIsWindows returns true only if the runtime is Windows native. It returns false in other cases, including Windows Subsystem For Linux.
func HostIsWindows() bool {
	return runtime.GOOS == "windows"
}

// SkipIfWindows asks a test to be skipped if it is being run on Windows natively (not "subsystem for Linux").
func SkipIfWindows(t testingstub.T) {
	if HostIsWindows() {
		t.Skip("this test is skipped on Windows")
	}
}

/*
GetLocalUserNames returns all user names from /etc/passwd (Unix-like) or local account names (Windows). It returns an
empty map if the names cannot be retrieved.
*/
func GetLocalUserNames() (ret map[string]bool) {
	ret = make(map[string]bool)
	if HostIsWindows() {
		out, err := InvokeProgram(nil, CommonOSCmdTimeoutSec, `C:\Windows\System32\Wbem\WMIC.exe`, "useraccount", "get", "name")
		if err != nil {
			return
		}
		for _, name := range strings.Split(out, "\n") {
			name = strings.TrimSpace(name)
			// Skip trailing empty line and Name header line
			if name == "" || strings.ToLower(name) == "name" {
				continue
			}
			ret[name] = true
		}
	} else {
		passwd, err := os.ReadFile("/etc/passwd")
		if err != nil {
			return
		}
		for _, line := range strings.Split(string(passwd), "\n") {
			idx := strings.IndexRune(line, ':')
			if idx > 0 {
				ret[line[:idx]] = true
			}
		}
	}
	return
}

// BlockUserLogin uses many independent mechanisms to stop a user from logging into system.
func BlockUserLogin(userName string) (ok bool, out string) {
	ok = true
	if HostIsWindows() {
		progOut, err := InvokeProgram(nil, CommonOSCmdTimeoutSec, `C:\Windows\system32\net.exe`, "user", userName, "/active:no")
		if err != nil {
			ok = false
			out += fmt.Sprintf("net user failed: %v - %s\n", err, strings.TrimSpace(progOut))
		}
	} else {
		// Some systems use chsh while some others use chmod
		progOut, err := InvokeProgram(nil, CommonOSCmdTimeoutSec, "chsh", "-s", "/bin/false", userName)
		if err != nil {
			usermodOut, usermodErr := InvokeProgram(nil, CommonOSCmdTimeoutSec, "usermod", "-s", "/bin/false", userName)
			if usermodErr != nil {
				ok = false
				out += fmt.Sprintf("chsh failed (%v - %s) and then usermod shell failed as well: %v - %s\n", err, strings.TrimSpace(progOut), usermodErr, strings.TrimSpace(usermodOut))
			}
		}
		progOut, err = InvokeProgram(nil, CommonOSCmdTimeoutSec, "passwd", "-l", userName)
		if err != nil {
			ok = false
			out += fmt.Sprintf("passwd failed: %v - %s\n", err, strings.TrimSpace(progOut))
		}
		progOut, err = InvokeProgram(nil, CommonOSCmdTimeoutSec, "usermod", "--expiredate", "1", userName)
		if err != nil {
			ok = false
			out += fmt.Sprintf("usermod expiry failed: %v - %s\n", err, strings.TrimSpace(progOut))
		}
	}
	return
}

// DisableStopDaemon disables a system service and prevent it from ever starting again.
func DisableStopDaemon(daemonNameNoSuffix string) (ok bool) {
	if HostIsWindows() {
		// "net stop" conveniently stops dependencies as well
		if out, err := InvokeProgram(nil, CommonOSCmdTimeoutSec, `C:\Windows\system32\net.exe`, "stop", "/yes", daemonNameNoSuffix); err == nil || strings.Contains(strings.ToLower(out), "is not started") {
			ok = true
		}
		/*
			Be aware that, if "sc stop" responds with:
			"The specified service does not exist as an installed service."
			The response is actually saying there is denied access and it cannot determine the state of the service.
		*/
		if out, err := InvokeProgram(nil, CommonOSCmdTimeoutSec, `C:\Windows\system32\sc.exe`, "stop", daemonNameNoSuffix); err == nil || strings.Contains(strings.ToLower(out), "has not been started") {
			ok = true
		}
		if _, err := InvokeProgram(nil, CommonOSCmdTimeoutSec, `C:\Windows\system32\sc.exe`, "config", daemonNameNoSuffix, "start=", "disabled"); err == nil {
			ok = true
		}
	} else {
		// Some hosting providers still have not used systemd yet, such as the OS on Elastic Beanstalk.
		_, _ = InvokeProgram(nil, CommonOSCmdTimeoutSec, "/etc/init.d/"+daemonNameNoSuffix, "stop")
		if _, err := InvokeProgram(nil, CommonOSCmdTimeoutSec, "chkconfig", " --level", "0123456", daemonNameNoSuffix, "off"); err == nil {
			ok = true
		}
		if _, err := InvokeProgram(nil, CommonOSCmdTimeoutSec, "chmod", "0000", "/etc/init.d/"+daemonNameNoSuffix); err == nil {
			ok = true
		}
		_, _ = InvokeProgram(nil, CommonOSCmdTimeoutSec, "systemctl", "stop", daemonNameNoSuffix+".service")
		if _, err := InvokeProgram(nil, CommonOSCmdTimeoutSec, "systemctl", "disable", daemonNameNoSuffix+".service"); err == nil {
			ok = true
		}
		if _, err := InvokeProgram(nil, CommonOSCmdTimeoutSec, "systemctl", "mask", daemonNameNoSuffix+".service"); err == nil {
			ok = true
		}
	}
	return
}

// EnableStartDaemon enables and starts a system service.
func EnableStartDaemon(daemonNameNoSuffix string) (ok bool) {
	if HostIsWindows() {
		if _, err := InvokeProgram(nil, CommonOSCmdTimeoutSec, `C:\Windows\system32\sc.exe`, "config", daemonNameNoSuffix, "start=", "auto"); err == nil {
			ok = true
		}
		if out, err := InvokeProgram(nil, CommonOSCmdTimeoutSec, `C:\Windows\system32\sc.exe`, "start", daemonNameNoSuffix); err == nil || strings.Contains(strings.ToLower(out), "already running") {
			ok = true
		}
	} else {
		// Some hosting providers still have not used systemd yet, such as the OS on Elastic Beanstalk.
		_, _ = InvokeProgram(nil, CommonOSCmdTimeoutSec, "chmod", "0755", "/etc/init.d/"+daemonNameNoSuffix)
		if _, err := InvokeProgram(nil, CommonOSCmdTimeoutSec, "chkconfig", " --level", "345", daemonNameNoSuffix, "on"); err == nil {
			ok = true
		}
		if _, err := InvokeProgram(nil, CommonOSCmdTimeoutSec, "/etc/init.d/"+daemonNameNoSuffix, "start"); err == nil {
			ok = true
		}
		_, _ = InvokeProgram(nil, CommonOSCmdTimeoutSec, "systemctl", "unmask", daemonNameNoSuffix+".service")
		if _, err := InvokeProgram(nil, CommonOSCmdTimeoutSec, "systemctl", "enable", daemonNameNoSuffix+".service"); err == nil {
			ok = true
		}
		if _, err := InvokeProgram(nil, CommonOSCmdTimeoutSec, "systemctl", "start", daemonNameNoSuffix+".service"); err == nil {
			ok = true
		}
	}
	return
}

/*
DisableInterferingResolved disables systemd-resolved service to prevent it from interfering with laitos DNS server daemon.
Otherwise, systemd-resolved daemon listens on 127.0.0.53:53 and prevents laitos DNS server from listening on all network interfaces (0.0.0.0).
*/
func DisableInterferingResolved() (out string) {
	if _, err := InvokeProgram(nil, CommonOSCmdTimeoutSec, "systemctl", "is-active", "systemd-resolved"); err != nil {
		return "will not change name resolution settings as systemd-resolved is not active"
	}
	// Read the configuration file, it may have already been overwritten by systemd-resolved.
	originalContent, err := os.ReadFile("/etc/resolv.conf")
	var hasUplinkNameServer bool
	if err == nil {
		for _, line := range strings.Split(string(originalContent), "\n") {
			if regexp.MustCompile(`^\s*nameserver.+$`).MatchString(line) && !regexp.MustCompile(`^\s*nameserver\s+127\..*$`).MatchString(line) {
				hasUplinkNameServer = true
				break
			}
		}
	}
	// Stop systemd-resolved but do not disable it, it still helps to collect uplink DNS server configuration next boot.
	_, err = InvokeProgram(nil, CommonOSCmdTimeoutSec, "systemctl", "stop", "systemd-resolved.service")
	if err != nil {
		out += "failed to stop systemd-resolved.service\n"
	}
	// Distributions that use systemd-resolved usually makes resolv.conf a symbol link to an automatically generated file
	os.RemoveAll("/etc/resolv.conf")
	var newContent string
	if hasUplinkNameServer {
		// The configuration created by systemd-resolved connects directly to uplink DNS servers (e.g. LAN), hence retaining the configuration.
		out += "retaining uplink DNS server configuration\n"
		newContent = string(originalContent)
	} else {
		/*
			Create a new resolv.conf consisting of primary servers of popular public DNS resolvers.
			glibc cannot use more than three DNS resolvers.
		*/
		out += "using public DNS servers\n"
		newContent = `
# Generated by laitos software - DisableInterferingResolved
options rotate timeout:3 attempts:3
# Quad9, OpenDNS, CloudFlare primary
nameserver 9.9.9.9
nameserver 208.67.222.222
nameserver 1.1.1.2
`
	}
	if err := os.WriteFile("/etc/resolv.conf", []byte(newContent), 0644); err == nil {
		out += "resolv.conf has been reset\n"
	} else {
		out += fmt.Sprintf("failed to overwrite resolv.conf - %v\n", err)
	}
	return
}

// SwapOff turns off all swap files and partitions for improved system confidentiality.
func SwapOff() error {
	// Wait quite a while to ensure that caller gets an accurate result return value.
	out, err := InvokeProgram(nil, CommonOSCmdTimeoutSec, "swapoff", "-a")
	if err != nil {
		return fmt.Errorf("SwapOff: %v - %s", err, out)
	}
	return nil
}

// SetTimeZone changes system time zone to the specified value, such as "UTC" or
// "Europe/Dublin".
func SetTimeZone(zone string) error {
	if IsMacOS() {
		// Using this approach to set time zone on MacOS results in the time
		// zone to be reset to UTC instead - regardless of the value specified,
		// and the Date/Time page of System Preferences will indicate that no
		// time zone is set at all. This bug is observed on MacOS 11.
		return errors.New("will not set time zone on MacOS due to an OS bug")
	}
	zoneInfoPath := filepath.Join("/usr/share/zoneinfo/", zone)
	if stat, err := os.Stat(zoneInfoPath); err != nil || stat.IsDir() {
		return fmt.Errorf("failed to read zoneinfo file of %s - %v", zone, err)
	}
	os.Remove("/etc/localtime")
	if err := os.Symlink(zoneInfoPath, "/etc/localtime"); err != nil {
		return fmt.Errorf("failed to make localtime symlink: %v", err)
	}
	return nil
}

// GetRedactedEnviron returns the program's environment varibles in "Key=Value" string array similar to those returned
// by os.Environ. Sensitive environment variables that amy reveal API secrets or passwords will be present, though their
// values will be string "REDACTED".
func GetRedactedEnviron() []string {
	environ := os.Environ()
	ret := make([]string, 0, len(environ))
	for _, keyValue := range environ {
		components := strings.SplitN(keyValue, "=", 2)
		if len(components) < 2 {
			continue
		}
		envKey := components[0]
		var redacted bool
		for _, needle := range []string{"access", "cred", "key", "pass", "secret", "token", misc.EnvironmentDecryptionPassword} {
			if strings.Contains(strings.ToLower(envKey), needle) {
				ret = append(ret, envKey+"=REDACTED")
				redacted = true
				break
			}
		}
		if !redacted {
			ret = append(ret, keyValue)
		}
	}
	return ret
}

// GetProgramStatusSummary returns a formatted human-readable text that describes key OS resource usage status and program environment.
func GetProgramStatusSummary(withPublicIP bool) ProgramStatusSummary {
	// System resource usage
	usedMem, totalMem := GetSystemMemoryUsageKB()
	usedRoot, freeRoot, totalRoot := GetRootDiskUsageKB()
	// Network info
	hostName, _ := os.Hostname()
	// Program environment and runtime info
	exeAbsPath, _ := os.Executable()
	workingDir, _ := os.Getwd()
	dirEntries, _ := os.ReadDir(workingDir)
	dirEntryNames := make([]string, 0)
	for i, entry := range dirEntries {
		if i > 100 {
			break
		}
		if entry.IsDir() {
			dirEntryNames = append(dirEntryNames, entry.Name()+"/")
		} else {
			dirEntryNames = append(dirEntryNames, entry.Name())
		}
	}
	envVars := GetRedactedEnviron()
	if len(envVars) > 100 {
		envVars = envVars[:100]
	}

	summary := ProgramStatusSummary{
		HostName:          hostName,
		ClockTime:         time.Now(),
		SysUptime:         time.Duration(GetSystemUptimeSec()) * time.Second,
		ProgramUptime:     time.Since(misc.StartupTime),
		SysTotalMemMB:     totalMem / 1024,
		SysUsedMemMB:      usedMem / 1024,
		ProgUsedMemMB:     GetProgramMemoryUsageKB() / 1024,
		SysLoad:           GetSystemLoad(),
		DiskUsedMB:        usedRoot / 1024,
		DiskFreeMB:        freeRoot / 1024,
		DiskCapMB:         totalRoot / 1024,
		NumCPU:            runtime.NumCPU(),
		NumGoMaxProcs:     runtime.GOMAXPROCS(0),
		NumGoroutines:     runtime.NumGoroutine(),
		PID:               os.Getpid(),
		PPID:              os.Getppid(),
		UID:               os.Getuid(),
		EUID:              os.Geteuid(),
		GID:               os.Getgid(),
		EGID:              os.Getegid(),
		ExePath:           exeAbsPath,
		CLIFlags:          os.Args[1:],
		WorkingDirPath:    workingDir,
		WorkingDirContent: dirEntryNames,
		EnvironmentVars:   envVars,
	}
	if withPublicIP {
		summary.PublicIP = inet.GetPublicIP().String()
	}
	return summary
}
