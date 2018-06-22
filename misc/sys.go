package misc

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/testingstub"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var RegexVmRss = regexp.MustCompile(`VmRSS:\s*(\d+)\s*kB`)               // Parse VmRss value from /proc/*/status line
var RegexMemAvailable = regexp.MustCompile(`MemAvailable:\s*(\d+)\s*kB`) // Parse MemAvailable value from /proc/meminfo
var RegexMemTotal = regexp.MustCompile(`MemTotal:\s*(\d+)\s*kB`)         // Parse MemTotal value from /proc/meminfo
var RegexMemFree = regexp.MustCompile(`MemFree:\s*(\d+)\s*kB`)           // Parse MemFree value from /proc/meminfo
var RegexTotalUptimeSec = regexp.MustCompile(`(\d+).*`)                  // Parse uptime seconds from /proc/meminfo

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

// GetRootDiskUsageKB returns used and total space of the file system mounted on /. Returns 0 if they cannot be determined.
func GetRootDiskUsageKB() (usedKB, freeKB, totalKB int) {
	fs := syscall.Statfs_t{}
	err := syscall.Statfs("/", &fs)
	if err != nil {
		return
	}
	totalKB = int(int64(fs.Blocks) * int64(fs.Bsize) / 1024)
	freeKB = int(int64(fs.Bfree) * int64(fs.Bsize) / 1024)
	usedKB = totalKB - freeKB
	return
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
	logger.Info("PrepareUtilities", "", nil, "going to reset program environment PATH and copy non-essential utility programs to "+UtilityDir)
	os.Setenv("PATH", CommonPATH)
	if err := os.MkdirAll(UtilityDir, 0755); err != nil {
		progress.Warning("PrepareUtilities", "", err, "failed to create directory %s", UtilityDir)
		return
	}
	srcDestName := []string{
		"busybox-1.28.1-x86_64", "busybox",
		"busybox-x86_64", "busybox",
		"busybox", "busybox",
		"toybox-0.7.5-x86_64", "toybox",
		"toybox-x86_64", "toybox",
		"toybox", "toybox",
		"phantomjs-2.1.1-x86_64", "phantomjs",
		"phantomjs", "phantomjs",
	}
	// The GOPATH directory is useful for developing test cases, and CWD is useful for running deployed laitos.
	findInPaths := []string{path.Join(os.Getenv("GOPATH"), "/src/github.com/HouzuoGuo/laitos/extra/"), "./"}
	for i := 0; i < len(srcDestName); i += 2 {
		srcName := srcDestName[i]
		destName := srcDestName[i+1]
		for _, aPath := range findInPaths {
			srcPath := path.Join(aPath, srcName)
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
			destPath := path.Join(UtilityDir, destName)
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
InvokeProgram launches an external program with time constraints. The external program inherits laitos' environment
mixed with additional input environment variables. The additional variables take precedence over inherited ones.
Returns stdout+stderr output combined, and error if there is any.
*/
func InvokeProgram(envVars []string, timeoutSec int, program string, args ...string) (out string, err error) {
	if timeoutSec < 1 {
		return "", errors.New("invalid time limit")
	}
	// Make an environment variable array of common PATH, inherited values, and newly specified values.
	myEnv := os.Environ()
	combinedEnv := make([]string, 0, 1+len(myEnv))
	// Inherit environment variables from program environment
	combinedEnv = append(combinedEnv, myEnv...)
	/*
		Put common PATH values into the mix. Since go 1.9, when environment variables contain duplicated keys, only the
		last value of duplicated key is effective. This behaviour enables caller to override PATH if deemede necessary.
	*/
	combinedEnv = append(combinedEnv, "PATH="+CommonPATH)
	if envVars != nil {
		combinedEnv = append(combinedEnv, envVars...)
	}
	// Collect stdout and stderr all together in a single buffer
	var outBuf bytes.Buffer
	proc := exec.Command(program, args...)
	proc.Env = combinedEnv
	proc.Stdout = &outBuf
	proc.Stderr = &outBuf
	// Use process group so that child processes are also killed upon time out
	proc.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Monitor for time out
	var timedOut bool
	timeOutTimer := time.AfterFunc(time.Duration(timeoutSec)*time.Second, func() {
		timedOut = true
		if !KillProcess(proc.Process) {
			logger.Warning("InvokeProgram", program, nil, "failed to kill after time limit exceeded")
		}
	})
	err = proc.Run()
	timeOutTimer.Stop()
	if timedOut {
		err = errors.New("time limit exceeded")
	}
	out = outBuf.String()
	return
}

// KillProcess kills the process or the group of processes associated with it.
func KillProcess(proc *os.Process) (success bool) {
	// Kill process group if it is one
	if killErr := syscall.Kill(-proc.Pid, syscall.SIGKILL); killErr == nil {
		success = true
	}
	if killErr := syscall.Kill(proc.Pid, syscall.SIGKILL); killErr == nil {
		success = true
	}
	if proc.Kill() == nil {
		success = true
	}
	proc.Wait()
	return
}

/*
InvokeShell launches an external shell process with time constraints to run a piece of code.
Returns shell stdout+stderr output combined and error if there is any.
*/
func InvokeShell(timeoutSec int, interpreter string, content string) (out string, err error) {
	return InvokeProgram(nil, timeoutSec, interpreter, "-c", content)
}

// GetSysctlStr returns string value of a sysctl parameter corresponding to the input key.
func GetSysctlStr(key string) (string, error) {
	content, err := ioutil.ReadFile(path.Join("/proc/sys/", strings.Replace(key, ".", "/", -1)))
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
	filePath := path.Join("/proc/sys/", strings.Replace(key, ".", "/", -1))
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

// SkipTestIfCI asks a test to be skipped if it is being run on Circle CI.
func SkipTestIfCI(t testingstub.T) {
	if os.Getenv("CIRCLECI") != "" {
		t.Skip("this test is skipped on CircleCI")
	}
}

// SkipIfWSL asks a test to be skipped if it is being run on Windows Subsystem For Linux.
func SkipIfWSL(t testingstub.T) {
	cmd := exec.Command("uname", "-a")
	out, err := cmd.CombinedOutput()
	if err == nil && strings.Contains(string(out), "Microsoft") {
		t.Skip("this test is skipped on Windows Subsystem For Linux")
	}
}

// GetLocalUserNames returns all user names from /etc/passwd, or an empty map if they cannot be read.
func GetLocalUserNames() (ret map[string]bool) {
	ret = make(map[string]bool)
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
	return
}

// BlockUserLogin uses three separate mechanisms to stop a user from logging into the system.
func BlockUserLogin(userName string) (ok bool, out string) {
	ok = true
	progOut, err := InvokeProgram(nil, 3, "chsh", "-s", "/bin/false", userName)
	if err != nil {
		ok = false
		out += fmt.Sprintf("chsh failed: %v - %s\n", err, strings.TrimSpace(progOut))
	}
	progOut, err = InvokeProgram(nil, 3, "passwd", "-l", userName)
	if err != nil {
		ok = false
		out += fmt.Sprintf("passwd failed: %v - %s\n", err, strings.TrimSpace(progOut))
	}
	progOut, err = InvokeProgram(nil, 3, "usermod", "--expiredate", "1", userName)
	if err != nil {
		ok = false
		out += fmt.Sprintf("usermod failed: %v - %s\n", err, strings.TrimSpace(progOut))
	}
	return
}

// DisableStopDaemon disables a system daemon program and prevent it from ever starting again.
func DisableStopDaemon(daemonNameNoSuffix string) (ok bool) {
	// Some hosting providers still have not used systemd yet, such as the OS on Elastic Beanstalk.
	InvokeProgram(nil, 5, "/etc/init.d/"+daemonNameNoSuffix, "stop")
	if _, err := InvokeProgram(nil, 5, "chkconfig", " --level", "0123456", daemonNameNoSuffix, "off"); err == nil {
		ok = true
	}
	if _, err := InvokeProgram(nil, 5, "chmod", "0000", "/etc/init.d/"+daemonNameNoSuffix); err == nil {
		ok = true
	}
	InvokeProgram(nil, 5, "systemctl", "stop", daemonNameNoSuffix+".service")
	if _, err := InvokeProgram(nil, 5, "systemctl", "disable", daemonNameNoSuffix+".service"); err == nil {
		ok = true
	}
	if _, err := InvokeProgram(nil, 5, "systemctl", "mask", daemonNameNoSuffix+".service"); err == nil {
		ok = true
	}
	return
}

// EnableStartDaemon enables and starts a system program.
func EnableStartDaemon(daemonNameNoSuffix string) (ok bool) {
	// Some hosting providers still have not used systemd yet, such as the OS on Elastic Beanstalk.
	InvokeProgram(nil, 5, "chmod", "0755", "/etc/init.d/"+daemonNameNoSuffix)
	if _, err := InvokeProgram(nil, 5, "chkconfig", " --level", "345", daemonNameNoSuffix, "on"); err == nil {
		ok = true
	}
	if _, err := InvokeProgram(nil, 5, "/etc/init.d/"+daemonNameNoSuffix, "start"); err == nil {
		ok = true
	}
	InvokeProgram(nil, 5, "systemctl", "unmask", daemonNameNoSuffix+".service")
	if _, err := InvokeProgram(nil, 5, "systemctl", "enable", daemonNameNoSuffix+".service"); err == nil {
		ok = true
	}
	if _, err := InvokeProgram(nil, 5, "systemctl", "start", daemonNameNoSuffix+".service"); err == nil {
		ok = true
	}
	return
}

/*
DisableInterferingResolvd prevents systemd-resolved from interfering with laitos DNS daemon. Due to a systemd quirk,
a running resolved prevents DNS from listening on all interfaces.
*/
func DisableInterferingResolved() (relevant bool, out string) {
	if _, err := InvokeProgram(nil, 5, "systemctl", "is-active", "systemd-resolved"); err != nil {
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
nameserver 8.26.56.26
nameserver 8.20.247.20
`
	if err := ioutil.WriteFile("/etc/resolv.conf", []byte(newContent), 0644); err == nil {
		out += "resolv.conf is reset\n"
	} else {
		out += fmt.Sprintf("failed to write into resolv.conf - %v\n", err)
	}
	return
}

// SwapOff turns off all swap files and partitions for improved system security.
func SwapOff() error {
	out, err := InvokeProgram(nil, 60, "swapoff", "-a")
	if err != nil {
		return fmt.Errorf("SwapOff: %v - %s", err, out)
	}
	return nil
}

// LockMemory locks program memory to prevent swapping, protecting sensitive user data.
func LockMemory() {
	// Lock all program memory into main memory to prevent sensitive data from leaking into swap.
	if os.Geteuid() == 0 {
		if err := syscall.Mlockall(syscall.MCL_CURRENT | syscall.MCL_FUTURE); err != nil {
			logger.Warning("LockMemory", "", err, "failed to lock memory")
			return
		}
		logger.Warning("LockMemory", "", nil, "program has been locked into memory for safety reasons")
	} else {
		logger.Warning("LockMemory", "", nil, "program is not running as root (UID 0) hence memory cannot be locked, your private information will leak into swap.")
	}
}

// SetTimeZone changes system time zone to the specified value (such as "UTC").
func SetTimeZone(zone string) error {
	zoneInfoPath := path.Join("/usr/share/zoneinfo/", zone)
	if stat, err := os.Stat(zoneInfoPath); err != nil || stat.IsDir() {
		return fmt.Errorf("failed to read zoneinfo file of %s - %v", zone, err)
	}
	os.Remove("/etc/localtime")
	if err := os.Symlink(zoneInfoPath, "/etc/localtime"); err != nil {
		return fmt.Errorf("failed to make localtime symlink: %v", err)
	}
	return nil
}
