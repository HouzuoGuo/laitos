package encarchive

import (
	"fmt"
	"github.com/HouzuoGuo/laitos/misc"
	"io/ioutil"
	"log"
	"os"
	"time"
)

const (
	RamdiskCommandTimeoutSec = 10 // RamdiskCommandTimeoutSec is the timeout for mounting and umounting ramdisk.
)

// MakeRamdisk uses mount command to create a ramdisk in a temporary directory and return the directory's path.
func MakeRamdisk(sizeMB int) (string, error) {
	mountPoint, err := ioutil.TempDir("/root", "laitos-bundle")
	if err != nil {
		return "", err
	}
	out, err := misc.InvokeShell(RamdiskCommandTimeoutSec, "/bin/sh", fmt.Sprintf("mount -t tmpfs -o size=%dm tmpfs '%s'", sizeMB, mountPoint))
	if err != nil {
		return "", fmt.Errorf("MakeRamdisk: mount command failed due to error %v - %s", err, out)
	}
	return mountPoint, nil
}

// DestroyRamdisk un-mounts the ramdisk's mount point, effectively destroying the content.
func DestroyRamdisk(mountPoint string) {
	out, err := misc.InvokeShell(RamdiskCommandTimeoutSec, "/bin/sh", fmt.Sprintf("umount -lfr '%s'", mountPoint))
	if err != nil {
		// Retry once
		time.Sleep(1 * time.Second)
		out, err = misc.InvokeShell(RamdiskCommandTimeoutSec, "/bin/sh", fmt.Sprintf("umount -lfr '%s'", mountPoint))
	}
	if err != nil {
		log.Printf("DestroyRamdisk: umount command failed due to error %v - %s", err, out)
	}
	os.RemoveAll(mountPoint)
}
