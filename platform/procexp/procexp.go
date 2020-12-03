package procexp

import (
	"io/ioutil"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// GetProcIDs returns a list of all process IDs visible to this program according to the information available from procfs.
func GetProcIDs() (ret []int) {
	ret = make([]int, 0)
	pidsUnderProcfs, err := filepath.Glob("/proc/[1-9]*")
	if err != nil {
		return
	}
	for _, pidPath := range pidsUnderProcfs {
		// Remove the prefix /proc/ from each of the return value
		id, _ := strconv.Atoi(strings.TrimPrefix(pidPath, "/proc/"))
		if id > 0 {
			ret = append(ret, id)
		}
	}
	sort.Ints(ret)
	return
}

// GetProcStatus returns the status and resource information about a process according to the information available from procfs.
// As a special case, if the input PID is 0, then the function will retrieve process status information about its own process ("self").
func GetProcStatus(pid int) (status ProcessStatus, err error) {
	pidStr := "self"
	if pid > 0 {
		pidStr = strconv.Itoa(pid)
	}
	statusContent, err := ioutil.ReadFile(path.Join("/proc", pidStr, "status"))
	if err != nil {
		return
	}
	statContent, err := ioutil.ReadFile(path.Join("/proc", pidStr, "stat"))
	if err != nil {
		return
	}
	schedstatContent, err := ioutil.ReadFile(path.Join("/proc", pidStr, "schedstat"))
	if err != nil {
		return
	}
	status = getProcStatus(string(statusContent), string(schedstatContent), string(statContent))
	return
}
