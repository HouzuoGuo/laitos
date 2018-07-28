package misc

import (
	"fmt"
	"github.com/HouzuoGuo/laitos/testingstub"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

var RegexVmRss = regexp.MustCompile(`VmRSS:\s*(\d+)\s*kB`)               // Parse VmRss value from /proc/*/status line
var RegexMemAvailable = regexp.MustCompile(`MemAvailable:\s*(\d+)\s*kB`) // Parse MemAvailable value from /proc/meminfo
var RegexMemTotal = regexp.MustCompile(`MemTotal:\s*(\d+)\s*kB`)         // Parse MemTotal value from /proc/meminfo
var RegexMemFree = regexp.MustCompile(`MemFree:\s*(\d+)\s*kB`)           // Parse MemFree value from /proc/meminfo
var RegexTotalUptimeSec = regexp.MustCompile(`(\d+).*`)                  // Parse uptime seconds from /proc/meminfo

// CommonOSCmdTimeoutSec is the number of seconds to tolerate for running a wide range of system management utilities.
const CommonOSCmdTimeoutSec = 30

// Use regex to parse input string, and return an integer parsed from specified capture group, or 0 if there is no match/no integer.
func FindNumInRegexGroup(numRegex *regexp.Regexp, input string, groupNum int) int {
	match := numRegex.FindStringSubmatch(input)
	if match == nil || len(match) <= groupNum {
		return 0
	}
	val, err := strconv.Atoi(match[groupNum])
	if err == nil {
		return val
	}
	return 0
}

// Return RSS memory usage of this process. Return 0 if the memory usage cannot be determined.
func GetProgramMemoryUsageKB() int {
	statusContent, err := ioutil.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	return FindNumInRegexGroup(RegexVmRss, string(statusContent), 1)
}

// Return operating system memory usage. Return 0 if the memory usage cannot be determined.
func GetSystemMemoryUsageKB() (usedKB int, totalKB int) {
	infoContent, err := ioutil.ReadFile("/proc/meminfo")
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
	content, err := ioutil.ReadFile("/proc/loadavg")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}

// Get system uptime in seconds. Return 0 if it cannot be determined.
func GetSystemUptimeSec() int {
	content, err := ioutil.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	return FindNumInRegexGroup(RegexTotalUptimeSec, string(content), 1)
}

/*
UtilityDir is an element of PATH that points to a directory where laitos bundled utility programs are stored. The
utility programs are not essential to most of laitos operations, however they come in handy in certain scenarios:
- statically linked "busybox" (maintenance daemon uses it to synchronise system clock)
- statically linked "toybox" (its rich set of utilities help with shell usage)
- dynamically linked "phantomjs" (used by text interactive web browser feature and browser-in-browser HTTP handler)
*/
const UtilityDir = "/tmp/laitos-util"

/*
CommonPATH is a PATH environment variable value that includes most common executable locations across Unix and Linux.
Be aware that, when laitos launches external programs they usually should inherit all of the environment variables from
parent process, which may include PATH. However, as an exception, AWS ElasticBeanstalk launches programs via a
"supervisord" that resets PATH variable to deliberately exclude sbin directories, therefore, it is often useful to use
this hard coded PATH value to launch programs.
*/
const CommonPATH = UtilityDir + ":/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin:/usr/local/sbin:/opt/bin:/opt/sbin"

/*
PrepareUtilities resets program environment PATH to be a comprehensive list of common executable locations, then
it copies non-essential laitos utility programs to a designated directory.

This is a rather expensive function due to involvement of heavy file IO, and be aware that the OS template on AWS
ElasticBeanstalk aggressively clears /tmp at regular interval, therefore caller may want to to invoke this function at
regular interval.
*/
func PrepareUtilities(progress Logger) {
	if HostIsWindows() {
		progress.Info("PrepareUtilities", "", nil, "will not do anything on Windows")
		return
	}
	progress.Info("PrepareUtilities", "", nil, "going to reset program environment PATH and copy non-essential utility programs to "+UtilityDir)
	os.Setenv("PATH", CommonPATH)
	if err := os.MkdirAll(UtilityDir, 0755); err != nil {
		progress.Warning("PrepareUtilities", "", err, "failed to create directory %s", UtilityDir)
		return
	}
	srcDestName := []string{
		"busybox-1.28.1-x86_64", "busybox",
		"busybox-x86_64", "busybox",
		"busybox", "busybox",
		"toybox-0.7.7-x86_64", "toybox",
		"toybox-x86_64", "toybox",
		"toybox", "toybox",
		"phantomjs-2.1.1-x86_64", "phantomjs",
		"phantomjs", "phantomjs",
	}
	// The GOPATH directory is useful for developing test cases, and CWD is useful for running deployed laitos.
	findInPaths := []string{filepath.Join(os.Getenv("GOPATH"), "/src/github.com/HouzuoGuo/laitos/extra/linux"), "./"}
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
				progress.Info("PrepareUtilities", destName, err, "successfully copied from %s to %s", srcPath, destPath)
			}
		}
	}
}

/*
InvokeShell launches an external shell process with time constraints to run a piece of shell code. The code is fed into
shell command parameter "-c", which happens to be universally accepted by Unix shells and Windows Powershell.
Returns shell stdout+stderr output combined and error if there is any.
*/
func InvokeShell(timeoutSec int, interpreter string, content string) (out string, err error) {
	return InvokeProgram(nil, timeoutSec, interpreter, "-c", content)
}

// GetSysctlStr returns string value of a sysctl parameter corresponding to the input key.
func GetSysctlStr(key string) (string, error) {
	content, err := ioutil.ReadFile(filepath.Join("/proc/sys/", strings.Replace(key, ".", "/", -1)))
	return strings.TrimSpace(string(content)), err
}

// GetSysctlInt return integer value of a sysctl parameter corresponding to the input key.
func GetSysctlInt(key string) (int, error) {
	val, err := GetSysctlStr(key)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(val)
}

// SetSysctl writes a new value into sysctl parameter.
func SetSysctl(key, value string) (old string, err error) {
	filePath := filepath.Join("/proc/sys/", strings.Replace(key, ".", "/", -1))
	if old, err = GetSysctlStr(key); err != nil {
		return
	}
	err = ioutil.WriteFile(filePath, []byte(strings.TrimSpace(value)), 0644)
	return
}

// IncreaseSysctlInt increases sysctl parameter to the specified value. If value already exceeds, it is left untouched.
func IncreaseSysctlInt(key string, atLeast int) (old int, err error) {
	old, err = GetSysctlInt(key)
	if err != nil {
		return
	}
	if old < atLeast {
		_, err = SetSysctl(key, strconv.Itoa(atLeast))
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

// SkipIfWSL asks a test to be skipped if it is being run on Windows Subsystem For Linux.
func SkipIfWSL(t testingstub.T) {
	if HostIsWSL() {
		t.Skip("this test is skipped on Windows Subsystem For Linux")
	}
}

// HostIsWindows returns true only if the runtime is Windows native, not subsystem for Linux.
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
		passwd, err := ioutil.ReadFile("/etc/passwd")
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
		progOut, err := InvokeProgram(nil, CommonOSCmdTimeoutSec, "chsh", "-s", "/bin/false", userName)
		if err != nil {
			ok = false
			out += fmt.Sprintf("chsh failed: %v - %s\n", err, strings.TrimSpace(progOut))
		}
		progOut, err = InvokeProgram(nil, CommonOSCmdTimeoutSec, "passwd", "-l", userName)
		if err != nil {
			ok = false
			out += fmt.Sprintf("passwd failed: %v - %s\n", err, strings.TrimSpace(progOut))
		}
		progOut, err = InvokeProgram(nil, CommonOSCmdTimeoutSec, "usermod", "--expiredate", "1", userName)
		if err != nil {
			ok = false
			out += fmt.Sprintf("usermod failed: %v - %s\n", err, strings.TrimSpace(progOut))
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
		InvokeProgram(nil, CommonOSCmdTimeoutSec, "/etc/init.d/"+daemonNameNoSuffix, "stop")
		if _, err := InvokeProgram(nil, CommonOSCmdTimeoutSec, "chkconfig", " --level", "0123456", daemonNameNoSuffix, "off"); err == nil {
			ok = true
		}
		if _, err := InvokeProgram(nil, CommonOSCmdTimeoutSec, "chmod", "0000", "/etc/init.d/"+daemonNameNoSuffix); err == nil {
			ok = true
		}
		InvokeProgram(nil, CommonOSCmdTimeoutSec, "systemctl", "stop", daemonNameNoSuffix+".service")
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
		InvokeProgram(nil, CommonOSCmdTimeoutSec, "chmod", "0755", "/etc/init.d/"+daemonNameNoSuffix)
		if _, err := InvokeProgram(nil, CommonOSCmdTimeoutSec, "chkconfig", " --level", "345", daemonNameNoSuffix, "on"); err == nil {
			ok = true
		}
		if _, err := InvokeProgram(nil, CommonOSCmdTimeoutSec, "/etc/init.d/"+daemonNameNoSuffix, "start"); err == nil {
			ok = true
		}
		InvokeProgram(nil, CommonOSCmdTimeoutSec, "systemctl", "unmask", daemonNameNoSuffix+".service")
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
DisableInterferingResolvd prevents systemd-resolved from interfering with laitos DNS daemon. Due to a systemd quirk,
a running resolved prevents DNS from listening on all interfaces.
*/
func DisableInterferingResolved() (relevant bool, out string) {
	if _, err := InvokeProgram(nil, CommonOSCmdTimeoutSec, "systemctl", "is-active", "systemd-resolved"); err != nil {
		return false, "will not touch name resolution settings as resolved is not active"
	}
	relevant = true
	// Completely disable systemd-resolved
	if DisableStopDaemon("systemd-resolved") {
		out += "systemd-resolved is disabled\n"
	} else {
		out += "failed to disable systemd-resolved\n"
	}
	// Distributions that use systemd-resolved usually makes resolv.conf a symbol link to an automatically generated file
	os.RemoveAll("/etc/resolv.conf")
	// Overwrite its content with a sane set of public DNS servers (2 x Quad9, 2 x SafeDNS, 2 x Comodo SecureDNS)
	newContent := `
options rotate timeout:3 attempts:3
nameserver 9.9.9.9
nameserver 149.112.112.112
nameserver 195.46.39.39
nameserver 195.46.39.40
nameserver 208.67.222.222
nameserver 208.67.220.220
`
	if err := ioutil.WriteFile("/etc/resolv.conf", []byte(newContent), 0644); err == nil {
		out += "resolv.conf is reset\n"
	} else {
		out += fmt.Sprintf("failed to write into resolv.conf - %v\n", err)
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

// SetTimeZone changes system time zone to the specified value (such as "UTC").
func SetTimeZone(zone string) error {
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
