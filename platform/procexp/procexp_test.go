package procexp

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"sort"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/platform"
)

func TestGetProcIDs(t *testing.T) {
	if platform.HostIsWindows() {
		t.Skip("this test will not run on windows")
	}
	allProcIDs := GetProcIDs()
	selfProcStatus, err := GetProcStatus(0)
	if err != nil {
		t.Fatal(err)
	}
	var mustFindPIDs = []struct {
		pid int
	}{{1}, {selfProcStatus.ProcessID}}
	for _, findPID := range mustFindPIDs {
		t.Run(fmt.Sprintf("find PID %d", findPID.pid), func(t *testing.T) {
			if foundAt := sort.SearchInts(allProcIDs, findPID.pid); allProcIDs[foundAt] != findPID.pid {
				t.Fatal(allProcIDs)
			}
		})
	}
}

func TestGetProcStatusByPID(t *testing.T) {
	if platform.HostIsWindows() {
		t.Skip("this test will not run on windows")
	}
	cmd := exec.Command("/usr/bin/yes")
	cmd.Stdout = ioutil.Discard
	cmd.Stderr = ioutil.Discard
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()
	// It takes couple of seconds for the process to use up some CPU time
	time.Sleep(8 * time.Second)
	status, err := GetProcStatus(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("\nStatus: %+v\n", status)
	if status.Name != "yes" || (status.State != "R" && status.State != "S") || status.ThreadGroupID == 0 || status.StartedAtUptimeSec == 0 ||
		status.ProcessPrivilege.EffectiveUID == 0 || status.ProcessPrivilege.EffectiveGID == 0 ||
		status.ProcessMemUsage.VirtualMemSizeBytes == 0 || status.ProcessMemUsage.ResidentSetMemSizeBytes == 0 ||
		status.ProcessCPUUsage.NumUserModeSecInclChildren+status.ProcessCPUUsage.NumSysModeSecInclChildren == 0 ||
		status.ProcessSchedulerStats.NumVoluntaryCtxSwitches == 0 || status.ProcessSchedulerStats.NumNonVoluntaryCtxSwitches == 0 || status.ProcessSchedulerStats.NumRunSec == 0 {
		t.Fatalf("\n%+v\n", status)
	}
}
