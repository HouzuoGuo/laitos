package encarchive

import (
	"fmt"
	"github.com/HouzuoGuo/laitos/misc"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"
)

const (
	RamdiskCommandTimeoutSec = 10 // RamdiskCommandTimeoutSec is the timeout for mounting and umounting ramdisk.

	/*
		RamdiskParentDir is the directory under which new ramdisk mount points will be created.
		The directory is placed underneath /root to prevent accidental access by computer users or other programs.
	*/
	RamdiskParentDir = "/root"

	// RamdiskTmpdirNamePrefix is the prefix name of temporary directory which is to become mount point for ramdisk.
	RamdiskTmpdirNamePrefix = "ramdisk-laitos-extracted-bundle"
)

// MakeRamdisk uses mount command to create a ramdisk in a temporary directory and return the directory's path.
func MakeRamdisk(sizeMB int) (string, error) {
	mountPoint, err := ioutil.TempDir(RamdiskParentDir, RamdiskTmpdirNamePrefix)
	if err != nil {
		return "", err
	}
	out, err := misc.InvokeProgram(nil, RamdiskCommandTimeoutSec, "mount", "-t", "tmpfs", "-o", fmt.Sprintf("size=%dm", sizeMB), "tmpfs", mountPoint)
	if err != nil {
		TryDestroyRamdisk(mountPoint)
		return "", fmt.Errorf("MakeRamdisk: mount command failed due to error %v - %s", err, out)
	}
	misc.DefaultLogger.Warning("MakeRamdisk", mountPoint, nil, "successfully created a %d MB ramdisk", sizeMB)
	return mountPoint, nil
}

// TryDestroyRamdisk umounts a mount point directory and removes it, all done without force. Returns true only if successful.
func TryDestroyRamdisk(mountPoint string) bool {
	misc.InvokeProgram(nil, RamdiskCommandTimeoutSec, "umount", mountPoint)
	err2 := os.Remove(mountPoint)
	if err2 == nil {
		misc.DefaultLogger.Warning("TryDestroyRamdisk", mountPoint, nil, "successfully destroyed ramdisk")
		return true
	}
	return false
}

/*
TryDestroyAllRamdisks unmounts all ramdisk mount points from ramdisk parent directory, without forcing. This is
especially useful when laitos is restarted via systemd or supervisord, to clean up left-over ramdisks from previous
launches.
*/
func TryDestroyAllRamdisks() {
	dirs, err := ioutil.ReadDir(RamdiskParentDir)
	if err != nil {
		return
	}
	for _, info := range dirs {
		if strings.HasPrefix(info.Name(), RamdiskTmpdirNamePrefix) {
			TryDestroyRamdisk(path.Join(RamdiskParentDir, info.Name()))
		}
	}
}

// DestroyRamdisk un-mounts the ramdisk's mount point and removes the mount point directory and its content.
func DestroyRamdisk(mountPoint string) {
	out, err := misc.InvokeProgram(nil, RamdiskCommandTimeoutSec, "umount", "-l", "-f", "-r", mountPoint)
	if err != nil {
		// Retry once
		time.Sleep(2 * time.Second)
		out, err = misc.InvokeProgram(nil, RamdiskCommandTimeoutSec, "umount", "-l", "-f", "-r", mountPoint)
	}
	if err != nil {
		misc.DefaultLogger.Warning("DestroyRamdisk", mountPoint, err, "umount command failed, output is - %s", out)
	}
	if err := os.RemoveAll(mountPoint); err == nil {
		misc.DefaultLogger.Warning("DestroyRamdisk", mountPoint, nil, "successfully destroyed ramdisk")
	} else {
		misc.DefaultLogger.Warning("DestroyRamdisk", mountPoint, err, "failed to remove mount point directory, output is - %s", out)
	}
}
