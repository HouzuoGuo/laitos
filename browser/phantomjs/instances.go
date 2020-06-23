package phantomjs

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/HouzuoGuo/laitos/lalog"
	"github.com/HouzuoGuo/laitos/misc"
)

/*
Instances manage lifecycle of a fixed number of browser server instances (PhantomJS). As of 2018-06-26, there is no plan
to support this remote browser on Windows.
*/
type Instances struct {
	PhantomJSExecPath string `json:"PhantomJSExecPath"` // Absolute or relative path to PhantomJS executable
	MaxInstances      int    `json:"MaxInstances"`      // Maximum number of instances
	MaxLifetimeSec    int    `json:"MaxLifetimeSec"`    // Unconditionally kill instance after this number of seconds elapse
	BasePortNumber    int    `json:"BasePortNumber"`    // Browser instances listen on a port number beginning from this one

	browserMutex   *sync.Mutex // Protect against concurrent modification to browsers
	browsers       []*Instance // All browsers
	browserCounter int         // Increment only counter
	logger         lalog.Logger
}

// Check configuration and initialise internal states.
func (instances *Instances) Initialise() error {
	instances.logger = lalog.Logger{
		ComponentName: "phantomjs.Instances",
		ComponentID:   []lalog.LoggerIDField{{Key: "MaxInst", Value: instances.MaxInstances}, {Key: "MaxLifetime", Value: instances.MaxLifetimeSec}},
	}
	if instances.MaxInstances < 1 {
		instances.MaxInstances = 5 // reasonable for a few consumers
	}
	if instances.MaxLifetimeSec < 1 {
		instances.MaxLifetimeSec = 1800 // half hour is quite reasonable
	}
	if instances.PhantomJSExecPath == "" {
		instances.PhantomJSExecPath = "phantomjs" // find it among $PATH
	}
	if instances.BasePortNumber < 1024 {
		return errors.New("phantomjs.Instances.Initialise: BasePortNumber must be greater than 1023")
	}

	instances.browserMutex = new(sync.Mutex)
	instances.browsers = make([]*Instance, instances.MaxInstances)
	instances.browserCounter = -1
	return nil
}

// TestPhantomJSExecutable returns an error only if there is a problem with using the PhantomJS executable.
func (instances *Instances) TestPhantomJSExecutable() error {
	if misc.HostIsWindows() {
		/*
			PhantomJS won't work on Windows, this hacky attempt silences its initialisation error so that configuration
			made for Linux hosts will run unmodified on Windows.
		*/
		return nil
	}
	if _, err := os.Stat(instances.PhantomJSExecPath); err == nil {
		/*
			If the executable path appears to be a file that is readable, then make sure it has the correct executable permission.
			Ignore the error if the file cannot be modified, e.g. laitos progrma data resides on a read-only file system.
		*/
		_ = os.Chmod(instances.PhantomJSExecPath, 0755)
	} else if strings.ContainsRune(instances.PhantomJSExecPath, '/') {
		/*
			If the executable path looks like a file path (i.e., "programs/phantomjs" instead of "phantomjs"), but it
			cannot be read, then it is a severe configuration error.
		*/
		return fmt.Errorf("phantomjs.Instances.Initialise: cannot find PhantomJS executable \"%s\" - %v", instances.PhantomJSExecPath, err)
	} else if _, err := exec.LookPath(instances.PhantomJSExecPath); err != nil {
		/*
			If the executable path does not look like a file path and looks like a command name instead, make sure it
			can be found among $PATH.
		*/
		return fmt.Errorf("phantomjs.Instances.Initialise: cannot find PhantomJS executable among $PATH \"%s\" - %v", instances.PhantomJSExecPath, err)
	}
	return nil
}

// Acquire a new instance instance. If necessary, kill an existing instance to free up the space for the new instance.
func (instances *Instances) Acquire() (index int, browser *Instance, err error) {
	instances.browserMutex.Lock()
	defer instances.browserMutex.Unlock()
	instances.browserCounter++
	index = instances.browserCounter % len(instances.browsers)
	if instance := instances.browsers[index]; instance != nil {
		instance.Kill()
	}
	browser = &Instance{
		PhantomJSExecPath:  instances.PhantomJSExecPath,
		RenderImagePath:    filepath.Join(os.TempDir(), fmt.Sprintf("laitos-browser-instance-render-phantomjs-%d-%d.jpg", time.Now().Unix(), index)),
		Port:               instances.BasePortNumber + int(index),
		AutoKillTimeoutSec: instances.MaxLifetimeSec,
		Index:              index,
	}
	instances.browsers[index] = browser
	err = browser.Start()
	return
}

/*
Return instance instance of the specified index and match its tag against expectation.
If instance instance does not exist or tag does not match, return nil.
*/
func (instances *Instances) Retrieve(index int, expectedTag string) *Instance {
	instances.browserMutex.Lock()
	defer instances.browserMutex.Unlock()
	browser := instances.browsers[index]
	if browser == nil || browser.Tag != expectedTag {
		return nil
	}
	return browser
}

// Forcibly stop all browser instances.
func (instances *Instances) KillAll() {
	instances.browserMutex.Lock()
	defer instances.browserMutex.Unlock()
	for _, instance := range instances.browsers {
		if instance != nil {
			instance.Kill()
		}
	}
}
