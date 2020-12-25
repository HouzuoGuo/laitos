package procexp

import (
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// TaskStatus describes the scheduler status of a thread ("task") that belongs to a process.
type TaskStatus struct {
	ID              int
	KernelStack     []string
	WaitChannelName string
	SchedulerStats  SchedulerStats
}

// ProcessAndTasks describes the identity and resource usage of a process and its tasks.
type ProcessAndTasks struct {
	Status            ProcessStatus
	Stats             ProcessStats
	Tasks             map[int]TaskStatus
	SchedulerStatsSum SchedulerStats
}

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

// GetProcTaskIDs returns a sorted list of task (thread) IDs that belong to the process identified by ID.
// As a special case, if the input process ID is 0, then the return value will belong to the process invoking this function.
func GetProcTaskIDs(pid int) (ret []int) {
	ret = make([]int, 0)
	pidStr := "self"
	if pid > 0 {
		pidStr = strconv.Itoa(pid)
	}
	tasksUnderProcfs, err := filepath.Glob(fmt.Sprintf("/proc/%s/task/*", pidStr))
	if err != nil {
		return
	}
	for _, taskPath := range tasksUnderProcfs {
		// Remove the prefix /proc/ from each of the return value
		id, _ := strconv.Atoi(strings.TrimPrefix(taskPath, fmt.Sprintf("/proc/%s/task/", pidStr)))
		if id > 0 {
			ret = append(ret, id)
		}
	}
	sort.Ints(ret)
	return
}

// GetTaskStatus returns the stack information and scheduler stats for the specified task.
// As a special case, if the input process ID is 0, then the function will find the task (identified by ID) from its own process.
func GetTaskStatus(pid, taskID int) (status TaskStatus) {
	status.ID = taskID
	pidStr := "self"
	if pid > 0 {
		pidStr = strconv.Itoa(pid)
	}
	taskDir := fmt.Sprintf("/proc/%s/task/%d/", pidStr, taskID)
	status.KernelStack = make([]string, 0)
	stack, _ := ioutil.ReadFile(path.Join(taskDir, "stack"))
	// On each line of the stack trace there is a function name
	// A line looks like: [<0>] poll_schedule_timeout.constprop.0+0x46/0x70
	for _, line := range strings.Split(string(stack), "\n") {
		if rightBracket := strings.IndexByte(line, ']'); rightBracket > 3 {
			status.KernelStack = append(status.KernelStack, strings.TrimSpace(line[rightBracket+1:]))
		}
	}
	// Wait channel usually is the name of the function at the very top of the stack
	waitChannelName, _ := ioutil.ReadFile(path.Join(taskDir, "wchan"))
	status.WaitChannelName = string(waitChannelName)

	schedContent, _ := ioutil.ReadFile(path.Join(taskDir, "sched"))
	schedstatContent, _ := ioutil.ReadFile(path.Join(taskDir, "schedstat"))
	status.SchedulerStats = getSchedStats(string(schedstatContent), string(schedContent))
	return
}

// GetProcAndTaskStatus returns the status and resource information about a process according to the information available from procfs.
// As a special case, if the input PID is 0, then the function will retrieve process status information from its own process ("self").
func GetProcAndTaskStatus(pid int) (ret ProcessAndTasks, err error) {
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
	ret.Status = getStatus(string(statusContent))
	ret.Stats = getStats(string(statContent))
	ret.Tasks = make(map[int]TaskStatus)
	// List process tasks and get the status of individual tasks
	for _, id := range GetProcTaskIDs(pid) {
		taskStatus := GetTaskStatus(pid, id)
		ret.Tasks[id] = taskStatus
		ret.SchedulerStatsSum.NumRunSec += taskStatus.SchedulerStats.NumRunSec
		ret.SchedulerStatsSum.NumWaitSec += taskStatus.SchedulerStats.NumWaitSec
		ret.SchedulerStatsSum.NumVoluntarySwitches += taskStatus.SchedulerStats.NumVoluntarySwitches
		ret.SchedulerStatsSum.NumInvoluntarySwitches += taskStatus.SchedulerStats.NumInvoluntarySwitches
	}
	return
}
