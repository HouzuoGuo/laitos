package browser

import (
	"errors"
	"fmt"
	"github.com/HouzuoGuo/laitos/misc"
	"os"
	"path"
	"sync"
	"time"
)

// Instances manage lifecycle of a fixed number of browser server instances.
type Instances struct {
	PhantomJSExecPath string `json:"PhantomJSExecPath"` // Absolute or relative path to PhantomJS executable
	MaxInstances      int    `json:"MaxInstances"`      // Maximum number of instances
	MaxLifetimeSec    int    `json:"MaxLifetimeSec"`    // Unconditionally kill instance after this number of seconds elapse
	BasePortNumber    int    `json:"BasePortNumber"`    // Browser instances listen on a port number beginning from this one

	browserMutex   *sync.Mutex // Protect against concurrent modification to browsers
	browsers       []*Instance // All browsers
	browserCounter int         // Increment only counter
	logger         misc.Logger
}

// Check configuration and initialise internal states.
func (instances *Instances) Initialise() error {
	instances.logger = misc.Logger{ComponentName: "Instances", ComponentID: ""}
	if instances.MaxInstances < 1 {
		instances.MaxInstances = 5 // reasonable for a few consumers
	}
	if instances.MaxLifetimeSec < 1 {
		instances.MaxLifetimeSec = 1800 // half hour is quite reasonable
	}
	if _, err := os.Stat(instances.PhantomJSExecPath); err != nil {
		return fmt.Errorf("Instances.Initialise: cannot find PhantomJS executable \"%s\" - %v", instances.PhantomJSExecPath, err)
	}
	if err := os.Chmod(instances.PhantomJSExecPath, 0700); err != nil {
		return fmt.Errorf("Instances.Initialise: failed to chmod PhantomJS - %v", err)
	}
	if instances.BasePortNumber < 1024 {
		return errors.New("Instances.Initialise: BasePortNumber must be greater than 1023")
	}
	instances.browserMutex = new(sync.Mutex)
	instances.browsers = make([]*Instance, instances.MaxInstances)
	instances.browserCounter = -1
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
		RenderImagePath:    path.Join(os.TempDir(), fmt.Sprintf("laitos-browser-instance-render-%d-%d.png", time.Now().Unix(), index)),
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

// Forcibly stop all instance instances.
func (instances *Instances) KillAll() {
	instances.browserMutex.Lock()
	defer instances.browserMutex.Unlock()
	for _, instance := range instances.browsers {
		if instance != nil {
			instance.Kill()
		}
	}
}
