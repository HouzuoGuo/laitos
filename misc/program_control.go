package misc

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
)

const (
	// EnvironmentDecryptionPassword is a name of environment variable, the value of which supplies decryption password
	// to laitos program when it is started using encrypted config file (and or data files).
	EnvironmentDecryptionPassword = "LAITOS_DECRYPTION_PASSWORD"
)

var (
	// StartTime is the timestamp captured when this program started.
	StartupTime = time.Now()
	// ConfigFilePath is the absolute path to JSON configuration file that was used to launch this program.
	ConfigFilePath string

	// EnableAWSIntegration is a program-global flag that determines whether to integrate with various AWS services for the normal operation of laitos,
	// an example of such integration feature is to send incoming store&forward message to kinesis firehose.
	EnableAWSIntegration bool
	// EnablePrometheusIntegration is a program-global flag that determines whether to enable integration with prometheus by collecting and
	// serving metrics readings.
	EnablePrometheusIntegration bool
	// EmergencyLockDown is a flag checked by features and daemons, they should stop functioning or refuse to serve when the flag is true.
	EmergencyLockDown bool
	// ErrEmergencyLockDown is returned by some daemons to inform user that lock-down is in effect.
	ErrEmergencyLockDown = errors.New("LOCKED DOWN")

	// ProgramDataDecryptionPassword is the password string used to decrypt program data that was previously encrypted by
	// laitos' "datautil" encryption routine, protecting files such as configuration file and TLS certificate key.
	// The main function will collect this password from the channel of ProgramDataDecryptionPasswordInput.
	// The password will then be used for initialising daemons, starting them, etc.
	// In addition, when the supervisor recovers main program from an incidental crash, the supervisor will feed this password
	// to the standard input of the restarted main program.
	ProgramDataDecryptionPassword string

	// ProgramDataDecryptionPasswordInput is a synchronous channel that accepts a password string for decryption of program config and data.
	// It can be fed by a number of potential sources, such as standard input, AWS lambda handler, etc.
	// The main function will receive the supplied password and store it in ProgramDataDecryptionPassword string variable for further use.
	// Normally, only one source will send a password to this channel, but over here the channel is defined as a buffered to be on the defensive
	// side of programming mistakes.
	ProgramDataDecryptionPasswordInput = make(chan string, 10)

	// logger is used by some of the miscellaneous actions affecting laitos process globally.
	logger = lalog.Logger{ComponentName: "misc", ComponentID: []lalog.LoggerIDField{{Key: "PID", Value: os.Getpid()}}}
)

/*
TriggerEmergencyLockDown turns on EmergencyLockDown flag, so that features and daemons will immediately (or very soon)
stop functioning or refuse to serve more requests. The program process will keep running (i.e. not going to crash).
Once the function is called, there is no way to cancel lock-down status other than restarting the program.
*/
func TriggerEmergencyLockDown() {
	logger.Warning("", nil, "toolbox features and daemons will be disabled ASAP")
	EmergencyLockDown = true
}

// TriggerEmergencyStop crashes the program with an abort signal in 10 seconds.
func TriggerEmergencyStop() {
	logger.Warning("", nil, "program will crash soon")
	// Do not crash immediately. Give caller a short 10 seconds window to send a final notification Email if it wishes.
	go func() {
		time.Sleep(10 * time.Second)
		logger.Abort("", nil, "program crashes now")
	}()
}

/*
TriggerEmergencyKill wipes as much data as possible from all storage attached to this computer, and then crash the program.
This is a very dangerous operation!
*/
func TriggerEmergencyKill() {
	logger.Warning("", nil, "computer storage will be destroyed ASAP and then this program will then crash")
	// Do not kill immediately. Give caller a short 10 seconds window to send a final notification Email if it wishes.
	go func() {
		time.Sleep(10 * time.Second)
		filesToKill := getFilesToKill()
		dirsToKill := getDirsToKill()
		disksToKill := getDisksToKill()
		// Begin overwriting files to destroy them
		go func() {
			// Destroy files in parallel
			logger.Warning("", nil, "going to kill files - %v", filesToKill)
			for _, fileToKill := range filesToKill {
				go func() {
					// Ignore but log failure and keep going
					for {
						logger.Info("", nil, "attempt to destroy file - %s", fileToKill)
						err := overwriteWithZero(fileToKill)
						logger.Info("", err, "finished attempt at destroying file - %s", fileToKill)
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
			logger.Warning("", nil, "going to kill directories - %v", dirsToKill)
			for _, dirToKill := range dirsToKill {
				// Destroy directories in parallel
				go func() {
					// Ignore but log failure and keep going
					for {
						logger.Info("", nil, "attempt to destroy directory - %s", dirToKill)
						err := os.RemoveAll(dirToKill)
						logger.Info("", err, "finished attempt at destroying directory - %s", dirToKill)
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
			logger.Warning("", nil, "going to kill disks - %v", disksToKill)
			for _, diskToKill := range disksToKill {
				// Destroy disks in parallel
				go func() {
					// Ignore but log failure and keep going
					for {
						logger.Info("", nil, "attempt to destroy disk - %s", diskToKill)
						err := overwriteWithZero(diskToKill)
						logger.Info("", err, "finished attempt at destroying disk - %s", diskToKill)
						// Avoid overwhelming disk or CPU due to the deliberately infinite loop
						time.Sleep(1 * time.Second)
					}
				}()
				time.Sleep(200 * time.Millisecond)
			}
		}()

		// 120 seconds should be enough to cause sufficient damage, time to quit.
		time.Sleep(120 * time.Second)
		logger.Abort("", nil, "sufficient damage has been done, good bye.")
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
	for _, pattern := range []string{"/dev/sd*", "/dev/vd*", "/dev/xvd*", "/dev/nvme*", "/dev/hd*", "/dev/root"} {
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

// overwriteWithZero fills an existing file with 0s.
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
	if err := fh.Sync(); err != nil {
		return err
	}
	return fh.Close()
}
