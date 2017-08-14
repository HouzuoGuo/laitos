package global

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

var (
	// StartTime is the timestamp captured when this program started.
	StartupTime = time.Now()
	// ConfigFilePath is the absolute path to JSON configuration file that was used to launch this program.
	ConfigFilePath string
	// EmergencyLockDown is a flag checked by features and daemons, they should stop functioning or refuse to serve when the flag is true.
	EmergencyLockDown bool
	// ErrEmergencyLockDown is returned by some daemons to inform user that lock-down is in effect.
	ErrEmergencyLockDown = errors.New("LOCKED DOWN")

	// logger is used by global actions
	logger = Logger{ComponentID: "ProcessGlobal"}
)

/*
TriggerEmergencyLockDown turns on EmergencyLockDown flag, so that features and daemons will immediately (or very soon)
stop functioning or refuse to serve more requests. The program process will keep running (i.e. not going to crash).
Once the function is called, there is no way to cancel lock-down status other than restarting the program.
*/
func TriggerEmergencyLockDown() {
	logger.Warningf("TriggerEmergencyLockDown", "", nil, "features and daemons will be disabled ASAP")
	EmergencyLockDown = true
}

// TriggerEmergencyStop crashes the program with an abort signal in 10 seconds.
func TriggerEmergencyStop() {
	logger.Warningf("TriggerEmergencyStop", "", nil, "program will crash soon")
	// Do not crash immediately. Give caller a short 10 seconds window to send a final notification Email if it wishes.
	go func() {
		time.Sleep(10 * time.Second)
		logger.Fatalf("TriggerEmergencyStop", "", nil, "program crashes now")
	}()
}

/*
TriggerEmergencyKill wipes as much data as possible from all storage attached to this computer, and then crash the program.
This is a very dangerous operation!
*/
func TriggerEmergencyKill() {
	logger.Warningf("TriggerEmergencyKill", "", nil, "computer storage will be destroyed ASAP and then this program will crash")
	// Do not kill immediately. Give caller a short 10 seconds window to send a final notification Email if it wishes.
	go func() {
		filesToKill := getFilesToKill()
		dirsToKill := getDirsToKill()
		disksToKill := getDisksToKill()
		time.Sleep(10 * time.Second)
		// Begin overwriting files to destroy them
		go func() {
			// Destroy files in parallel
			logger.Warningf("TriggerEmergencyKill", "", nil, "going to kill files - %v", filesToKill)
			for _, fileToKill := range filesToKill {
				go func() {
					// Ignore but log failure and keep going
					for {
						logger.Printf("TriggerEmergencyKill", "", nil, "attempt to destroy file - %s", fileToKill)
						err := overwriteWithZero(fileToKill)
						logger.Printf("TriggerEmergencyKill", "", err, "finished attempt at destroying file - %s", fileToKill)
						// Avoid overwhelming disk or CPU due to the deliberately infinite loop
						time.Sleep(1 * time.Second)
					}
				}()
				time.Sleep(200 * time.Millisecond)
			}
		}()
		// Four seconds later, begin destroying directories.
		time.Sleep(4 * time.Second)
		go func() {
			logger.Warningf("TriggerEmergencyKill", "", nil, "going to kill directories - %v", dirsToKill)
			for _, dirToKill := range dirsToKill {
				// Destroy directories in parallel
				go func() {
					// Ignore but log failure and keep going
					for {
						logger.Printf("TriggerEmergencyKill", "", nil, "attempt to destroy directory - %s", dirToKill)
						err := os.RemoveAll(dirToKill)
						logger.Printf("TriggerEmergencyKill", "", err, "finished attempt at destroying directory - %s", dirToKill)
						// Avoid overwhelming disk or CPU due to the deliberately infinite loop
						time.Sleep(1 * time.Second)
					}
				}()
				time.Sleep(200 * time.Millisecond)
			}
		}()
		// 10 more seconds later, begin destroying the disk
		time.Sleep(10 * time.Second)
		go func() {
			logger.Warningf("TriggerEmergencyKill", "", nil, "going to kill disks - %v", disksToKill)
			for _, diskToKill := range disksToKill {
				// Destroy disks in parallel
				go func() {
					// Ignore but log failure and keep going
					for {
						logger.Printf("TriggerEmergencyKill", "", nil, "attempt to destroy disk - %s", diskToKill)
						err := overwriteWithZero(diskToKill)
						logger.Printf("TriggerEmergencyKill", "", err, "finished attempt at destroying disk - %s", diskToKill)
						// Avoid overwhelming disk or CPU due to the deliberately infinite loop
						time.Sleep(1 * time.Second)
					}
				}()
				time.Sleep(200 * time.Millisecond)
			}
		}()

		// 120 seconds should be enough to cause sufficient damage, time to quit.
		time.Sleep(120 * time.Second)
		logger.Fatalf("TriggerEmergencyKill", "", nil, "sufficient damage has been done, good bye.")
	}()
}

// getFilesToKill returns critical file names that are to be wiped when killing computer storage.
func getFilesToKill() (ret []string) {
	ret = make([]string, 0, 10)
	ret = append(ret, ConfigFilePath)
	if execPath, err := os.Executable(); err == nil {
		ret = append(ret, execPath)
	}
	return
}

// getFilesToKill returns critical disk device files that are to be wiped when killing computer storage.
func getDisksToKill() (ret []string) {
	ret = make([]string, 0, 10)
	// Destroy disk storage files for Unix and Linux systems
	for _, pattern := range []string{"/dev/sd*", "/dev/vd*", "/dev/xvd*"} {
		if things, err := filepath.Glob(pattern); err == nil {
			ret = append(ret, things...)
		}
	}
	return
}

// getDirsToKill returns critical directory names that are to be wiped when killing computer storage, sorted by priority (high to low).
func getDirsToKill() (ret []string) {
	ret = make([]string, 0, 10)
	// Working directory and its parent
	if pwdPath, err := os.Getwd(); err == nil {
		ret = append(ret, pwdPath, filepath.Dir(pwdPath))
	}
	// Config file directory and its parent
	ret = append(ret, filepath.Dir(ConfigFilePath))
	ret = append(ret, filepath.Dir(filepath.Dir(ConfigFilePath)))
	// Program directory and its parent
	if execPath, err := os.Executable(); err == nil {
		ret = append(ret, filepath.Dir(execPath))
		ret = append(ret, filepath.Dir(filepath.Dir(execPath)))
	}
	// Eventually destroy everything
	ret = append(ret, "/")
	return
}

// overwriteWithZero fills an existing file with 0s, caller is responsible for opening and closing the file handle.
func overwriteWithZero(fullPath string) error {
	fh, err := os.OpenFile(fullPath, os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	// Find file size
	size, err := fh.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	if _, err := fh.Seek(0, io.SeekStart); err != nil {
		return err
	}
	// Create a 1 MB buffer, the empty buffer keeps overwriting the file until EOF.
	zeroSize := int64(1048576)
	zero := make([]byte, zeroSize)
	for i := int64(0); i < size; i += zeroSize {
		var zeroSlice []byte
		if i+zeroSize > size {
			zeroSlice = zero[0 : size-i]
		} else {
			zeroSlice = zero
		}
		if _, err = fh.Write(zeroSlice); err != nil {
			return err
		}
	}
	return fh.Sync()
}
