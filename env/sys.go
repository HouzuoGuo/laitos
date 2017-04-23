package env

import (
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"
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
