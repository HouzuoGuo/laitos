package procexp

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/platform"
)

func TestGetProcIDs(t *testing.T) {
	if platform.HostIsWindows() {
		t.Skip("this test will not run on windows")
		return
	}
	allProcIDs := GetProcIDs()
	var mustFindPIDs = []struct {
		pid int
	}{{1}, {os.Getpid()}}
	for _, findPID := range mustFindPIDs {
		t.Run(fmt.Sprintf("find PID %d", findPID.pid), func(t *testing.T) {
			if foundAt := sort.SearchInts(allProcIDs, findPID.pid); allProcIDs[foundAt] != findPID.pid {
				t.Fatal(findPID, allProcIDs)
			}
		})
	}
}

func TestGetProcTaskIDs(t *testing.T) {
	if platform.HostIsWindows() {
		t.Skip("this test will not run on windows")
		return
	}
	// The program's main thread (task ID == program PID) must show up
	allTaskIDs := GetProcTaskIDs(os.Getpid())
	if foundAt := sort.SearchInts(allTaskIDs, os.Getpid()); allTaskIDs[foundAt] != os.Getpid() {
		t.Fatal(allTaskIDs)
	}
	// Get this process' own task IDs
	allTaskIDs = GetProcTaskIDs(0)
	if foundAt := sort.SearchInts(allTaskIDs, os.Getpid()); allTaskIDs[foundAt] != os.Getpid() {
		t.Fatal(allTaskIDs)
	}
}

func TestGetTaskStatus_WaitChannel(t *testing.T) {
	if platform.HostIsWindows() || platform.HostIsWSL() {
		t.Skip("this test will not run on windows")
		return
	}
	cmd := exec.Command("/bin/cat")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()
	time.Sleep(1 * time.Second)
	// By now kernel should see that sleep command is sleeping nicely
	status := GetTaskStatus(cmd.Process.Pid, cmd.Process.Pid)
	// On the stack there shall be at very least syscall entry, architecture-dependent sleep function, and a common sleep function.
	if status.ID != cmd.Process.Pid || status.WaitChannelName == "" {
		t.Fatalf("%+v", status)
	}
	// The stack file can only be read by root, hence it's not part of this test yet.
}

func TestGetTaskStatus_SchedulerStats(t *testing.T) {
	if platform.HostIsWindows() || platform.HostIsWSL() {
		t.Skip("this test will not run on windows")
		return
	}
	cmd := exec.Command("/usr/bin/yes")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()
	// It takes couple of seconds for the process to use up some CPU time
	time.Sleep(8 * time.Second)
	status := GetTaskStatus(cmd.Process.Pid, cmd.Process.Pid)
	if status.SchedulerStats.NumVoluntarySwitches == 0 || status.SchedulerStats.NumInvoluntarySwitches == 0 || status.SchedulerStats.NumRunSec == 0 {
		t.Fatalf("%+v", status)
	}
}

func TestGetProcAndTaskStatus(t *testing.T) {
	if platform.HostIsWindows() || platform.HostIsWSL() {
		t.Skip("this test will not run on windows")
		return
	}
	cmd := exec.Command("/usr/bin/yes")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()
	// It takes couple of seconds for the process to use up some CPU time
	time.Sleep(8 * time.Second)
	status, err := GetProcAndTaskStatus(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if status.Stats.NumUserModeSecInclChildren+status.Stats.NumKernelModeSecInclChildren < 1 ||
		status.Stats.ResidentSetMemSizeBytes < 100 || status.Stats.VirtualMemSizeBytes < 100 ||
		status.Stats.State == "" || status.Status.Name != "yes" ||
		len(status.Tasks) < 1 {
		t.Fatalf("%+v", status)
	}
	mainTask := status.Tasks[cmd.Process.Pid]
	if mainTask.ID != cmd.Process.Pid || mainTask.SchedulerStats.NumRunSec < 1 || mainTask.SchedulerStats.NumWaitSec < 0.001 ||
		mainTask.SchedulerStats.NumVoluntarySwitches < 10 || mainTask.SchedulerStats.NumInvoluntarySwitches < 10 {
		t.Fatalf("%+v", status)
	}
	if status.SchedulerStatsSum.NumRunSec < 1 || status.SchedulerStatsSum.NumWaitSec < 0.001 ||
		status.SchedulerStatsSum.NumVoluntarySwitches < 10 || status.SchedulerStatsSum.NumInvoluntarySwitches < 10 {
		t.Fatalf("%+v", status)
	}
}
