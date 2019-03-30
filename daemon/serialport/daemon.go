package serialport

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
	"github.com/HouzuoGuo/laitos/testingstub"
	"github.com/HouzuoGuo/laitos/toolbox"
)

const (
	// MaxCommandLength is the maximum length (number of bytes) of an acceptable input toolbox command.
	MaxCommandLength = 4096
	// GlobIntervalSec is the number of seconds to wait in between attempts at scanning (globbing) newly connected serial devices.
	GlobIntervalSec = 3
	// RateLimitIntervalSec is an interval measured in seconds to measure the rate of incoming toolbox commands from each connected serial device.
	RateLimitIntervalSec = 1
	// CommandTimeoutSec is the maximum duration allowed for a toolbox command to execute.
	CommandTimeoutSec = 10 * 60
)

// Daemon implements a server-side program to serve toolbox commands over serial communication made via the eligible devices.
type Daemon struct {
	/*
		DeviceGlobPatterns determines the patterns of eligible serial devices to which toolbox commands will run.
		The daemon periodically scans and serves newly connected devices that match these patterns.
	*/
	DeviceGlobPatterns []string `json:"DeviceGlobPatterns"`

	// PerDeviceLimit is the approximate number of requests allowed from a serial device within a designated interval.
	PerDeviceLimit int `json:"PerDeviceLimit"`
	// Processor is the toolbox command processor.
	Processor *common.CommandProcessor `json:"-"`

	// connectedDevices contains the device names of all ongoing serial connections. The value signals ongoing connection to stop.
	connectedDevices      map[string]chan bool
	connectedDevicesMutex *sync.Mutex
	// stop signals StartAndBlock loop to stop processing newly connected devices.
	stop chan bool

	loopIsRunning bool // loopIsRunning indicates that daemon is looking for new devices to converse with.
	rateLimit     *misc.RateLimit
	logger        lalog.Logger
}

// Initialise validates configuration and initialises internal states.
func (daemon *Daemon) Initialise() error {
	if daemon.DeviceGlobPatterns == nil {
		daemon.DeviceGlobPatterns = []string{}
	}
	if daemon.PerDeviceLimit < 1 {
		daemon.PerDeviceLimit = 2 // reasonable for interactive usage
	}

	// Validate all patterns
	for _, pattern := range daemon.DeviceGlobPatterns {
		_, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("serialport.Initialise: device glob pattern \"%s\" is malformed", pattern)
		}
	}

	daemon.connectedDevices = make(map[string]chan bool)
	daemon.connectedDevicesMutex = new(sync.Mutex)

	// Though serial devices are unlikely to be connected via the Internet, the safety check is nonetheless useful.
	if errs := daemon.Processor.IsSaneForInternet(); len(errs) > 0 {
		return fmt.Errorf("serialport.Initialise: %+v", errs)
	}

	daemon.logger = lalog.Logger{ComponentName: "serialport"}
	daemon.Processor.SetLogger(daemon.logger)
	daemon.rateLimit = &misc.RateLimit{
		MaxCount: daemon.PerDeviceLimit,
		UnitSecs: RateLimitIntervalSec,
		Logger:   daemon.logger,
	}
	daemon.stop = make(chan bool)
	daemon.rateLimit.Initialise()
	return nil
}

// StartAndBlock continuously looks for newly connected serial devices and serve them.
func (daemon *Daemon) StartAndBlock() error {
	defer func() {
		daemon.loopIsRunning = false
	}()
	daemon.loopIsRunning = true
	for {
		if misc.EmergencyLockDown {
			daemon.logger.Warning("StartAndBlock", "", misc.ErrEmergencyLockDown, "")
			return misc.ErrEmergencyLockDown
		}
		// Look for all populated serial devices that match the patterns
		for _, pattern := range daemon.DeviceGlobPatterns {
			matches, err := filepath.Glob(pattern)
			if err != nil {
				daemon.logger.Warning("StartAndBlock", pattern, err, "failed to use the pattern to scan for serial devices")
				continue // next pattern
			}
			daemon.connectToDevices(matches)
		}
		// Sleep for the interval and continue scanning
		select {
		case <-daemon.stop:
			return nil
		case <-time.After(GlobIntervalSec * time.Second):
		}
	}
}

// connectToDevices looks for new device paths yet to be connected among the input array and start a processing loop dedicated to each new device.
func (daemon *Daemon) connectToDevices(devicePaths []string) {
	daemon.connectedDevicesMutex.Lock()
	defer daemon.connectedDevicesMutex.Unlock()
	for _, dev := range devicePaths {
		if _, exists := daemon.connectedDevices[dev]; !exists {
			// Conversation may be stopped by either explicit daemon termination or IO error, hence there are maximum of two bufferd stop signals.
			stopChan := make(chan bool, 2)
			daemon.connectedDevices[dev] = stopChan
			go daemon.converseWithDevice(dev, stopChan)
		}
	}
}

/*
converseWithDevice continuously proceses toolbox commands input from the serial device in a loop, and terminates when the channel is notified by
either termination of daemon or device IO error.
*/
func (daemon *Daemon) converseWithDevice(devPath string, stopChan chan bool) {
	daemon.logger.Info("converseWithDevice", devPath, nil, "beginning conversation")
	defer func() {
		// Upon termination, remove the serial device from connected device map.
		daemon.connectedDevicesMutex.Lock()
		delete(daemon.connectedDevices, devPath)
		daemon.connectedDevicesMutex.Unlock()
		daemon.logger.Info("converseWithDevice", devPath, nil, "conversation terminated")
	}()
	// Converse with the serial device as if it is an ordinary file. This approach works on both Windows and Linux.
	devFile, err := os.OpenFile(devPath, os.O_RDWR, 0600)

	if err != nil {
		daemon.logger.Warning("converseWithDevice", devPath, err, "failed to open device file handle")
		return
	}
	// Terminate the conversation upon IO error or explicit daemon termination
	defer func() {
		daemon.logger.MaybeError(devFile.Close())
	}()
	// Converse with the device in a background routine, signal stopChan to terminate the conversation in case of IO error.
	go func() {
		for {
			if misc.EmergencyLockDown {
				stopChan <- true
				daemon.logger.Warning("HandleTCPConnection", "", misc.ErrEmergencyLockDown, "")
				return
			}
			// Serial devices often use \r or \n (or both) to indicate end of line, readUntilDelimiter only returns non-empty string without delimiter.
			cmdBytes, err := readUntilDelimiter(io.LimitReader(devFile, MaxCommandLength), '\r', '\n')
			if err != nil {
				daemon.logger.Warning("converseWithDevice", devPath, err, "failed to read command")
				stopChan <- true
				return
			}
			// Check against rate limit
			if !daemon.rateLimit.Add(devPath, true) {
				continue
			}
			// Trim the input toolbox command as a string
			cmd := strings.TrimSpace(string(cmdBytes))
			if len(cmd) == 0 {
				continue
			}
			// Execute the toolbox comamnd
			daemon.logger.Info("converseWithDevice", devPath, nil, "received %d characters", len(cmd))
			result := daemon.Processor.Process(toolbox.Command{Content: cmd, TimeoutSec: CommandTimeoutSec}, true)
			daemon.logger.Info("converseWithDevice", devPath, nil, "about to transmit %d characters", len(result.CombinedOutput))
			if err := writeSlowly(devFile, []byte(result.CombinedOutput+"\r\n")); err != nil {
				daemon.logger.Warning("converseWithDevice", devPath, err, "failed to write command response")
				stopChan <- true
				return
			}
		}
	}()
	<-stopChan
}

// Stop stops accepting new device connections and then disconnects all ongoing conversations with connected serial devices.
func (daemon *Daemon) Stop() {
	if !daemon.loopIsRunning {
		return
	}
	// Prevent more conversations from being started
	daemon.stop <- true
	// Terminate all ongoing conversations
	for _, stopChan := range daemon.connectedDevices {
		stopChan <- true
	}
}

// TestDaemon provides unit test coverage for the serial port daemon.
func TestDaemon(daemon *Daemon, t testingstub.T) {
	// Instead of emulating a serial device driven by OS driver, the test subject simply uses a text file with a line of command.
	tmpFileNamePrefix := fmt.Sprintf("laitos-serialport-TestDaemon-%d", time.Now().UnixNano())
	tmpFile, err := ioutil.TempFile("", tmpFileNamePrefix+"*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Fatal(err)
		}
	}()
	// In a toolbox command, write down a valid PIN and a shell command that prints a line of text
	anticipatedResponse := "test daemon response"
	if err := ioutil.WriteFile(tmpFile.Name(), []byte(fmt.Sprintf("%s .s echo %s\r\n", common.TestCommandProcessorPIN, anticipatedResponse)), 0600); err != nil {
		t.Fatal(err)
	}
	// Reinitialise daemon to use a pattern to glob the temporary text file
	oldPatterns := daemon.DeviceGlobPatterns
	daemon.DeviceGlobPatterns = []string{path.Join(os.TempDir(), tmpFileNamePrefix+"*")}
	defer func() {
		daemon.DeviceGlobPatterns = oldPatterns
	}()
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	// Server should start within two seconds
	var stoppedNormally bool
	go func() {
		if err := daemon.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
		stoppedNormally = true
	}()
	time.Sleep(2 * time.Second)
	// Wait for daemon to pick up this newly "connected" serial device file
	time.Sleep((GlobIntervalSec + 1) * time.Second)
	successfulContentMatch := make(chan bool, 1)
	go func() {
		// Keep watching the file content looking for the anticipated response
		for i := 0; i < 100; i++ {
			content, err := ioutil.ReadFile(tmpFile.Name())
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(content), anticipatedResponse) == 2 {
				successfulContentMatch <- true
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	// Anticipate correct response in a second
	select {
	case <-time.After(1 * time.Second):
		t.Fatal("daemon did not respond to toolbox command timely")
	case <-successfulContentMatch:
		// Test is OK
	}

	// Daemon should stop within a second
	daemon.Stop()
	time.Sleep(1 * time.Second)
	if !stoppedNormally {
		t.Fatal("did not stop")
	}
	// Repeatedly stopping the daemon should have no negative consequence
	daemon.Stop()
	daemon.Stop()
}
