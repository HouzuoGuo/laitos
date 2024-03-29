package procexp

import (
	"os"
	"reflect"
	"testing"
)

func TestGetSchedStats(t *testing.T) {
	schedstatsContent := `107299274581 51731777094 2172621`
	schedContent := `laitos.linux (929, #threads: 21)
-------------------------------------------------------------------
se.exec_start                                :     270775690.872708
se.vruntime                                  :        808034.826730
se.sum_exec_runtime                          :            22.068051
se.nr_migrations                             :                   12
nr_switches                                  :                  164
nr_voluntary_switches                        :                  128
nr_involuntary_switches                      :                   36
se.load.weight                               :              1048576
se.runnable_weight                           :              1048576
se.avg.load_sum                              :                   24
se.avg.runnable_load_sum                     :                   24
se.avg.util_sum                              :                25519
se.avg.load_avg                              :                    0
se.avg.runnable_load_avg                     :                    0
se.avg.util_avg                              :                    0
se.avg.last_update_time                      :      270775690871808
se.avg.util_est.ewma                         :                   21
se.avg.util_est.enqueued                     :                    1
policy                                       :                    0
prio                                         :                  120
clock-delta                                  :                   59
mm->numa_scan_seq                            :                    0
numa_pages_migrated                          :                    0
numa_preferred_nid                           :                   -1
total_numa_faults                            :                    0
current_node=0, numa_group_id=0
numa_faults node=0 task_private=0 task_shared=0 group_private=0 group_shared=0
`
	ret := getSchedStats(schedstatsContent, schedContent)
	match := SchedulerStats{
		NumVoluntarySwitches:   128,
		NumInvoluntarySwitches: 36,
		NumRunSec:              float64(107299274581) / 1000000000,
		NumWaitSec:             float64(51731777094) / 1000000000,
	}
	if !reflect.DeepEqual(ret, match) {
		t.Fatalf("\n%+v\n%+v\n", ret, match)
	}
}

func TestGetStats(t *testing.T) {
	statContent := `1036 (laitos.linux) S 1030 324 324 0 -1 1077936384 18171 29162419 13 491 68264 54000 82510 11761 20 0 16 0 2749 741847040 20403 18446744073709551615 4194304 12687727 140724042029504 0 0 0 0 0 2143420159 0 0 0 17 1 0 0 585 0 0 20758528 21091232 46993408 140724042030258 140724042030498 140724042030498 140724042031075 0`
	ret := getStats(statContent)
	match := ProcessStats{
		State:                        "S",
		StartedAtUptimeSec:           2749 / getClockTicksPerSecond(),
		VirtualMemSizeBytes:          741847040,
		ResidentSetMemSizeBytes:      20403 * os.Getpagesize(),
		NumUserModeSecInclChildren:   float64(68264+82510) / float64(getClockTicksPerSecond()),
		NumKernelModeSecInclChildren: float64(54000+11761) / float64(getClockTicksPerSecond()),
	}
	if !reflect.DeepEqual(ret, match) {
		t.Fatalf("\n%+v\n%+v\n", ret, match)
	}
}

func TestGetStatus(t *testing.T) {
	statusContent := `Name:   laitos.linux
Umask:  0022
State:  S (sleeping)
Tgid:   1036
Ngid:   0
Pid:    1036
PPid:   1030
TracerPid:      0
Uid:    1       2       3       4
Gid:    5       6       7       8
FDSize: 128
Groups:
NStgid: 1036
NSpid:  1036
NSpgid: 324
NSsid:  325
VmPeak:   724460 kB
VmSize:   724460 kB
VmLck:    724444 kB
VmPin:         0 kB
VmHWM:     81612 kB
VmRSS:     81612 kB
RssAnon:           67520 kB
RssFile:           14092 kB
RssShmem:              0 kB
VmData:   109112 kB
VmStk:       132 kB
VmExe:      8296 kB
VmLib:         4 kB
VmPTE:       252 kB
VmSwap:        0 kB
HugetlbPages:          0 kB
CoreDumping:    0
THP_enabled:    1
Threads:        16
SigQ:   0/31409
SigPnd: 0000000000000000
ShdPnd: 0000000000000000
SigBlk: 0000000000000000
SigIgn: 0000000000000000
SigCgt: fffffffe7fc1feff
CapInh: 0000000000000000
CapPrm: 0000003fffffffff
CapEff: 0000003fffffffff
CapBnd: 0000003fffffffff
CapAmb: 0000000000000000
NoNewPrivs:     0
Seccomp:        0
Speculation_Store_Bypass:       vulnerable
Cpus_allowed:   3
Cpus_allowed_list:      0-1
Mems_allowed:   00000000,00000000,00000000,00000000,00000000,00000000,00000000,00000000,00000000,00000000,00000000,00000000,00000000,00000000,00000000,00000000,00000000,00000000,00000000,00000000,00000000,00000000,00000000,00000000,00000000,00000000,00000000,00000000,00000000,00000000,00000000,00000001
Mems_allowed_list:      0
voluntary_ctxt_switches:        1432879
nonvoluntary_ctxt_switches:     739742`

	ret := getStatus(statusContent)
	match := ProcessStatus{
		Name:           "laitos.linux",
		Umask:          "0022",
		ThreadGroupID:  1036,
		ProcessID:      1036,
		ParentPID:      1030,
		ProcessGroupID: 324,
		SessionID:      325,
		ProcessPrivilege: ProcessPrivilege{
			RealUID:      1,
			EffectiveUID: 2,
			RealGID:      5,
			EffectiveGID: 6,
		},
	}
	if !reflect.DeepEqual(match, ret) {
		t.Fatalf("\n%+v\n%+v\n", match, ret)
	}
}
