package misc

import (
	"bytes"
	"errors"
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
	totalKB = int(int64(fs.Blocks) * fs.Bsize / 1024)
	freeKB = int(int64(fs.Bfree) * fs.Bsize / 1024)
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
		"busybox-1.26.2-x86_64", "busybox",
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
			if _, err := os.Stat(srcPath); err != nil {
				//progress.Info("PrepareUtilities", destName, nil, "failed to stat srcPath", srcPath)
				continue
			}
			from, err := os.Open(srcPath)
			if err != nil {
				//progress.Info("PrepareUtilities", destName, nil, "failed to open srcPath", srcPath)
				continue
			}
			defer from.Close()
			destPath := path.Join(UtilityDir, destName)
			to, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
			if err != nil {
				//progress.Info("PrepareUtilities", destName, nil, "failed to open destPath", destPath)
				continue
			}
			defer to.Close()
			if err := os.Chmod(destPath, 0755); err != nil {
				continue
			}
			if _, err = io.Copy(to, from); err == nil {
				progress.Info("PrepareUtilities", destName, nil, "successfully copied from %s to %s", srcPath, destPath)
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
		// Instead of using Kill() function that only kills one process, use syscall to kill the entire process group.
		if killErr := syscall.Kill(-proc.Process.Pid, syscall.SIGKILL); killErr != nil {
			logger.Warning("InvokeProgram", program, killErr, "failed to kill after time limit exceeded")
		}
		timedOut = true
	})
	err = proc.Run()
	timeOutTimer.Stop()

	if timedOut {
		err = errors.New("time limit exceeded")
	}
	out = outBuf.String()
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
