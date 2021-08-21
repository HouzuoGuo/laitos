package maintenance

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/platform"
)

const (
	SwapFilePath = "/laitos-swap-file"
)

// SynchroniseSystemClock uses three different tools to immediately synchronise system clock via NTP servers.
func (daemon *Daemon) SynchroniseSystemClock(out *bytes.Buffer) {
	daemon.logPrintStage(out, "synchronise clock")
	if platform.HostIsWindows() {
		result, err := platform.InvokeProgram(nil, 120, `C:\Windows\system32\w32tm.exe`, "/config", `/manualpeerlist:"0.pool.ntp.org sg.pool.ntp.org us.pool.ntp.org fi.pool.ntp.org"`, "/syncfromflags:manual", "/reliable:yes", "/update")
		daemon.logPrintStageStep(out, "w32tm config: %v - %s", err, strings.TrimSpace(result))
		result, err = platform.InvokeProgram(nil, 120, `C:\Windows\system32\w32tm.exe`, "/resync", "/force")
		daemon.logPrintStageStep(out, "w32tm resync: %v - %s", err, strings.TrimSpace(result))
	} else {
		// Use three tools to immediately synchronise system clock
		result, err := platform.InvokeProgram([]string{"PATH=" + platform.CommonPATH}, 120, "ntpdate", "-4", "1.pool.ntp.org", "us.pool.ntp.org", "de.pool.ntp.org", "au.pool.ntp.org")
		daemon.logPrintStageStep(out, "ntpdate: %v - %s", err, strings.TrimSpace(result))
		result, err = platform.InvokeProgram([]string{"PATH=" + platform.CommonPATH}, 120, "chronyd", "-q", "pool pool.ntp.org iburst")
		daemon.logPrintStageStep(out, "chronyd: %v - %s", err, strings.TrimSpace(result))
		result, err = platform.InvokeProgram([]string{"PATH=" + platform.CommonPATH}, 120, "busybox", "ntpd", "-n", "-q", "-p", "2.pool.ntp.org", "-p", "uk.pool.ntp.org", "-p", "ca.pool.ntp.org", "-p", "jp.pool.ntp.org")
		daemon.logPrintStageStep(out, "busybox ntpd: %v - %s", err, strings.TrimSpace(result))
	}
	daemon.CorrectStartupTime(out)
}

/*
CorrectStartTime corrects program start time in case system clock is skewed.
The program startup time is used to detect outdated commands (such as in telegram bot), in rare case if system clock
was severely skewed, causing program startup time to be in the future, the detection mechanisms will misbehave.
*/
func (daemon *Daemon) CorrectStartupTime(out *bytes.Buffer) {
	if misc.StartupTime.After(time.Now()) {
		daemon.logPrintStage(out, "clock was severely skewed, reset program startup time.")
		// The earliest possible opportunity for system maintenance to run is now minus initial delay
		misc.StartupTime = time.Now().Add(-InitialDelaySec * time.Second)
	}
}

// MaintainServices manipulate service state according to configuration.
func (daemon *Daemon) MaintainServices(out *bytes.Buffer) {
	if daemon.DisableStopServices == nil && daemon.EnableStartServices == nil {
		return
	}
	daemon.logPrintStage(out, "maintain system services")

	if daemon.DisableStopServices != nil {
		sort.Strings(daemon.DisableStopServices)
		for _, name := range daemon.DisableStopServices {
			if platform.DisableStopDaemon(name) {
				daemon.logPrintStageStep(out, "disable&stop %s: OK", name)
			} else {
				daemon.logPrintStageStep(out, "disable&stop %s: failed or service does not exist", name)
			}
		}
	}
	if daemon.EnableStartServices != nil {
		sort.Strings(daemon.EnableStartServices)
		for _, name := range daemon.EnableStartServices {
			if platform.EnableStartDaemon(name) {
				daemon.logPrintStageStep(out, "enable&start %s: OK", name)
			} else {
				daemon.logPrintStageStep(out, "enable&start %s: failed or service does not exist", name)
			}
		}
	}
}

// BlockUnusedLogin will block/disable system login from users not listed in the exception list.
func (daemon *Daemon) BlockUnusedLogin(out *bytes.Buffer) {
	if daemon.BlockSystemLoginExcept == nil || len(daemon.BlockSystemLoginExcept) == 0 {
		return
	}
	daemon.logPrintStage(out, "block unused system login")
	// Exception name list is case insensitive
	exceptionMap := make(map[string]bool)
	for _, name := range daemon.BlockSystemLoginExcept {
		exceptionMap[strings.ToLower(name)] = true
	}
	for userName := range platform.GetLocalUserNames() {
		if exceptionMap[strings.ToLower(userName)] {
			daemon.logPrintStageStep(out, "not going to touch excluded user %s", userName)
			continue
		}
		if ok, blockOut := platform.BlockUserLogin(userName); ok {
			daemon.logPrintStageStep(out, "blocked user %s", userName)
		} else {
			daemon.logPrintStageStep(out, "failed to block user %s - %v", userName, blockOut)
		}
	}
}

// MaintainWindowsIntegrity uses DISM and SFC utilities to maintain Windows system integrity and runs Windows Update.
func (daemon *Daemon) MaintainWindowsIntegrity(out *bytes.Buffer) {
	if !platform.HostIsWindows() {
		return
	}
	daemon.logPrintStage(out, "maintain windows system integrity")
	// These tools seriously spend a lot of time
	daemon.logPrintStageStep(out, "running dism StartComponentCleanup")
	progOut, err := platform.InvokeProgram(nil, 4*3600, `C:\Windows\system32\Dism.exe`, "/Online", "/Cleanup-Image", "/StartComponentCleanup", "/ResetBase")
	daemon.logPrintStageStep(out, "dism StartComponentCleanup result is: %v - %s", err, progOut)
	daemon.logPrintStageStep(out, "running dism SPSuperseded")
	progOut, err = platform.InvokeProgram(nil, 4*3600, `C:\Windows\system32\Dism.exe`, "/Online", "/Cleanup-Image", "/SPSuperseded")
	daemon.logPrintStageStep(out, "dism SPSuperseded, result is: %v - %s", err, progOut)
	daemon.logPrintStageStep(out, "starting dism Restorehealth")
	progOut, err = platform.InvokeProgram(nil, 4*3600, `C:\Windows\system32\Dism.exe`, "/Online", "/Cleanup-Image", "/Restorehealth")
	daemon.logPrintStageStep(out, "dism Restorehealth result is: %v - %s", err, progOut)
	daemon.logPrintStageStep(out, "running system file checker")
	/*
		The output coming from sfc often causes rendering defects in a large variety of terminals - they appear to be
		rendered in twice the width of ordinary letters. Consequently, the maintenance report delivered as mail is cut
		right before the sfc output section by popular SMTP servers. Until the exact nature of the command output can be
		determined, the output will be suppressed.
	*/
	_, err = platform.InvokeProgram(nil, 4*3600, `C:\Windows\system32\sfc.exe`, "/ScanNow")
	daemon.logPrintStageStep(out, "system file checker result is: %v", err)
	daemon.logPrintStage(out, "installing windows updates")
	// Have to borrow script host's capability to search and installwindows updates
	script, err := ioutil.TempFile("", "laitos-windows-update-script*.vbs")
	if err != nil {
		daemon.logPrintStageStep(out, "failed to create update script: %v", err)
		return
	}
	defer func() {
		_ = os.Remove(script.Name())
	}()
	if err := script.Close(); err != nil {
		daemon.logPrintStageStep(out, "failed to create update script: %v", err)
		return
	}
	err = ioutil.WriteFile(script.Name(), []byte(`
Set updateSession = CreateObject("Microsoft.Update.Session")
updateSession.ClientApplicationID = "laitos"
Set searchResult = updateSession.CreateUpdateSearcher().Search("IsInstalled=0 and Type='Software' and IsHidden=0")
If searchResult.Updates.Count = 0 Then
    WScript.Echo "Already up to date"
    WScript.Quit
End If
Set updatesToDownload = CreateObject("Microsoft.Update.UpdateColl")
For I = 0 to searchResult.Updates.Count-1
    Set update = searchResult.Updates.Item(I)
    addThisUpdate = false
    If update.InstallationBehavior.CanRequestUserInput = true Then
        WScript.Echo I + 1 & "> skipping: " & update.Title
    Else
        If update.EulaAccepted = false Then
            update.AcceptEula()
        End If
        updatesToDownload.Add(update)
    End If
Next
If updatesToDownload.Count = 0 Then
    WScript.Echo "Nothing to install - all updates require user interaction"
    WScript.Quit
End If
Set downloader = updateSession.CreateUpdateDownloader()
downloader.Updates = updatesToDownload
downloader.Download()
Set updatesToInstall = CreateObject("Microsoft.Update.UpdateColl")
For I = 0 To searchResult.Updates.Count-1
    set update = searchResult.Updates.Item(I)
    If update.IsDownloaded = true Then
        updatesToInstall.Add(update)
    End If
Next
If updatesToInstall.Count = 0 Then
    WScript.Echo "Failed to download updates"
    WScript.Quit
End If
Set installer = updateSession.CreateUpdateInstaller()
installer.Updates = updatesToInstall
Set installationResult = installer.Install()
WScript.Echo "Installation result: " & installationResult.ResultCode
WScript.Echo "Reboot required: " & installationResult.RebootRequired & vbCRLF
WScript.Echo "Individual installation result:"
For I = 0 to updatesToInstall.Count - 1
		WScript.Echo I + 1 & "> " & updatesToInstall.Item(i).Title & ": " & installationResult.GetUpdateResult(i).ResultCode
Next
`), 0600)
	if err != nil {
		daemon.logPrintStageStep(out, "failed to write update script: %v", err)
		return
	}
	progOut, err = platform.InvokeProgram(nil, 4*3600, `C:\Windows\system32\cscript.exe`, script.Name())
	daemon.logPrintStageStep(out, "windows update result: %v - %s", err, progOut)
}

// MaintainSwapFile creates and activates a swap file for Linux system, or turns swap off depending on configuration input.
func (daemon *Daemon) MaintainSwapFile(out *bytes.Buffer) {
	if platform.HostIsWindows() {
		daemon.logPrintStage(out, "skipped on windows: maintain swap file")
		return
	}
	if daemon.SwapFileSizeMB == 0 {
		return
	}
	daemon.logPrintStage(out, "create/turn on swap file "+SwapFilePath)
	if daemon.SwapFileSizeMB < 0 {
		daemon.logPrintStageStep(out, "turn off swap")
		if err := platform.SwapOff(); err != nil {
			daemon.logPrintStageStep(out, "failed to turn off swap: %v", err)
		}
		return
	} else if daemon.SwapFileSizeMB > 0 {
		_, swapFileStatus := os.Stat(SwapFilePath)
		// Create the swap file if it does not yet exist
		if os.IsNotExist(swapFileStatus) {
			buf := make([]byte, 1048576)
			fh, err := os.Create(SwapFilePath)
			if err != nil {
				daemon.logPrintStageStep(out, "failed to create swap file - %v", err)
				return
			}
			for i := 0; i < daemon.SwapFileSizeMB; i++ {
				if _, err := fh.Write(buf); err != nil {
					daemon.logPrintStageStep(out, "failed to create swap file - %v", err)
					return
				}
			}
			if err := fh.Sync(); err != nil {
				daemon.logPrintStageStep(out, "failed to create swap file - %v", err)
				return
			}
			if err := fh.Close(); err != nil {
				daemon.logPrintStageStep(out, "failed to create swap file - %v", err)
				return
			}
			// If the file already exists, it will not be grown or recreated.
		} else if swapFileStatus != nil {
			daemon.logPrintStageStep(out, "failed to determine swap file status - %v", swapFileStatus)
			return
		} else {
			daemon.logPrintStage(out, "the swap file appears to already exist")
		}
		// Correct the swap file permission and ownership
		if err := os.Chmod(SwapFilePath, 0600); err != nil {
			daemon.logPrintStageStep(out, "failed to correct swap file permission - %v", err)
			return
		}
		if err := os.Chown(SwapFilePath, 0, 0); err != nil {
			daemon.logPrintStageStep(out, "failed to correct swap file owner - %v", err)
			return
		}
		// Format the swap file
		if progOut, err := platform.InvokeProgram(nil, platform.CommonOSCmdTimeoutSec, "mkswap", SwapFilePath); err != nil {
			daemon.logPrintStageStep(out, "failed to format swap file - %v - %s", err, progOut)
		}
		// Turn on the swap file
		progOut, err := platform.InvokeProgram(nil, platform.CommonOSCmdTimeoutSec, "swapon", SwapFilePath)
		if err != nil {
			daemon.logPrintStageStep(out, "failed to turn on swap file - %v - %s", err, progOut)
		}
	}
}

// recursivelyChown changes owner and group of all files underneath the path, including the path itself.
func recursivelyChown(rootPath string, newUID, newGID int) (succeeded, failed int) {
	_ = filepath.Walk(rootPath, func(thisPath string, info os.FileInfo, err error) error {
		if err := os.Lchown(thisPath, newUID, newGID); err == nil {
			succeeded++
		} else {
			failed++
		}
		return nil
	})
	return
}

// EnhanceFileSecurity hardens ownership and permission of common locations in file system.
func (daemon *Daemon) EnhanceFileSecurity(out *bytes.Buffer) {
	if !daemon.DoEnhanceFileSecurity {
		return
	}
	if platform.HostIsWindows() {
		daemon.logPrintStage(out, "skipped on windows: enhance file security")
		return
	}
	daemon.logPrintStage(out, "enhance file security")

	myUser, err := user.Current()
	if err != nil {
		daemon.logPrintStageStep(out, "failed to get current user - %v", err)
		return
	}
	// Will run chown on the following paths later
	pathUID := make(map[string]int)
	pathGID := make(map[string]int)
	// Will run chmod on the following paths later
	path600 := make(map[string]struct{})
	path700 := make(map[string]struct{})

	// Discover all ordinary user home directories
	allHomeDirAbs := make(map[string]struct{})
	if myUser.HomeDir != "" {
		allHomeDirAbs[myUser.HomeDir] = struct{}{}
	}
	for _, homeDirParent := range []string{"/home", "/Users"} {
		subDirs, err := ioutil.ReadDir(homeDirParent)
		if err != nil {
			continue
		}
		for _, subDir := range subDirs {
			allHomeDirAbs[filepath.Join(homeDirParent, subDir.Name())] = struct{}{}
		}
	}

	// Reset owner and group of an ordinary user's home directory
	for homeDirAbs := range allHomeDirAbs {
		userName := filepath.Base(homeDirAbs)
		if userName != "" && userName != "." {
			u, err := user.Lookup(userName)
			if err == nil {
				// Chown the home directory
				if i, err := strconv.Atoi(u.Gid); err == nil {
					pathGID[homeDirAbs] = i
				}
				if i, err := strconv.Atoi(u.Uid); err == nil {
					pathUID[homeDirAbs] = i
				}
			}
		}
	}

	// Reset owner and group of root home directory
	for _, rootHomeAbs := range []string{"/root", "/private/var/root"} {
		if stat, err := os.Stat(rootHomeAbs); err == nil && stat.IsDir() {
			pathUID[rootHomeAbs] = 0
			pathGID[rootHomeAbs] = 0
			// Reset permission on the home directory and ~/.ssh later
			allHomeDirAbs[rootHomeAbs] = struct{}{}
		}
	}

	// Reset permission on home directory and ~/.ssh
	for homeDirAbs := range allHomeDirAbs {
		// chmod 700 ~
		path700[homeDirAbs] = struct{}{}
		// Chmod 700 ~/.ssh
		sshDirAbs := filepath.Join(homeDirAbs, ".ssh")
		if stat, err := os.Stat(sshDirAbs); err == nil && stat.IsDir() {
			path700[sshDirAbs] = struct{}{}
			// Chmod 600 ~/.ssh/*
			if sshContent, err := ioutil.ReadDir(sshDirAbs); err == nil {
				for _, entry := range sshContent {
					path600[filepath.Join(sshDirAbs, entry.Name())] = struct{}{}
				}
			}
		}
	}

	// Do it!
	for aPath, newUID := range pathUID {
		succeeded, failed := recursivelyChown(aPath, newUID, -1)
		daemon.logPrintStageStep(out, "recursively set owner to %d in path %s - %d succeeded, %d failed", newUID, aPath, succeeded, failed)
	}
	for aPath, newGID := range pathGID {
		succeeded, failed := recursivelyChown(aPath, -1, newGID)
		daemon.logPrintStageStep(out, "recursively set group to %d in path %s - %d succeeded, %d failed", newGID, aPath, succeeded, failed)
	}
	for aPath := range path600 {
		daemon.logPrintStageStep(out, "set permission to 600 to path %s - %v", aPath, os.Chmod(aPath, 0600))

	}
	for aPath := range path700 {
		daemon.logPrintStageStep(out, "set permission to 700 to path %s - %v", aPath, os.Chmod(aPath, 0700))
	}
}

// RunMaintenanceScripts runs the shell script specifically defined for the host OS type in daemon configuration.
func (daemon *Daemon) RunMaintenanceScripts(out *bytes.Buffer) {
	var scriptOut string
	var err error
	if daemon.ScriptForUnix != "" && !platform.HostIsWindows() {
		daemon.logPrintStage(out, "run script for unix-like system")
		scriptOut, err = platform.InvokeShell(daemon.IntervalSec/2, platform.GetDefaultShellInterpreter(), daemon.ScriptForUnix)
	}
	if daemon.ScriptForWindows != "" && platform.HostIsWindows() {
		daemon.logPrintStage(out, "run script for windows system")
		scriptOut, err = platform.InvokeShell(daemon.IntervalSec/2, platform.GetDefaultShellInterpreter(), daemon.ScriptForWindows)
	}
	daemon.logPrintStage(out, "script result: %s - %v", scriptOut, err)
}
