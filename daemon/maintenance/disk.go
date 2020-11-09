package maintenance

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/platform"
)

// CleanUpFiles gets rid of unused temporary files on both Unix-like and Windows OSes.
func (daemon *Daemon) CleanUpFiles(out *bytes.Buffer) {
	daemon.logPrintStage(out, "disk clean up")
	sevenDaysAgo := time.Now().Add(-(7 * 24 * time.Hour))
	// Keep in mind that /var/tmp is supposed to hold "persistent temporary files" in Linux
	locations := []string{`/tmp`, `C:\Temp`, `C:\Tmp`, `C:\Windows\Temp`}
	if envTemp := os.Getenv("TEMP"); envTemp != "" {
		locations = append(locations, envTemp)
	}
	if envTmp := os.Getenv("TMP"); envTmp != "" {
		locations = append(locations, envTmp)
	}
	daemon.logPrintStageStep(out, "will remove week-old temporary files from: %v", locations)
	for _, location := range locations {
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
	if platform.HostIsWindows() {
		/*
			Use Windows Disk Cleanup to do a more thorough round of clean up. The Windows program is not very well
			suited for headless operation, modes such as "/SAGERUN" and "/VERYLOWDISK" have to spawn a window, and in
			case of headless operation (e.g. running laitos at boot automatically via Task Scheduler) the modes will
			simply hang.
			"AUTOCLEAN" however does not hang in headless mode and seems to kick off some kind of background routine to
			complete the disk clean up operation.
		*/
		result, err := platform.InvokeProgram(nil, 3600, `C:\Windows\system32\cleanmgr.exe`, "/AUTOCLEAN")
		daemon.logPrintStageStep(out, "windows automated disk clean up: %v - %s", err, strings.TrimSpace(result))
	}
}

// DefragmentAllDisks defragments all disks on Windows. This routine does nothing on Linux.
func (daemon *Daemon) DefragmentAllDisks(out *bytes.Buffer) {
	if !platform.HostIsWindows() {
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
	daemon.logPrintStageStep(out, "defragmenting C drive")
	result, err := platform.InvokeProgram(nil, 4*3600, `C:\Windows\system32\Defrag.exe`, "C:")
	daemon.logPrintStageStep(out, "C drive defragmentation result: %v - %s", err, strings.TrimSpace(result))
	daemon.logPrintStageStep(out, "defragmenting all drives")
	result, err = platform.InvokeProgram(nil, 4*3600, `C:\Windows\system32\Defrag.exe`, "/C")
	daemon.logPrintStageStep(out, "all drives defragmentation result: %v - %s", err, strings.TrimSpace(result))
}

// TrimSSDDisk executes SSD TRIM operation on C:\ drive (Windows) or all drives
func (daemon *Daemon) TrimAllSSDs(out *bytes.Buffer) {
	daemon.logPrintStage(out, "trim SSD disks")
	if platform.HostIsWindows() {
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
