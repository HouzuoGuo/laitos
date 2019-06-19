package maintenance

import (
	"bytes"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/platform"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CleanUpFiles gets rid of unused temporary files on both Unix-like and Windows OSes.
func (daemon *Daemon) CleanUpFiles(out *bytes.Buffer) {
	daemon.logPrintStage(out, "disk clean up")
	sevenDaysAgo := time.Now().Add(-(7 * 24 * time.Hour))
	// Keep in mind that /var/tmp is supposed to hold "persistent temporary files" in Linux
	for _, location := range []string{`/tmp`, `C:\Temp`, `C:\Windows\Temp`} {
		_ = filepath.Walk(location, func(thisPath string, info os.FileInfo, err error) error {
			if err == nil {
				if info.ModTime().Before(sevenDaysAgo) {
					if deleteErr := os.RemoveAll(thisPath); deleteErr == nil {
						daemon.logPrintStageStep(out, "deleted %s", thisPath)
					} else {
						daemon.logPrintStageStep(out, "failed to deleted %s - %v", thisPath, deleteErr)
					}
				}
			}
			return nil
		})
	}
	if misc.HostIsWindows() {
		// On Windows, run Disk Cleanup application to do a more thorough round.
		daemon.logPrintStageStep(out, "running Windows Disk Cleanup")
		result, err := platform.InvokeProgram(nil, 3600, `C:\Windows\system32\cleanmgr.exe`, "/SAGERUN:1")
		daemon.logPrintStageStep(out, "disk clean up result (round 1): %v - %s", err, strings.TrimSpace(result))
		result, err = platform.InvokeProgram(nil, 3600, `C:\Windows\system32\cleanmgr.exe`, "/VERYLOWDISK")
		daemon.logPrintStageStep(out, "disk clean up result (final round): %v - %s", err, strings.TrimSpace(result))
	}
}

// DefragmentAllDisks defragments all disks on Windows. This routine does nothing on Linux.
func (daemon *Daemon) DefragmentAllDisks(out *bytes.Buffer) {
	if !misc.HostIsWindows() {
		daemon.logPrintStage(out, "skipped on non-windows: defragment disks")
		return
	}
	daemon.logPrintStage(out, "defragment disks")
	/*
		According to Windows command help, the following command defragments all drives:
		C:\Windows\system32\Defrag.exe /C /V
		The command however malfunctions when at least one disk is disqualified for defragmentation.
		Therefore, the strategy is to first try defragmenting C drive alone, and then use the command above to
		defragment all drives, ensuring that at lest the system drive is defragmented.
		This way, the defragmentation of C: drive will not have to share its command timeout with other drives.
	*/
	daemon.logPrintStageStep(out, "Defragmenting C drive")
	result, err := platform.InvokeProgram(nil, 4*3600, `C:\Windows\system32\Defrag.exe`, "C:", "/V")
	daemon.logPrintStageStep(out, "C drive defragmentation result: %v - %s", err, strings.TrimSpace(result))
	daemon.logPrintStageStep(out, "Defragmenting all drives")
	result, err = platform.InvokeProgram(nil, 4*3600, `C:\Windows\system32\Defrag.exe`, "/C", "/V")
	daemon.logPrintStageStep(out, "All drives defragmentation result: %v - %s", err, strings.TrimSpace(result))
}

// TrimSSDDisk executes SSD TRIM operation on C:\ drive (Windows) or all drives
func (daemon *Daemon) TrimAllSSDs(out *bytes.Buffer) {
	daemon.logPrintStage(out, "trim SSD disks")
	if misc.HostIsWindows() {
		/*
			According to Windows command help, the following command defragments all drives:
			C:\Windows\system32\Defrag.exe /C /L
			The command however malfunctions when at least one disk is disqualified for SSD trimming.
			Therefore, the strategy is to first try trimming C drive alone, and then use the command above to trim all
			drives, ensuring that at lest the system drive is trimmed.
		*/
		daemon.logPrintStageStep(out, "trimming SSD C drive")
		result, err := platform.InvokeProgram(nil, 30*60, `C:\Windows\system32\Defrag.exe`, "C:", "/L")
		daemon.logPrintStageStep(out, "C drive trimming result: %v - %s", err, strings.TrimSpace(result))
		daemon.logPrintStageStep(out, "trimming all SSD drives")
		result, err = platform.InvokeProgram(nil, 30*60, `C:\Windows\system32\Defrag.exe`, "/C", "/L")
		daemon.logPrintStageStep(out, "all drives trimming result: %v - %s", err, strings.TrimSpace(result))
	} else {
		daemon.logPrintStageStep(out, "trimming all SSD drives")
		result, err := platform.InvokeProgram([]string{"PATH=" + platform.CommonPATH}, 30*60, "fstrim", "-a")
		daemon.logPrintStageStep(out, "all drives trimming result: %v - %s", err, strings.TrimSpace(result))
	}
}
