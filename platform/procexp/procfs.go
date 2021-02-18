package procexp

import (
	"encoding/binary"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

const (
	// SizeOfUint is the size of an unsigned integer - either 32 or 64.
	SizeOfUint uint = 32 << (^uint(0) >> 63)
)

var (
	regexColonKeyValue   = regexp.MustCompile(`^\s*(\w+)\s*:\s*(.*)`)
	regexSchedstatFields = regexp.MustCompile(`^\s*(\d+)\s+(\d+)\s+(\d+).*`)
	// PID, executable base name (up to 16 characters long), state, and 35 more fields.
	// See https://man7.org/linux/man-pages/man5/procfs.5.html for the complete list of fields.
	regexStatFields = regexp.MustCompile(`^\s*(\d+)\s+\((.*)\)\s+(\S+)\s+` + strings.Repeat(`(\S+)\s+`, 35) + `.*`)

	// sysconfClockTick is the cached value of the number of times kernel timer interrupts each second,
	// the value is going to be calculated by function getClockTicksPerSecondOnce.
	sysconfClockTick           int = 0
	getClockTicksPerSecondOnce     = new(sync.Once)
)

// SchedulerStats describes the resource usage of a process task from scheduler's perspective.
type SchedulerStats struct {
	// From schedstat
	NumRunSec  float64
	NumWaitSec float64
	// From sched
	NumVoluntarySwitches   int
	NumInvoluntarySwitches int
}

// ProcessStats describes overall resource usage of a process across all of its tasks.
type ProcessStats struct {
	State                        string
	StartedAtUptimeSec           int
	VirtualMemSizeBytes          int
	ResidentSetMemSizeBytes      int
	NumUserModeSecInclChildren   float64
	NumKernelModeSecInclChildren float64
}

// ProcessStatus describes the identity and privileges of a process or a process task.
type ProcessStatus struct {
	Name             string
	Umask            string
	ParentPID        int
	ThreadGroupID    int
	ProcessID        int
	ProcessGroupID   int
	SessionID        int
	ProcessPrivilege ProcessPrivilege
}

// ProcessPrivilege describes the the UID and GID under which a process runs.
type ProcessPrivilege struct {
	RealUID      int
	EffectiveUID int
	RealGID      int
	EffectiveGID int
}

// ProcessMemUsage describes the memory usage of a process.
type ProcessMemUsage struct {
	VirtualMemSizeBytes     int
	ResidentSetMemSizeBytes int
}

// ProcessCPUUsage describes the accumulated CPU usage of a process.
type ProcessCPUUsage struct {
	NumUserModeSecInclChildren float64
	NumSysModeSecInclChildren  float64
}

// atoiOr0 returns the integer converted from the input string, or 0 if the input string does not represent a valid integer.
func atoiOr0(str string) int {
	ret, _ := strconv.Atoi(str)
	return ret
}

// strSliceElemOrEmpty retrieves the string element at index I from the input slice, or "" if the slice does not contain an index I.
func strSliceElemOrEmpty(slice []string, i int) string {
	if len(slice) > i {
		return slice[i]
	}
	return ""
}

// getDACIDsFromProcfs returns the real, effective, and saved UID/GID from the input space-separated string fields.
func getDACIDsFromProcfs(in string) []int {
	ids := regexp.MustCompile(`\s+`).Split(in, -1)
	return []int{
		atoiOr0(strSliceElemOrEmpty(ids, 0)),
		atoiOr0(strSliceElemOrEmpty(ids, 1)),
		atoiOr0(strSliceElemOrEmpty(ids, 2)),
	}
}

// getClockTicksPerSecond returns the number of times kernel timer interrupts every second.
func getClockTicksPerSecond() int {
	getClockTicksPerSecondOnce.Do(func() {
		// The function body is heavily inspired by github.com/tklauser/go-sysconf
		auxv, err := ioutil.ReadFile("/proc/self/auxv")
		if err == nil {
			bufPos := int(SizeOfUint / 8)
		loop:
			for i := 0; i < len(auxv)-bufPos*2; i += bufPos * 2 {
				var tag, value uint
				switch SizeOfUint {
				case 32:
					tag = uint(binary.LittleEndian.Uint32(auxv[i:]))
					value = uint(binary.LittleEndian.Uint32(auxv[i+bufPos:]))
				case 64:
					tag = uint(binary.LittleEndian.Uint64(auxv[i:]))
					value = uint(binary.LittleEndian.Uint64(auxv[i+bufPos:]))
				}
				switch tag {
				// See asm/auxvec.h for the definition of constant AT_CLKTCK ("frequency at which times() increments"), which is an integer 17.
				case 17:
					sysconfClockTick = int(value)
					break loop
				}
			}
		}
		// Apparently 100 HZ is a very common value of _SC_CLK_TCK, it seems to be this way with Linux kernel 5.4.0 on x86-64.
		sysconfClockTick = 100
	})
	return sysconfClockTick
}

func getSchedStats(schedstatContent, schedContent string) (ret SchedulerStats) {
	// Collect fields of strings from /proc/XXXX.../schedstat
	// According to https://lkml.org/lkml/2019/7/24/906, the first field of "schedstat" means
	// "sum of all time spent running by tasks on this processor (in nanoseconds, or jiffies prior to 2.6.23)"
	// and the second field means "sum of all time spent waiting to run by tasks on this processor (in nanoseconds, or jiffies prior to 2.6.23)".
	schedstatFields := regexSchedstatFields.FindStringSubmatch(strings.TrimSpace(schedstatContent))
	ret.NumRunSec = float64(atoiOr0(strSliceElemOrEmpty(schedstatFields, 1))) / 1000000000
	ret.NumWaitSec = float64(atoiOr0(strSliceElemOrEmpty(schedstatFields, 2))) / 1000000000

	// Collect key-value pairs from /proc/XXXX.../sched
	schedKeyValue := make(map[string]string)
	for _, line := range strings.Split(schedContent, "\n") {
		submatches := regexColonKeyValue.FindStringSubmatch(strings.TrimSpace(line))
		if len(submatches) > 2 {
			schedKeyValue[submatches[1]] = submatches[2]
		}
	}
	ret.NumVoluntarySwitches = atoiOr0(schedKeyValue["nr_voluntary_switches"])
	ret.NumInvoluntarySwitches = atoiOr0(schedKeyValue["nr_involuntary_switches"])
	return
}

func getStats(content string) (ret ProcessStats) {
	statFields := regexStatFields.FindStringSubmatch(strings.TrimSpace(content))
	ret.State = strSliceElemOrEmpty(statFields, 3)
	ret.StartedAtUptimeSec = atoiOr0(strSliceElemOrEmpty(statFields, 22)) / getClockTicksPerSecond()
	ret.VirtualMemSizeBytes = atoiOr0(strSliceElemOrEmpty(statFields, 23))
	ret.ResidentSetMemSizeBytes = atoiOr0(strSliceElemOrEmpty(statFields, 24)) * os.Getpagesize()
	ret.NumUserModeSecInclChildren = float64(atoiOr0(strSliceElemOrEmpty(statFields, 14))+atoiOr0(strSliceElemOrEmpty(statFields, 16))) / float64(getClockTicksPerSecond())
	ret.NumKernelModeSecInclChildren = float64(atoiOr0(strSliceElemOrEmpty(statFields, 15))+atoiOr0(strSliceElemOrEmpty(statFields, 17))) / float64(getClockTicksPerSecond())
	return
}

func getStatus(content string) (ret ProcessStatus) {
	// Collect key-value pairs from /proc/XXXX/status
	statusKeyValue := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		submatches := regexColonKeyValue.FindStringSubmatch(strings.TrimSpace(line))
		if len(submatches) > 2 {
			statusKeyValue[submatches[1]] = submatches[2]
		}
	}
	uids := getDACIDsFromProcfs(statusKeyValue["Uid"])
	gids := getDACIDsFromProcfs(statusKeyValue["Gid"])
	ret.Name = statusKeyValue["Name"]
	ret.Umask = statusKeyValue["Umask"]
	ret.ParentPID = atoiOr0(statusKeyValue["PPid"])
	ret.ThreadGroupID = atoiOr0(statusKeyValue["NStgid"])
	ret.ProcessID = atoiOr0(statusKeyValue["NSpid"])
	ret.ProcessGroupID = atoiOr0(statusKeyValue["NSpgid"])
	ret.SessionID = atoiOr0(statusKeyValue["NSsid"])
	ret.ProcessPrivilege = ProcessPrivilege{
		RealUID:      uids[0],
		EffectiveUID: uids[1],
		RealGID:      gids[0],
		EffectiveGID: gids[1],
	}
	return
}
